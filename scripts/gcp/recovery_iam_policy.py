"""Pure builders for a narrowly scoped, temporary recovery IAM binding."""

import copy
import datetime as dt
import json
import re

MEMBER = "serviceAccount:espacigo-release-job-builder@espacigo-testprod.iam.gserviceaccount.com"
PROJECT = "espacigo-testprod"
PROJECT_NUMBER = "447373903153"
BUCKET = "espacigo-testprod-cloud-build-447373903153"
REGION = "southamerica-west1"
JOB = "espacigo-db-r-2dc002a95831471780646432b98d3e01"
EXECUTION = JOB + "-v62zb"
PERMISSIONS = frozenset({"storage.objects.get"})
LEGACY_PERMISSIONS = frozenset({"storage.objects.get", "run.executions.get", "run.executions.list"})
UUID = re.compile(r"[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}")
ROLE = re.compile(r"projects/espacigo-testprod/roles/espacigoRecoveryRead20261107[a-f0-9]{8}")


def _aware_utc(value, label):
    if isinstance(value, str):
        try:
            value = dt.datetime.fromisoformat(value.replace("Z", "+00:00"))
        except ValueError as exc:
            raise ValueError(f"{label} must be RFC3339") from exc
    if not isinstance(value, dt.datetime) or value.tzinfo is None or value.utcoffset() is None:
        raise ValueError(f"{label} must be timezone-aware")
    return value.astimezone(dt.timezone.utc)


def _resources(parent_bid, legacy=False):
    storage = (
        f"projects/_/buckets/{BUCKET}/objects/cloudrun-main.lock",
        f"projects/_/buckets/{BUCKET}/objects/recovery-inspections/{parent_bid}/job-request.json",
    )
    if not legacy:
        return storage
    return storage + (
        f"projects/{PROJECT}/locations/{REGION}/jobs/{JOB}",
        f"projects/{PROJECT_NUMBER}/locations/{REGION}/jobs/{JOB}",
        f"projects/{PROJECT}/locations/{REGION}/jobs/{JOB}/executions/{EXECUTION}",
        f"projects/{PROJECT_NUMBER}/locations/{REGION}/jobs/{JOB}/executions/{EXECUTION}",
    )


def _binding(parent_bid, role_name, expires_at, legacy=False):
    expiry = _aware_utc(expires_at, "expires_at")
    selectors = " || ".join(f"resource.name == {json.dumps(name)}" for name in _resources(parent_bid, legacy))
    expression = f"request.time < timestamp({json.dumps(expiry.isoformat().replace('+00:00', 'Z'))}) && ({selectors})"
    return {"role": role_name, "members": [MEMBER], "condition": {"title": "recovery-readonly-" + parent_bid, "description": "Temporary exact-resource recovery read access", "expression": expression}}


def build_binding(parent_bid, role_name, expires_at, now=None):
    if not isinstance(parent_bid, str) or UUID.fullmatch(parent_bid) is None:
        raise ValueError("parent_bid must be a lowercase UUID")
    if not isinstance(role_name, str) or ROLE.fullmatch(role_name) is None:
        raise ValueError("role_name must be the exact recovery custom-role form")
    expiry = _aware_utc(expires_at, "expires_at")
    current = _aware_utc(now if now is not None else dt.datetime.now(dt.timezone.utc), "now")
    if expiry <= current or expiry > current + dt.timedelta(hours=2):
        raise ValueError("expiry must be in the future and no more than two hours away")
    return _binding(parent_bid, role_name, expiry)


def _validate_policy(policy):
    if not isinstance(policy, dict) or not isinstance(policy.get("etag"), str) or not policy.get("etag"):
        raise ValueError("policy requires a nonempty etag")
    if policy.get("version") not in (1, 3):
        raise ValueError("policy version must be 1 or 3")
    if not isinstance(policy.get("bindings", []), list):
        raise ValueError("policy bindings must be a list")
    for binding in policy.get('bindings', []):
        if not isinstance(binding, dict) or not isinstance(binding.get('role'), str) or not binding['role'] or '_withcond_' in binding['role']:
            raise ValueError('existing binding role is malformed or conditions are not expanded')
        members = binding.get('members')
        if not isinstance(members, list) or not members or not all(isinstance(x, str) and x for x in members):
            raise ValueError('existing binding members are malformed')
        if 'condition' in binding:
            condition = binding['condition']
            if policy['version'] != 3 or not isinstance(condition, dict) or not isinstance(condition.get('expression'), str) or not condition['expression']:
                raise ValueError('existing condition is not intact in a version-3 policy')
            if not set(condition) <= {'expression', 'title', 'description', 'location'} or not all(isinstance(v, str) for v in condition.values()):
                raise ValueError('existing condition fields are malformed')


def _validate_binding(binding, allow_legacy=False):
    if not isinstance(binding, dict) or set(binding) != {"role", "members", "condition"}:
        raise ValueError("binding has unexpected fields")
    if not isinstance(binding["role"], str) or ROLE.fullmatch(binding["role"]) is None:
        raise ValueError("invalid recovery role")
    if binding["members"] != [MEMBER]:
        raise ValueError("binding must name only the authorized principal")
    c = binding["condition"]
    if not isinstance(c, dict) or set(c) != {"title", "description", "expression"}:
        raise ValueError("invalid condition shape")
    bid = c.get("title", "").removeprefix("recovery-readonly-")
    if UUID.fullmatch(bid) is None or c["title"] != "recovery-readonly-" + bid or c["description"] != "Temporary exact-resource recovery read access":
        raise ValueError("invalid recovery condition identity")
    expression = c.get("expression")
    match = re.match(r'^request\.time < timestamp\(("[^"\\]+")\) && \(', expression) if isinstance(expression, str) else None
    if match is None:
        raise ValueError("invalid canonical condition expiry")
    try:
        expiry = _aware_utc(json.loads(match[1]), "condition expiry")
    except (ValueError, TypeError, json.JSONDecodeError) as exc:
        raise ValueError("invalid condition expiry") from exc
    # Reconstruct the WHOLE expression. Counting permitted fragments would
    # accept appended `|| true` and grant outside the exact resource set.
    expected = build_binding(bid, binding['role'], expiry, now=expiry - dt.timedelta(hours=1))
    if binding != expected and allow_legacy:
        expected = _binding(bid, binding['role'], expiry, legacy=True)
    if binding != expected:
        raise ValueError("condition must exactly equal the canonical resource allowlist")
    return bid, expiry


def grant_policy(policy, binding, now=None):
    _validate_policy(policy)
    bid, expiry = _validate_binding(binding)
    build_binding(bid, binding['role'], expiry, now=now)
    result = copy.deepcopy(policy)
    bindings = result.setdefault("bindings", [])
    if any(item.get("role") == binding["role"] for item in bindings if isinstance(item, dict)):
        raise ValueError("recovery role already bound")
    if any(item == binding for item in bindings):
        raise ValueError("duplicate binding")
    result["version"] = 3
    bindings.append(copy.deepcopy(binding))
    return result


def revoke_policy(policy, binding):
    _validate_policy(policy)
    _validate_binding(binding, allow_legacy=True)
    result = copy.deepcopy(policy)
    bindings = result.setdefault("bindings", [])
    matches = [i for i, item in enumerate(bindings) if item == binding]
    if len(matches) != 1:
        raise ValueError("expected exactly one own binding")
    del bindings[matches[0]]
    return result


def semantic_policy(policy):
    _validate_policy(policy)
    result = copy.deepcopy(policy)
    result.pop("etag", None)
    result.pop("version", None)
    if isinstance(result.get("bindings"), list):
        normalized = []
        for binding in result["bindings"]:
            item = copy.deepcopy(binding)
            if isinstance(item, dict) and isinstance(item.get("members"), list):
                item["members"] = sorted(item["members"])
            normalized.append(item)
        result["bindings"] = sorted(normalized, key=lambda item: json.dumps(item, sort_keys=True, separators=(",", ":")))
    return result
