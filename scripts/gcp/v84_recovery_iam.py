"""Run-scoped, one-hour Secret Manager IAM lease for V84 recovery.

This module deliberately operates only on one secret-level IAM policy. It never
reads or emits secret payloads. Network mutations are not retried automatically.
"""
from __future__ import annotations

import copy
import datetime as dt
import json
import os
from pathlib import Path
import re
import stat


SECRET = "espacigo-testprod-cloudsql-postgres-admin-password"
MEMBER = "serviceAccount:espacigo-test-migrator@espacigo-testprod.iam.gserviceaccount.com"
ROLE = "roles/secretmanager.secretAccessor"
UUID_RE = re.compile(r"[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}")


def _require(ok, message):
    if not ok:
        raise ValueError(message)


def _directory(directory: Path, run_id: str) -> Path:
    _require(isinstance(run_id, str) and UUID_RE.fullmatch(run_id), "run_id must be a lowercase UUID")
    d = Path(directory)
    _require(not d.is_symlink() and d.is_dir(), "directory must be an existing private directory")
    st = d.stat()
    _require(stat.S_IMODE(st.st_mode) == 0o700 and st.st_uid == os.getuid(), "directory must be owned by caller with mode 0700")
    return d


def _member_binding() -> dict:
    return {"role": ROLE, "members": [MEMBER]}


def _owned(binding: dict) -> bool:
    return binding.get("role") == ROLE and MEMBER in binding.get("members", [])


def _reject_existing_admin(policy_doc: dict) -> None:
    for b in policy_doc.get("bindings", []):
        if _owned(b):
            raise ValueError("preexisting admin secret-access binding found; refusing grant")


def _canonical(doc: dict) -> dict:
    # Validate through the established policy canonicalizer while retaining all
    # policy fields (including auditConfigs and binding metadata).
    _require(isinstance(doc, dict) and isinstance(doc.get("bindings", []), list), "malformed secret IAM policy")
    result = copy.deepcopy(doc)
    result.setdefault("bindings", [])
    for binding in result["bindings"]:
        _require(isinstance(binding, dict) and isinstance(binding.get("role"), str)
                 and isinstance(binding.get("members"), list), "malformed IAM binding")
        _require(all(isinstance(m, str) and m for m in binding["members"]), "malformed IAM members")
        if "condition" in binding:
            _require(isinstance(binding["condition"], dict), "malformed IAM condition")
    return result


def _semantic(doc: dict) -> str:
    # Normalize only ordering; preserve every baseline field semantically.
    d = _canonical(doc)
    d.pop("etag", None)
    d.pop("version", None)
    for b in d["bindings"]:
        if isinstance(b.get("members"), list):
            b["members"] = sorted(b["members"])
    d["bindings"] = sorted(d["bindings"], key=lambda x: json.dumps(x, sort_keys=True, separators=(",", ":")))
    return json.dumps(d, sort_keys=True, separators=(",", ":"))


class _Cloud:
    def __init__(self, cloud):
        self.cloud = cloud

    def json(self, *args):
        # Accept the established Cloud adapter shape, but constrain commands.
        if hasattr(self.cloud, "json"):
            return self.cloud.json(*args)
        return json.loads(self.cloud.run(*args, "--format=json"))

    def run(self, *args):
        if hasattr(self.cloud, "run"):
            return self.cloud.run(*args)
        raise ValueError("cloud adapter lacks run()")

    def get(self):
        return self.json("secrets", "get-iam-policy", SECRET)

    def set(self, path):
        return self.json("secrets", "set-iam-policy", SECRET, str(path))


def _write_policy(directory: Path, value: dict, action: str) -> Path:
    _require(action in ('grant', 'revoke'), 'invalid IAM payload action')
    path = directory / ("secret-iam-policy-" + action + ".json")
    _require(not path.exists() and not path.is_symlink(), "policy payload path already exists; refusing overwrite")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        json.dump(value, f, sort_keys=True, separators=(",", ":"))
        f.write("\n"); f.flush(); os.fsync(f.fileno())
    return path


def _intent(directory: Path, run_id: str, action: str, binding: dict, baseline: dict, expected: dict) -> Path:
    path = directory / ("iam-" + action + "-intent-" + run_id + ".json")
    _require(not path.exists() and not path.is_symlink(), "IAM intent already exists; inspect outcome before retry")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        json.dump({"format": "espacigo.v84.secret-iam-intent.v1", "action": action,
                   "run_id": run_id, "secret": SECRET, "member": MEMBER, "role": ROLE,
                   "binding": binding, "baseline": baseline, "expected": expected,
                   "created_at": dt.datetime.now(dt.timezone.utc).isoformat()}, f,
                  sort_keys=True, separators=(",", ":"))
        f.write("\n"); f.flush(); os.fsync(f.fileno())
    return path


def _condition(run_id: str, now: dt.datetime) -> dict:
    expiry = now + dt.timedelta(hours=1)
    return {"role": ROLE, "members": [MEMBER], "condition": {
        "title": "v84-recovery-" + run_id,
        "description": "Temporary V84 database recovery access",
        "expression": "request.time < timestamp(" + json.dumps(expiry.isoformat().replace("+00:00", "Z")) + ")"}}


def _valid_own(b: dict, run_id: str) -> bool:
    c = b.get("condition", {})
    if b.get("role") != ROLE or b.get("members") != [MEMBER] or c.get("title") != "v84-recovery-" + run_id:
        return False
    m = re.fullmatch(r'request\.time < timestamp\(("[^"\\]+")\)', c.get("expression", ""))
    if not m:
        return False
    try:
        exp = dt.datetime.fromisoformat(json.loads(m.group(1)).replace("Z", "+00:00"))
    except (ValueError, TypeError):
        return False
    now = dt.datetime.now(dt.timezone.utc)
    return exp.tzinfo is not None and now < exp <= now + dt.timedelta(hours=1, minutes=1)


def grant(cloud, directory, run_id) -> dict:
    d = _directory(directory, run_id)
    api = _Cloud(cloud)
    fresh = _canonical(api.get())
    _require(isinstance(fresh.get('etag'), str) and bool(fresh['etag']), 'fresh policy etag required')
    _reject_existing_admin(fresh)
    now = dt.datetime.now(dt.timezone.utc)
    binding = _condition(run_id, now)
    baseline = copy.deepcopy(fresh)
    expected = copy.deepcopy(fresh)
    expected["version"] = 3
    expected.setdefault("bindings", []).append(copy.deepcopy(binding))
    intent_path = _intent(d, run_id, "grant", binding, baseline, expected)
    # Get etag from the fresh read; never rebuild a policy from an old receipt.
    payload = copy.deepcopy(expected)
    payload["etag"] = fresh["etag"]
    policy_path = _write_policy(d, payload, 'grant')
    api.set(policy_path)
    observed = _canonical(api.get())
    own = [b for b in observed.get("bindings", []) if _valid_own(b, run_id)]
    _require(len(own) == 1, "grant readback missing or ambiguous; inspect stored intent before retry")
    _require(_semantic(observed) == _semantic(expected), "grant readback differs from intended baseline; inspect stored intent")
    return {"action": "grant", "run_id": run_id, "secret": SECRET, "member": MEMBER,
            "role": ROLE, "expires_at": binding["condition"]["expression"],
            "intent": str(intent_path), "verified": True}


def _read_grant_intent(directory, run_id):
    path = directory / ('iam-grant-intent-' + run_id + '.json')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, encoding='utf-8') as stream:
        info = os.fstat(stream.fileno())
        _require(stat.S_ISREG(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o600
                 and info.st_uid == os.getuid() and info.st_nlink == 1, 'unsafe grant intent')
        intent = json.load(stream)
    _require(intent.get('format') == 'espacigo.v84.secret-iam-intent.v1'
             and intent.get('action') == 'grant' and intent.get('run_id') == run_id
             and intent.get('secret') == SECRET and intent.get('member') == MEMBER
             and intent.get('role') == ROLE, 'grant intent resource or identity differs')
    binding = intent.get('binding', {})
    _require(binding.get('role') == ROLE and binding.get('members') == [MEMBER]
             and binding.get('condition', {}).get('title') == 'v84-recovery-' + run_id,
             'stored grant binding differs from exact run')
    return intent


def revoke(cloud, directory, run_id) -> dict:
    d = _directory(directory, run_id)
    original = _read_grant_intent(d, run_id)
    api = _Cloud(cloud)
    fresh = _canonical(api.get())
    _require(isinstance(fresh.get('etag'), str) and bool(fresh['etag']), 'fresh policy etag required')
    bindings = fresh.get("bindings", [])
    matching = [b for b in bindings if b.get("role") == ROLE and MEMBER in b.get("members", [])
                and b.get("condition", {}).get("title") == "v84-recovery-" + run_id]
    _require(len(matching) <= 1, "multiple run-owned bindings found; manual recovery required")
    if matching:
        _require(matching[0].get('condition') == original['binding']['condition'],
                 'run binding condition was changed; refusing removal of an unknown binding')
    baseline = copy.deepcopy(fresh)
    expected = copy.deepcopy(fresh)
    if matching:
        target = expected['bindings'][bindings.index(matching[0])]
        members = [m for m in target["members"] if m != MEMBER]
        if members:
            target["members"] = members
        else:
            expected["bindings"].remove(target)
    intent_path = _intent(d, run_id, "revoke", matching[0] if matching else {}, baseline, expected)
    if matching:
        payload = copy.deepcopy(expected)
        payload["etag"] = fresh["etag"]
        policy_path = _write_policy(d, payload, "revoke")
        api.set(policy_path)
    observed = _canonical(api.get())
    _require(not any(b.get("role") == ROLE and MEMBER in b.get("members", [])
                     and b.get("condition", {}).get("title") == "v84-recovery-" + run_id
                     for b in observed.get("bindings", [])), "revoke readback still contains run-owned access")
    # Concurrent unrelated bindings are allowed; every baseline binding not
    # being removed must remain semantically present in readback.
    for b in expected.get("bindings", []):
        _require(any(_semantic({"bindings": [x]}) == _semantic({"bindings": [b]}) for x in observed.get("bindings", [])),
                 "revoke readback lost a retained binding")
    _require(_semantic({k: v for k, v in expected.items() if k != 'bindings'})
             == _semantic({k: v for k, v in observed.items() if k != 'bindings'}),
             'revoke readback changed retained policy metadata or audit configuration')
    return {"action": "revoke", "run_id": run_id, "secret": SECRET, "member": MEMBER,
            "role": ROLE, "changed": bool(matching), "intent": str(intent_path), "verified": True}
