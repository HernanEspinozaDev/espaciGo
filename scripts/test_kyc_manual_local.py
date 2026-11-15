import importlib.util
import os
import pathlib
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = pathlib.Path(__file__).with_name("kyc_manual_local.py")
spec = importlib.util.spec_from_file_location("kyc_manual_local", SCRIPT)
launcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(launcher)


class ManualLauncherTests(unittest.TestCase):
    def test_didit_env_parser_allows_only_three_literal_keys(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp) / "env"
            p.write_text("export DIDIT_API_KEY='key value'\nexport DIDIT_WEBHOOK_SECRET='hook value'\nexport DIDIT_BASE_URL='https://untrusted.invalid'\nexport DIDIT_WORKFLOW_ID=old\n")
            p.chmod(0o600)
            self.assertEqual(launcher.parse_didit_env(p), {
                "DIDIT_API_KEY": "key value", "DIDIT_WEBHOOK_SECRET": "hook value", "DIDIT_BASE_URL": "https://untrusted.invalid", "DIDIT_WORKFLOW_ID": "old"})

    def test_env_parser_rejects_duplicate_and_unapproved_keys_without_echo(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp) / "env"
            for text in ("export DIDIT_API_KEY=a\nexport DIDIT_API_KEY=b\n", "export DIDIT_API_KEY=secret\nexport EVIL=secret\n"):
                p.write_text(text)
                with self.assertRaises(launcher.LaunchError) as err:
                    launcher.parse_didit_env(p)
                self.assertNotIn("secret", str(err.exception))

    def test_private_file_validation_rejects_symlinks_and_permissive_mode(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp) / "private"
            root.mkdir(mode=0o700)
            file = root / "key.pem"
            file.write_text("synthetic")
            file.chmod(0o600)
            self.assertTrue(launcher.validate_private_file(file, root))
            file.chmod(0o644)
            with self.assertRaises(launcher.LaunchError):
                launcher.validate_private_file(file, root)
            file.chmod(0o600)
            link = root / "link"
            link.symlink_to(file)
            with self.assertRaises(launcher.LaunchError):
                launcher.validate_private_file(link, root)

    def test_frontend_environment_has_no_secret_or_database_keys(self):
        env = launcher.frontend_environment("/usr/bin/node")
        self.assertEqual(env["PORT"], "4201")
        self.assertEqual(env["HOSTNAME"], "127.0.0.1")
        self.assertFalse(any(k.startswith(("DIDIT", "PG", "DATABASE", "KEYRING")) for k in env))
        self.assertEqual(set(env), {"PATH", "HOME", "TMPDIR", "LANG", "NODE_ENV", "HOSTNAME", "PORT", "API_BACKEND_BASE_URL", "NEXT_TELEMETRY_DISABLED"})

    def test_api_environment_contract_overrides_workflow_and_uses_private_files(self):
        env = launcher.api_environment({"DIDIT_API_KEY":"synthetic", "DIDIT_WEBHOOK_SECRET":"hook", "DIDIT_WORKFLOW_ID":"old"})
        self.assertEqual(env["DIDIT_WORKFLOW_ID"], launcher.PUBLISHED_WORKFLOW_ID)
        self.assertEqual(env["HTTP_ADDR"], "127.0.0.1:18090")
        self.assertEqual(env["IDENTITY_DECLARATION_KEYRING_FILE"], str(launcher.MANUAL_ROOT / "declaration-keyring.json"))
        self.assertEqual(env["DATABASE_SET_ROLE"], "espacigo_runtime")
        self.assertEqual(env["DIDIT_WORKFLOW_VERSION"], "1")
        self.assertNotIn("DATABASE_URL", env)
        self.assertFalse(any(k.lower().startswith("legacy") for k in env))
        self.assertEqual(env["DIDIT_BASE_URL"], "https://verification.didit.me")

    def test_check_reports_prerequisites_without_loading_values(self):
        with patch.object(launcher, "validate_private_file"), patch.object(launcher, "parse_didit_env", side_effect=AssertionError("Didit env read")), patch.object(launcher.shutil, "which", return_value="/node"), patch.object(launcher, "port_free", return_value=True), patch.object(pathlib.Path, "is_file", return_value=True), patch.object(pathlib.Path, "is_dir", return_value=True):
            self.assertEqual(launcher.check_prerequisites("run-frontend"), "/node")

    def test_api_binary_is_under_manual_root(self):
        self.assertEqual(launcher.MANUAL_ROOT / "bin/api", launcher.api_binary_path())

    def test_executable_validation_requires_owned_regular_non_symlink_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            target = pathlib.Path(tmp) / "binary"
            target.write_text("synthetic")
            target.chmod(0o700)
            launcher.validate_executable(target)
            link = pathlib.Path(tmp) / "link"
            link.symlink_to(target)
            with self.assertRaises(launcher.LaunchError):
                launcher.validate_executable(link)
            target.chmod(0o600)
            with self.assertRaises(launcher.LaunchError):
                launcher.validate_executable(target)

    def test_mailbox_prerequisites_do_not_read_didit_file(self):
        with patch.object(launcher, "validate_executable"), patch.object(launcher, "parse_didit_env", side_effect=AssertionError("Didit env read")), patch.object(launcher.shutil, "which", return_value=None), patch.object(launcher, "port_free", return_value=True):
            self.assertIsNone(launcher.check_prerequisites("run-mailbox", False))

    def test_frontend_requires_standalone_static(self):
        with patch.object(launcher, "validate_private_file"), patch.object(launcher, "parse_didit_env", side_effect=AssertionError("Didit env read")), patch.object(launcher.shutil, "which", return_value="/node"), patch.object(launcher, "port_free", return_value=True), patch.object(pathlib.Path, "is_file", return_value=True), patch.object(pathlib.Path, "is_dir", autospec=True, side_effect=lambda p: p.name != "static"):
            with self.assertRaises(launcher.LaunchError):
                launcher.check_prerequisites("run-frontend")

    def test_exec_oserror_is_generic(self):
        with patch.object(launcher, "check_prerequisites", return_value="/node"), patch.object(launcher, "port_free", return_value=True), patch.object(launcher.os, "execve", side_effect=OSError("private path detail")), patch("sys.stderr", new_callable=__import__("io").StringIO) as err:
            self.assertEqual(launcher.main(["run-proxy"]), 1)
            self.assertNotIn("private path detail", err.getvalue())


if __name__ == "__main__":
    unittest.main()
