import os
import importlib.util
import hashlib
from unittest import mock
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[1] / "scripts/wizard-improvements-start-api.py"


def load_launcher():
    spec = importlib.util.spec_from_file_location("wizard_start_api", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class WizardAPIStartPreflightTests(unittest.TestCase):
    def test_has_no_home_evidence_path_or_source_substring_guard(self):
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertNotIn('Path.home() / ".local/testing/wizard-improvements/evidence"', source)
        self.assertNotIn("config_source", source)
        self.assertIn('EVIDENCE = TRIAL / "evidence"', source)

    def test_launcher_supplies_required_local_quota_and_exact_frontend_origin(self):
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn('"LOCAL_VERIFICATION_IP_LIMIT": "20"', source)
        self.assertIn('"CORS_ALLOWED_ORIGINS": "https://127.0.0.1:4206"', source)
        self.assertIn('"FRONTEND_PUBLIC_ORIGIN": "https://127.0.0.1:4206"', source)

    def test_rejects_uppercase_sha256_before_touching_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            trial = tmp / "trial"
            trial.mkdir(mode=0o700)
            os.chmod(trial, 0o700)
            (trial / "espacigo_api_login.pgpass").write_text("synthetic:placeholder\n")
            os.chmod(trial / "espacigo_api_login.pgpass", 0o600)
            script = SCRIPT.read_text(encoding="utf-8").replace(
                'TRIAL = ROOT / ".local/testing/wizard-improvements"', f'TRIAL = Path({str(trial)!r})'
            )
            altered = tmp / "launcher.py"
            altered.write_text(script, encoding="utf-8")
            binary = tmp / "dummy"
            binary.write_text("synthetic", encoding="utf-8")
            binary.chmod(0o700)
            module = load_launcher()
            module.TRIAL = trial
            module.PASSFILE = trial / "espacigo_api_login.pgpass"
            module.EVIDENCE = trial / "evidence"
            with mock.patch.object(sys, "argv", [str(altered), "--binary", str(binary), "--sha256", "A" * 64]), mock.patch.object(module, "port_free"), self.assertRaises(SystemExit) as raised:
                module.main()
            self.assertIn("64 lowercase hexadecimal", str(raised.exception))
            self.assertFalse((trial / "evidence").exists())

    def test_default_env_is_allowlisted_and_does_not_inspect_key(self):
        module = load_launcher()
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as tmp:
            trial = Path(tmp) / "trial"
            trial.mkdir(mode=0o700)
            trial.chmod(0o700)
            passfile = trial / "pass"
            passfile.write_text("synthetic\n")
            passfile.chmod(0o600)
            binary = Path(tmp) / "fake-api"
            binary.write_text("synthetic binary")
            binary.chmod(0o700)
            module.TRIAL, module.PASSFILE, module.EVIDENCE = trial, passfile, trial / "evidence"
            module.KEY_FILE = trial / "missing-key"
            argv = [str(SCRIPT), "--launch", "--binary", str(binary), "--sha256", hashlib.sha256(binary.read_bytes()).hexdigest()]
            with mock.patch.dict(os.environ, {"LOCAL_PUBLIC_CATALOG_TRIAL":"host", "LOCAL_PUBLIC_CATALOG_CURSOR_KEY_FILE":"host-key"}), mock.patch.object(sys, "argv", argv), mock.patch.object(module, "port_free"), mock.patch.object(module.os, "execve") as execve:
                module.main()
            env = execve.call_args.args[2]
            self.assertNotIn("LOCAL_PUBLIC_CATALOG_TRIAL", env)
            self.assertNotIn("LOCAL_PUBLIC_CATALOG_CURSOR_KEY_FILE", env)

    def test_optin_validates_key_and_sets_only_trial_env(self):
        module = load_launcher()
        with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as tmp:
            trial = Path(tmp) / "trial"
            trial.mkdir(mode=0o700); trial.chmod(0o700)
            passfile = trial / "pass"; passfile.write_text("synthetic\n"); passfile.chmod(0o600)
            key = trial / "public-catalog-cursor-key"; key.write_text("a" * 64); key.chmod(0o600)
            binary = Path(tmp) / "fake-api"; binary.write_text("synthetic binary"); binary.chmod(0o700)
            module.TRIAL, module.PASSFILE, module.EVIDENCE, module.KEY_FILE = trial, passfile, trial / "evidence", key
            argv = [str(SCRIPT), "--public-catalog", "--launch", "--binary", str(binary), "--sha256", hashlib.sha256(binary.read_bytes()).hexdigest()]
            with mock.patch.dict(os.environ, {}), mock.patch.object(sys, "argv", argv), mock.patch.object(module, "port_free"), mock.patch.object(module.os, "execve") as execve:
                module.main()
            env = execve.call_args.args[2]
            self.assertEqual(env["LOCAL_PUBLIC_CATALOG_TRIAL"], "1")
            self.assertEqual(env["LOCAL_PUBLIC_CATALOG_CURSOR_KEY_FILE"], str(key.resolve()))
            self.assertNotIn("a" * 64, repr(env))

    def test_optin_rejects_invalid_key_without_exec(self):
        for kind in ("missing", "mode", "symlink", "malformed"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as tmp:
                module = load_launcher(); trial = Path(tmp) / "trial"; trial.mkdir(mode=0o700); trial.chmod(0o700)
                passfile = trial / "pass"; passfile.write_text("synthetic\n"); passfile.chmod(0o600)
                key = trial / "public-catalog-cursor-key"
                if kind != "missing":
                    key.write_text("z" * 64 if kind == "malformed" else "a" * 64); key.chmod(0o644 if kind == "mode" else 0o600)
                if kind == "symlink":
                    target = trial / "target"; target.write_text("a" * 64); target.chmod(0o600); key.unlink(); key.symlink_to(target)
                binary = Path(tmp) / "fake-api"; binary.write_text("synthetic"); binary.chmod(0o700)
                module.TRIAL, module.PASSFILE, module.EVIDENCE, module.KEY_FILE = trial, passfile, trial / "evidence", key
                argv = [str(SCRIPT), "--public-catalog", "--launch", "--binary", str(binary), "--sha256", hashlib.sha256(binary.read_bytes()).hexdigest()]
                with mock.patch.object(sys, "argv", argv), mock.patch.object(module, "port_free"), mock.patch.object(module.os, "execve") as execve, self.assertRaises(SystemExit):
                    module.main()
                execve.assert_not_called()


if __name__ == "__main__":
    unittest.main()
