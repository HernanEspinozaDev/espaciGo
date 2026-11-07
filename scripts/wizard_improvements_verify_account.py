"""Verify one authorized pending synthetic account through local Mailpit.

This module never registers, reissues, logs in, or prints credentials.
"""
from __future__ import annotations
import json, os, re, stat, tempfile, urllib.parse
from pathlib import Path

API = "http://127.0.0.1:18086/api/v1"
MAILPIT_BASE = "http://127.0.0.1:18087/api/v1"
MAILPIT = MAILPIT_BASE + "/messages"
RECEIPT_NAME = "wizard-improvements-pending.json"
INVENTORY_NAME = "wizard-improvements-account-inventory.json"
TARGET = {"project":"espacigo_wizard_20261106","database":"espacigo_wizard_20261106","host":"127.0.0.1","port":15436}

class VerificationError(RuntimeError):
    def __init__(self, code):
        self.code = code
        super().__init__(code)

def _fail(code): raise VerificationError(code)
def _safe_path(path, root):
    root = Path(root).resolve(strict=True)
    path = Path(path)
    if not path.is_absolute(): _fail("unsafe_path")
    # Check lexical components before resolve(), which would hide symlink ancestors.
    current = Path(path.anchor)
    for part in path.parts[1:]:
        current = current / part
        try: component = current.lstat()
        except FileNotFoundError:
            _fail("unsafe_path")
            raise AssertionError("unreachable")
        if stat.S_ISLNK(component.st_mode): _fail("unsafe_path")
    resolved = path.resolve(strict=True)
    if not resolved.is_relative_to(root / ".local" / "testing") or not resolved.is_file(): _fail("unsafe_path")
    st = resolved.stat()
    if st.st_uid != os.getuid() or stat.S_IMODE(st.st_mode) != 0o600: _fail("unsafe_permissions")
    current = resolved.parent
    while current != root and current.is_relative_to(root):
        if current.is_symlink() or stat.S_IMODE(current.stat().st_mode) != 0o700: _fail("unsafe_permissions")
        current = current.parent
    return resolved
def _atomic_json(path, value):
    fd, name = tempfile.mkstemp(prefix=".account-inventory-", dir=path.parent)
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd,"w",encoding="utf-8") as f:
            json.dump(value,f,sort_keys=True); f.write("\n"); f.flush(); os.fsync(f.fileno())
        os.replace(name,path); os.chmod(path,0o600)
        d=os.open(path.parent,os.O_RDONLY); os.fsync(d); os.close(d)
    finally:
        if os.path.exists(name): os.unlink(name)
def _mail_items(response):
    body=response.json()
    if isinstance(body,dict):
        items=body.get("messages",body.get("Messages"))
    else: items=None
    if not isinstance(items,list): _fail("mail_format")
    return items
def verify_pending_account(receipt_path, request, timeout=30):
    """Consume the matching Mailpit message and verify this pending account.

    request(method, url, payload=None, timeout=8) must return status_code/json().
    Raises VerificationError with a fixed, non-sensitive code; secrets never logged.
    """
    receipt_path=Path(receipt_path)
    root=Path(__file__).resolve().parents[1]
    try:
        receipt_path=_safe_path(receipt_path,root)
        if receipt_path.name != RECEIPT_NAME: _fail("unsafe_path")
        data=json.loads(receipt_path.read_text())
        if not isinstance(data,dict) or data.get("status") not in ("pending","awaiting_verification"): _fail("not_pending")
        email=data.get("email"); password=data.get("password")
        if not isinstance(email,str) or not re.fullmatch(r"[^@\s]+@example\.invalid",email) or not isinstance(password,str) or not password: _fail("invalid_receipt")
        storage=receipt_path.parent
        policy_path=root/".local/testing/test-account-policy.json"
        policy_path=_safe_path(policy_path,root)
        policy=json.loads(policy_path.read_text())
        relative=Path(policy.get("private_storage",""))
        if relative.is_absolute() or ".." in relative.parts or relative.parts[:2] != (".local","testing") or len(relative.parts) <= 2: _fail("unsafe_path")
        expected_storage=root/relative
        if receipt_path.parent != expected_storage: _fail("unsafe_path")
        auth=policy.get("authorization",{})
        if policy.get("target") != TARGET or auth.get("maximum_new_accounts") != 2 or auth.get("synthetic_email_domain") != "example.invalid" or not (isinstance(auth.get("verification"),str) and "normal" in auth["verification"].lower() and "only" in auth["verification"].lower()): _fail("policy_denied")

        inv_path=storage/INVENTORY_NAME
        inv=[]
        if inv_path.exists():
            if inv_path.is_symlink() or stat.S_IMODE(inv_path.stat().st_mode)!=0o600: _fail("unsafe_permissions")
            inv=json.loads(inv_path.read_text())
            if not isinstance(inv,list) or len(inv)>=2 or any(not isinstance(x,dict) or not isinstance(x.get("email"),str) for x in inv): _fail("account_limit")
        if any(x.get("email")==email for x in inv): _fail("already_in_inventory")
        listing=request("GET",MAILPIT,timeout=min(timeout,8))
        if listing.status_code!=200: _fail("mail_unavailable")
        messages=_mail_items(listing)
        matches=[]
        for m in messages:
            if not isinstance(m,dict): continue
            tos=m.get("To",m.get("to",[]))
            if isinstance(tos,dict): tos=[tos]
            exact=False
            if isinstance(tos,list):
                exact=any(isinstance(a,dict) and (a.get("Address")==email or a.get("address")==email) for a in tos)
            if exact: matches.append(m)
        if len(matches)!=1: _fail("matching_message_unavailable")
        msg=matches[0]; mid=msg.get("ID",msg.get("id"))
        if not isinstance(mid,(str,int)) or not str(mid): _fail("mail_format")
        detail=request("GET",MAILPIT_BASE+"/message/"+urllib.parse.quote(str(mid),safe=""),timeout=min(timeout,8))
        if detail.status_code!=200: _fail("mail_unavailable")
        body=detail.json()
        # Mailer formats exact plaintext labels; capture only these two expected fields.
        text=body.get("Text",body.get("text")) if isinstance(body,dict) else None
        if not isinstance(text,str): _fail("mail_format")
        text=text.replace("\r\n", "\n")
        ids=re.findall(r"(?m)^Token ID: ([0-9a-fA-F-]{36})[ \t]*$",text)
        tokens=re.findall(r"(?m)^Token: ([A-Za-z0-9_-]+)[ \t]*$",text)
        if len(ids)!=1 or len(tokens)!=1: _fail("mail_format")
        response=request("POST",API+"/auth/verification",{"token_id":ids[0],"token":tokens[0]},timeout=timeout)
        if response.status_code!=204: _fail("verification_rejected")
        item={"email":email,"password":password}
        if data.get("account_id") is not None: item["account_id"]=data["account_id"]
        inv.append(item)
        _atomic_json(inv_path,inv)
        data["status"]="verified"
        _atomic_json(receipt_path,data)
        return {"status":"verified"}
    except VerificationError: raise
    except Exception:
        _fail("verification_failed")
