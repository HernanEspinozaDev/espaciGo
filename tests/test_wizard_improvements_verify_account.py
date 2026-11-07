import importlib.util
from typing import cast
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock

ROOT = Path("/home/nandev/Documentos/projects/espaciGo-wizard-improvements")
SCRIPT = ROOT / "scripts/wizard_improvements_verify_account.py"
spec = importlib.util.spec_from_file_location("verify_account", SCRIPT)
assert spec is not None and spec.loader is not None
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class VerifyTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(dir=ROOT / ".local/testing")
        self.base = Path(self.tmp.name)
        os.chmod(self.base,0o700)
        self.root = self.base / "project"
        self.root.mkdir(mode=0o700)
        (self.root / "scripts").mkdir(parents=True)
        self.old_module_file = module.__file__
        module.__file__ = str(self.root / "scripts/wizard_improvements_verify_account.py")
        testing = self.root / ".local/testing"
        testing.mkdir(parents=True, mode=0o700)
        os.chmod(self.root / ".local",0o700)
        os.chmod(testing,0o700)
        self.storage = testing / "private"
        self.storage.mkdir(mode=0o700)
        os.chmod(self.storage,0o700)
        self.policy = testing / "test-account-policy.json"
        self.policy.write_text(json.dumps({"target":module.TARGET,"private_storage":".local/testing/private","authorization":{"maximum_new_accounts":2,"synthetic_email_domain":"example.invalid","verification":"normal-flow-only"}}))
        os.chmod(self.policy,0o600)
        self.receipt = self.storage / module.RECEIPT_NAME
        self.receipt.write_text(json.dumps({"status":"awaiting_verification","email":"wizard-test@example.invalid","password":"secret-synthetic","account_id":"acct-test"}))
        os.chmod(self.receipt,0o600)
    def tearDown(self):
        module.__file__ = self.old_module_file
        self.tmp.cleanup()
    def request_for(self, post_status=204):
        token_text="Correo sintetico\nToken ID: 123e4567-e89b-12d3-a456-426614174000\nToken: abc_DEF-123\n"
        def call(method,url,payload=None,timeout=8):
            r=Mock(status_code=200)
            if url == module.MAILPIT:
                r.json.return_value={"messages":[{"ID":"wrong","To":[{"Address":"other@example.invalid"}]},{"ID":"right","To":[{"Address":"wizard-test@example.invalid"}]}]}
            elif url == module.MAILPIT_BASE + "/message/right":
                r.json.return_value={"Text":token_text}
            elif method=="POST":
                r.status_code=post_status; r.json.return_value={}
            return r
        return call
    def test_crlf_mailer_body_is_parsed_and_verified(self):
        calls=[]; request=self.request_for()
        def crlf(*a,**kw):
            r=request(*a,**kw)
            if a[1] == module.MAILPIT_BASE + "/message/right":
                r.json.return_value={"Text":r.json.return_value["Text"].replace("\n", "\r\n")}
            calls.append(a)
            return r
        self.assertEqual(module.verify_pending_account(self.receipt,crlf),{"status":"verified"})
        self.assertTrue(any(x[0]=="POST" and x[1]==module.API+"/auth/verification" for x in calls))

    def test_matching_recipient_and_expected_token_format_verified(self):
        calls=[]; request=self.request_for()
        def track(*a,**kw): calls.append(a); return request(*a,**kw)
        self.assertEqual(module.verify_pending_account(self.receipt,track),{"status":"verified"})
        self.assertEqual([x[1] for x in calls if x[0]=="GET" and x[1].startswith(module.MAILPIT_BASE)], [module.MAILPIT,module.MAILPIT_BASE+"/message/right"])
        self.assertEqual(json.loads(self.receipt.read_text())["status"],"verified")
        inv=json.loads((self.storage/module.INVENTORY_NAME).read_text())
        self.assertEqual(inv,[{"email":"wizard-test@example.invalid","password":"secret-synthetic","account_id":"acct-test"}])
        self.assertEqual((self.storage/module.INVENTORY_NAME).stat().st_mode & 0o777,0o600)
    def test_timeout_preserves_pending_receipt_and_no_inventory(self):
        def timeout(*a,**kw): raise TimeoutError("must not leak")
        with self.assertRaises(module.VerificationError) as err: module.verify_pending_account(self.receipt,timeout)
        self.assertEqual(err.exception.code,"verification_failed")
        self.assertEqual(json.loads(self.receipt.read_text())["status"],"awaiting_verification")
        self.assertFalse((self.storage/module.INVENTORY_NAME).exists())
    def test_rejection_rolls_back_inventory_and_no_registration_post(self):
        calls=[]; request=self.request_for(422)
        def track(*a,**kw): calls.append((a[0],a[1])); return request(*a,**kw)
        with self.assertRaises(module.VerificationError): module.verify_pending_account(self.receipt,track)
        self.assertEqual([x for x in calls if x[0]=="POST"],[("POST",module.API+"/auth/verification")])
        self.assertEqual(json.loads(self.receipt.read_text())["status"],"awaiting_verification")
        self.assertFalse((self.storage/module.INVENTORY_NAME).exists())
    def test_secrets_not_in_exception_or_stdout(self):
        def timeout(*a,**kw): raise TimeoutError("secret-synthetic")
        with self.assertRaises(module.VerificationError) as err: module.verify_pending_account(self.receipt,timeout)
        self.assertNotIn("secret-synthetic",str(err.exception))
    def test_rejects_policy_storage_mismatch_before_http(self):
        calls=[]
        def track(*args,**kwargs): calls.append((args,kwargs)); return self.request_for()(*args,**kwargs)
        self.policy.write_text(json.dumps({"target":module.TARGET,"private_storage":".local/testing/other","authorization":{"maximum_new_accounts":2,"synthetic_email_domain":"example.invalid","verification":"normal-flow-only"}}))
        calls.clear()
        with self.assertRaises(module.VerificationError) as err: module.verify_pending_account(self.receipt,track)
        self.assertEqual(err.exception.code,"unsafe_path")
        self.assertEqual(calls,[])
    def test_rejects_symlinked_ancestor_before_resolving_receipt(self):
        alias = self.base / "alias"
        alias.symlink_to(self.storage, target_is_directory=True)
        with self.assertRaises(module.VerificationError) as err:
            module.verify_pending_account(alias / module.RECEIPT_NAME, self.request_for())
        self.assertEqual(err.exception.code,"unsafe_path")

if __name__ == "__main__": unittest.main()
