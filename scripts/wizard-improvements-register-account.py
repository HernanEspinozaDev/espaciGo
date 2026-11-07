#!/usr/bin/env python3
"""Create one synthetic local wizard test account; never run without --confirm."""
import argparse
import json
import os
import secrets
import stat
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

TARGET = {"project": "espacigo_wizard_20261106", "database": "espacigo_wizard_20261106", "host": "127.0.0.1", "port": 15436}
API = "http://127.0.0.1:18086/api/v1"
EXPECTED_TYPES = {"terminos", "privacidad"}


class PendingReceiptError(RuntimeError):
    pass


def policy_allows(policy):
    if not isinstance(policy, dict) or policy.get("target") != TARGET:
        return False
    auth = policy.get("authorization")
    if not isinstance(auth, dict) or auth.get("maximum_new_accounts") != 2 or auth.get("synthetic_email_domain") != "example.invalid":
        return False
    if not isinstance(auth.get("account_inventory"), list):
        return False
    for key in ("registration", "verification"):
        value = auth.get(key)
        if not isinstance(value, str) or "normal" not in value.lower() or "only" not in value.lower():
            return False
    storage = policy.get("private_storage")
    return isinstance(storage, str) and bool(storage) and not Path(storage).is_absolute()


def _request(session, method, url, payload=None, timeout=8):
    try:
        if session is not None:
            return session.get(url, timeout=timeout) if method == "GET" else session.post(url, json=payload, timeout=timeout)
        data = None if payload is None else json.dumps(payload).encode()
        headers = {"Content-Type": "application/json"} if data else {}
        if method == "POST":
            headers["Origin"] = "https://127.0.0.1:4206"
        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        with urllib.request.urlopen(req, timeout=timeout) as response:
            status_code, body = response.status, response.read()
    except urllib.error.HTTPError as error:
        status_code, body = error.code, error.read()
    except urllib.error.URLError as exc:
        raise RuntimeError("local API request failed") from None

    class Response:
        def __init__(self, status_code, body):
            self.status_code, self._body = status_code, body
        def json(self):
            return json.loads(self._body)
    return Response(status_code, body)


def _private_dir(repo_root, storage):
    root = Path(repo_root).absolute()
    base = root / ".local" / "testing"
    # Inspect every existing component without following links; create only missing private dirs.
    for path in (root, root / ".local", base):
        try:
            info = path.lstat()
        except FileNotFoundError:
            if path == root:
                raise ValueError("repository root missing")
            path.mkdir(mode=0o700)
            info = path.lstat()
        if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode) or info.st_uid != os.getuid():
            raise PermissionError("private directory component is not owned and real")
        if path != root and stat.S_IMODE(info.st_mode) != 0o700:
            raise PermissionError("private directory permissions must be 0700")
    relative = Path(storage)
    if relative.is_absolute() or ".." in relative.parts or not relative.parts:
        raise ValueError("private_storage must remain under .local/testing/")
    parts = relative.parts
    if parts[:2] != (".local", "testing") or len(parts) <= 2:
        raise ValueError("private_storage must be a non-empty path under .local/testing/")
    current = base
    for part in parts[2:]:
        current = current / part
        try:
            info = current.lstat()
        except FileNotFoundError:
            current.mkdir(mode=0o700)
            info = current.lstat()
        if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o700:
            raise PermissionError("private storage must be owned real directories with mode 0700")
    if not current.resolve().is_relative_to(base.resolve()):
        raise ValueError("private_storage escaped .local/testing/")
    return current


def _write_private_json(path, value, exclusive=False):
    temp = path.with_name(path.name + ".tmp-" + secrets.token_hex(8))
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    fd = os.open(temp, flags, 0o600)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            json.dump(value, stream)
            stream.flush()
            os.fsync(stream.fileno())
        if exclusive:
            os.link(temp, path)
            temp.unlink()
        else:
            os.replace(temp, path)
        os.chmod(path, 0o600)
        dirfd = os.open(path.parent, os.O_RDONLY | getattr(os, "O_DIRECTORY", 0))
        try:
            os.fsync(dirfd)
        finally:
            os.close(dirfd)
    finally:
        try:
            temp.unlink()
        except FileNotFoundError:
            pass


def register_account(policy, repo_root=None, session=None, *, confirm=None):
    if confirm != TARGET["project"]:
        raise ValueError("explicit --confirm is required")
    if not policy_allows(policy):
        raise ValueError("policy target or authorization is not permitted")
    root = Path(repo_root) if repo_root is not None else Path(__file__).resolve().parents[1]
    storage = _private_dir(root, policy["private_storage"])
    auth = policy["authorization"]
    if len(auth["account_inventory"]) >= auth["maximum_new_accounts"]:
        raise ValueError("authorized account limit reached")
    inventory_file = storage / "wizard-improvements-account-inventory.json"
    if inventory_file.exists() or inventory_file.is_symlink():
        raise PendingReceiptError("account inventory exists; reconcile before any new registration")
    receipt = storage / "wizard-improvements-pending.json"
    if receipt.exists() or receipt.is_symlink():
        raise PendingReceiptError("pending receipt exists; reconcile before any retry")
    email = f"wizard-{secrets.token_hex(8)}@example.invalid"
    password = "Wz!9aA" + secrets.token_urlsafe(32)
    terms_resp = _request(session, "GET", API + "/auth/terms")
    if terms_resp.status_code != 200:
        raise RuntimeError("unable to obtain terms")
    items = terms_resp.json().get("items")
    if not isinstance(items, list) or len(items) != 2:
        raise RuntimeError("invalid terms response")
    found = {}
    for item in items:
        if not isinstance(item, dict) or item.get("type") not in EXPECTED_TYPES or item["type"] in found:
            raise RuntimeError("invalid terms response")
        doc = item.get("document")
        if not isinstance(item.get("id"), str) or not item["id"] or not isinstance(doc, dict) or doc.get("type") != item["type"] or not isinstance(doc.get("sections"), list) or not doc["sections"]:
            raise RuntimeError("invalid terms response")
        found[item["type"]] = item["id"]
    if set(found) != EXPECTED_TYPES or len(set(found.values())) != 2:
        raise RuntimeError("invalid terms response")
    receipt_data = {"status": "pending", "email": email, "password": password, "created_at": int(time.time())}
    _write_private_json(receipt, receipt_data, exclusive=True)
    response = _request(session, "POST", API + "/auth/register", {"email": email, "password": password, "terms_version_ids": [found["terminos"], found["privacidad"]]})
    if response.status_code not in (201, 202):
        raise RuntimeError("registration failed; private pending state retained")
    receipt_data["status"] = "awaiting_verification"
    _write_private_json(receipt, receipt_data)
    return receipt


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--confirm", required=True)
    args = parser.parse_args(argv)
    try:
        repo = Path(__file__).resolve().parents[1]
        policy_path = repo / ".local/testing/test-account-policy.json"
        info = policy_path.lstat()
        if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600 or info.st_uid != os.getuid():
            raise PermissionError("private policy file must be owned regular mode 0600")
        policy = json.loads(policy_path.read_text(encoding="utf-8"))
        receipt = register_account(policy, confirm=args.confirm)
    except Exception:
        print("registration failed; private pending state retained", file=sys.stderr)
        return 1
    print(f"Pending receipt created: {receipt}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
