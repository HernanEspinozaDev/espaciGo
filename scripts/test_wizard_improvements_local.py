from __future__ import annotations

import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

SCRIPT = Path(__file__).with_name("wizard-improvements-local.py")
spec = importlib.util.spec_from_file_location("wizard_local", SCRIPT)
assert spec and spec.loader
wizard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wizard)


class Runner:
    def __init__(self, collision: bool = False, nonextension_count: int = 0, daemon_failure: bool = False, image_failure: bool = False) -> None:
        self.calls: list[tuple[list[str], dict[str, str] | None]] = []
        self.collision = collision
        self.nonextension_count = nonextension_count
        self.daemon_failure = daemon_failure
        self.image_failure = image_failure

    def __call__(self, argv: list[str], env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        self.calls.append((argv, env))
        if argv[:2] == ["docker", "info"] and self.daemon_failure:
            return subprocess.CompletedProcess(argv, 1, "", "daemon unavailable")
        if argv[:3] == ["docker", "image", "inspect"]:
            return subprocess.CompletedProcess(argv, 1 if self.image_failure else 0, "" if self.image_failure else wizard.IMAGE, "missing")
        if "ls" in argv:
            if self.collision and f"name={wizard.CONTAINER}" in argv:
                return subprocess.CompletedProcess(argv, 0, wizard.CONTAINER, "")
            return subprocess.CompletedProcess(argv, 0, "", "")
        if "inspect" in argv and not self.collision:
            if len(argv) > 2 and argv[1] == "container":
                stderr = f"Error: No such container: {argv[-1]}"
            elif len(argv) > 2 and argv[1] == "volume":
                stderr = f"Error response from daemon: get {argv[-1]}: no such volume"
            elif len(argv) > 2 and argv[1] == "network":
                stderr = f"Error response from daemon: network {argv[-1]} not found"
            else:
                stderr = "Error: No such object"
            return subprocess.CompletedProcess(argv, 1, "", stderr)
        if argv and argv[0] == "psql":
            output = f"{wizard.DB}|postgres" if "current_database(),session_user" in argv[-1] else f"{wizard.DB}|postgres||||{self.nonextension_count}"
            return subprocess.CompletedProcess(argv, 0, output, "")
        if argv and argv[0] == "pg_dump":
            Path(argv[argv.index("-f") + 1]).write_bytes(b"mock custom dump")
        if argv[:2] == ["pg_restore", "--list"]:
            return subprocess.CompletedProcess(argv, 0, "TOC", "")
        return subprocess.CompletedProcess(argv, 0, "ok", "")


class CreateEmptyTests(unittest.TestCase):
    def _isolated_failure_paths(self, root: Path):
        private = root / "private"
        private.mkdir(mode=0o700)
        return (
            patch.object(wizard, "PRIVATE_DIR", private),
            patch.object(wizard, "TRIAL_DIR", private / "postgres"),
            patch.object(wizard, "ADMIN_FILE", private / "postgres/admin-password"),
            patch.object(wizard, "PASSFILE", private / "postgres/pgpass"),
            patch.object(wizard, "RECEIPT", private / "postgres/receipt.json"),
            patch.object(wizard, "BACKUP", private / "postgres/empty.dump"),
            patch.object(wizard, "POLICY", root / "policy"),
            patch.object(wizard.shutil, "which", return_value="docker"),
            patch.object(wizard, "ports_free", return_value=[]),
        )

    def test_create_empty_failure_stages_preserve_resources_and_journal(self) -> None:
        import json

        cases = (
            ("volume-create", lambda argv: argv[:3] == ["docker", "volume", "create"]),
            ("network-create", lambda argv: argv[:3] == ["docker", "network", "create"]),
            ("container-start", lambda argv: argv[:2] == ["docker", "start"]),
            ("readiness-readback", lambda argv: argv[0] == "psql"),
            ("backup-verify", lambda argv: argv[0] == "pg_restore"),
        )
        for stage, predicate in cases + (("second-credential", None),):
            with self.subTest(stage=stage), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                private = root / "private"
                runner = Runner()
                failure = RuntimeError("sentinel-never-log")
                if stage == "second-credential":
                    original_write = wizard.write_private

                    def write_private(path: Path, data: bytes) -> None:
                        self.assertTrue(path.is_relative_to(root))
                        if path == wizard.PASSFILE:
                            raise failure
                        original_write(path, data)

                    write_patch = patch.object(wizard, "write_private", side_effect=write_private)
                else:
                    original_runner = runner

                    def failing_runner(argv, env=None):
                        if predicate(argv):
                            raise failure
                        return original_runner(argv, env)

                    def safe_write(path: Path, data: bytes) -> None:
                        self.assertTrue(path.is_relative_to(root))
                        original_write_for_runner(path, data)

                    write_patch = patch.object(wizard, "write_private", side_effect=safe_write)
                    original_write_for_runner = wizard.write_private
                    runner = failing_runner

                with patch.multiple(wizard, PRIVATE_DIR=private, TRIAL_DIR=private / "postgres",
                                    ADMIN_FILE=private / "postgres/admin-password", PASSFILE=private / "postgres/pgpass",
                                    RECEIPT=private / "postgres/receipt.json", BACKUP=private / "postgres/empty.dump",
                                    POLICY=root / "policy"), patch.object(wizard.shutil, "which", return_value="docker"), \
                     patch.object(wizard, "ports_free", return_value=[]), write_patch:
                    private.mkdir(mode=0o700)
                    wizard.POLICY.touch()
                    with self.assertRaises(RuntimeError) as caught:
                        wizard.create_empty(runner=runner, sleep=lambda _: None)
                    receipt = private / "create-empty-failure.json"
                    data = json.loads(receipt.read_text())
                    self.assertIs(caught.exception, failure)
                    self.assertEqual(receipt.stat().st_mode & 0o777, 0o600)
                    self.assertEqual(data["stage"], stage)
                    self.assertEqual(data["state"], "needs-manual-reconciliation")
                    self.assertFalse(data["cleanup_performed"])
                    self.assertEqual(data["error_class"], "RuntimeError")
                    self.assertNotIn("sentinel-never-log", receipt.read_text())
                    self.assertTrue((private / "postgres").is_dir())
                    self.assertTrue((private / "postgres/admin-password").exists())
                    self.assertEqual((private / "postgres/pgpass").exists(), stage != "second-credential")
                    recorded = original_runner.calls if stage != "second-credential" else runner.calls
                    self.assertFalse(any(argv[1:2] == ["rm"] or argv[1:2] == ["prune"] for argv, _ in recorded))
                    calls_before = len(recorded)
                    with self.assertRaises(FileExistsError):
                        wizard.create_empty(runner=runner, sleep=lambda _: None)
                    self.assertEqual(len(recorded), calls_before)

    def test_journal_failure_does_not_replace_original(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            private = root / "private"
            runner = Runner()
            failure = RuntimeError("sentinel-never-log")
            original_runner = runner

            def failing_runner(argv, env=None):
                if argv[:3] == ["docker", "volume", "create"]:
                    raise failure
                return original_runner(argv, env)

            original_write = wizard.write_private

            def safe_write(path: Path, data: bytes) -> None:
                self.assertTrue(path.is_relative_to(root))
                original_write(path, data)

            with patch.multiple(wizard, PRIVATE_DIR=private, TRIAL_DIR=private / "postgres",
                                ADMIN_FILE=private / "postgres/admin-password", PASSFILE=private / "postgres/pgpass",
                                RECEIPT=private / "postgres/receipt.json", BACKUP=private / "postgres/empty.dump",
                                POLICY=root / "policy"), patch.object(wizard.shutil, "which", return_value="docker"), \
                 patch.object(wizard, "ports_free", return_value=[]), \
                 patch.object(wizard, "write_private", side_effect=safe_write), \
                 patch.object(wizard, "write_failure_receipt", side_effect=OSError("journal failure")):
                private.mkdir(mode=0o700)
                wizard.POLICY.touch()
                with self.assertRaises(RuntimeError) as caught:
                    wizard.create_empty(runner=failing_runner, sleep=lambda _: None)
            self.assertIs(caught.exception, failure)
            self.assertTrue((private / "postgres").is_dir())
            self.assertTrue((private / "postgres/admin-password").exists())
            self.assertTrue((private / "postgres/pgpass").exists())

    def test_successful_empty_listing_proves_absence(self) -> None:
        runner = lambda argv, env=None: subprocess.CompletedProcess(argv, 0, "", "")
        for kind, name in (("container", wizard.CONTAINER), ("volume", wizard.VOLUME), ("network", wizard.NETWORK)):
            self.assertFalse(wizard.docker_exists("docker", kind, name, runner))

    def test_exact_name_listing_proves_collision_but_prefix_does_not(self) -> None:
        for kind, name in (("container", wizard.CONTAINER), ("volume", wizard.VOLUME), ("network", wizard.NETWORK)):
            for output, expected in ((name, True), ("other-" + name, False)):
                runner = lambda argv, env=None, output=output: subprocess.CompletedProcess(argv, 0, output, "")
                self.assertEqual(wizard.docker_exists("docker", kind, name, runner), expected)

    def test_failed_listing_fails_closed_regardless_of_stderr(self) -> None:
        runner = lambda argv, env=None: subprocess.CompletedProcess(argv, 1, "", "permission denied")
        with self.assertRaisesRegex(RuntimeError, "cannot establish container absence"):
            wizard.docker_exists("docker", "container", wizard.CONTAINER, runner)

    def test_unknown_or_permission_docker_errors_fail_closed(self) -> None:
        runner = lambda argv, env=None: subprocess.CompletedProcess(argv, 1, "", "unrecognized Docker warning")
        with self.assertRaisesRegex(RuntimeError, "cannot establish network absence"):
            wizard.docker_exists("docker", "network", wizard.NETWORK, runner)

    def test_real_empty_query_uses_non_extension_objects_only(self) -> None:
        source = SCRIPT.read_text()
        self.assertIn("count(*) FILTER (WHERE d.objid IS NULL)", source)
        self.assertIn("d.deptype='e'", source)

    def test_unexpected_nonextension_relation_aborts_before_dump(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp); runner = Runner(nonextension_count=1)
            (root / "private").mkdir(mode=0o700)
            with (patch.object(wizard, "PRIVATE_DIR", root / "private"), patch.object(wizard, "TRIAL_DIR", root / "private/postgres"),
                  patch.object(wizard, "ADMIN_FILE", root / "private/postgres/admin-password"), patch.object(wizard, "PASSFILE", root / "private/postgres/pgpass"),
                  patch.object(wizard, "POLICY", root / "policy"), patch.object(wizard.shutil, "which", return_value="docker"), patch.object(wizard, "ports_free", return_value=[])):
                wizard.POLICY.touch()
                with self.assertRaisesRegex(RuntimeError, "application relations"):
                    wizard.create_empty(runner=runner, sleep=lambda _: None)
            self.assertFalse(any(call[0] and call[0][0] == "pg_dump" for call in runner.calls))

    def test_daemon_and_pinned_image_are_checked_before_mutation(self) -> None:
        for runner, message in ((Runner(daemon_failure=True), "required local command failed"), (Runner(image_failure=True), "pinned Docker image")):
            with tempfile.TemporaryDirectory() as temp:
                root = Path(temp); (root / "private").mkdir(mode=0o700)
                with (patch.object(wizard, "PRIVATE_DIR", root / "private"), patch.object(wizard, "TRIAL_DIR", root / "private/postgres"), patch.object(wizard, "POLICY", root / "policy"),
                      patch.object(wizard.shutil, "which", return_value="docker"), patch.object(wizard, "ports_free", return_value=[])):
                    wizard.POLICY.touch()
                    with self.assertRaisesRegex(RuntimeError, message): wizard.create_empty(runner=runner)
                self.assertFalse(any(call[0][1:3] in (["volume", "create"], ["network", "create"], ["create", "--name"]) for call in runner.calls))

    def test_wrong_confirmation_never_calls_setup(self) -> None:
        with patch.object(wizard, "create_empty") as create:
            self.assertEqual(wizard.main(["create-empty", "--confirm", "wrong"]), 2)
            create.assert_not_called()

    def test_collision_fails_before_any_mutation(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            runner = Runner(collision=True)
            (root / "private").mkdir(mode=0o700)
            with (patch.object(wizard, "PRIVATE_DIR", root / "private"),
                  patch.object(wizard, "TRIAL_DIR", root / "private/postgres"),
                  patch.object(wizard, "POLICY", root / "policy"),
                  patch.object(wizard.shutil, "which", return_value="docker"),
                  patch.object(wizard, "ports_free", return_value=[])):
                wizard.POLICY.touch()
                with self.assertRaisesRegex(RuntimeError, "already exists"):
                    wizard.create_empty(runner=runner)
            self.assertEqual(len(runner.calls), 2)  # daemon info, then exact-name listing
            self.assertTrue((root / "private").is_dir())
            self.assertFalse((root / "private/postgres").exists())

    def test_success_uses_isolated_resources_and_validates_backup(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            runner = Runner()
            (root / "private").mkdir(mode=0o700)
            with (patch.object(wizard, "PRIVATE_DIR", root / "private"),
                  patch.object(wizard, "TRIAL_DIR", root / "private/postgres"),
                  patch.object(wizard, "ADMIN_FILE", root / "private/postgres/admin-password"),
                  patch.object(wizard, "PASSFILE", root / "private/postgres/pgpass"),
                  patch.object(wizard, "RECEIPT", root / "private/postgres/receipt.json"),
                  patch.object(wizard, "BACKUP", root / "private/postgres/empty.dump"),
                  patch.object(wizard, "POLICY", root / "policy"),
                  patch.object(wizard.shutil, "which", return_value="docker"),
                  patch.object(wizard, "ports_free", return_value=[])):
                wizard.POLICY.touch()
                wizard.create_empty(runner=runner, sleep=lambda _: None)
            commands = [call[0][1:] if call[0][0].endswith("docker") else call[0] for call in runner.calls]
            self.assertEqual([x for x in commands if x[:2] == ["volume", "create"]][0][-1], wizard.VOLUME)
            net = next(x for x in commands if x[:2] == ["network", "create"])
            self.assertIn("--internal", net)
            create = next(x for x in commands if x[:2] == ["create", "--name"])
            self.assertIn("127.0.0.1:15436:5432", create)
            mount = next(arg for arg in create if arg.startswith("type=volume"))
            self.assertTrue(mount.endswith("dst=/var/lib/postgresql"), mount)
            self.assertNotIn("/var/lib/postgresql/data", mount)
            self.assertIn("POSTGRES_PASSWORD_FILE=/run/secrets/admin", create)
            self.assertNotIn("--privileged", create)
            self.assertFalse(any("restart" in arg for arg in create))
            self.assertEqual((root / "private/postgres/admin-password").stat().st_mode & 0o777, 0o600)
            self.assertEqual((root / "private/postgres/pgpass").stat().st_mode & 0o777, 0o600)
            self.assertEqual((root / "private/postgres/empty.dump").stat().st_mode & 0o777, 0o600)
            self.assertEqual((root / "private/postgres/receipt.json").stat().st_mode & 0o777, 0o600)
            self.assertTrue(any(x[:2] == ["pg_restore", "--list"] and "--file=/dev/null" not in x for x in commands))
            self.assertTrue(any(x[:2] == ["pg_restore", "--file=/dev/null"] for x in commands))
            psql_envs = [env for argv, env in runner.calls if argv and argv[0] == "psql"]
            self.assertTrue(all(env is not None and "PGPASSWORD" not in env and "PGOPTIONS" not in env and "PGSERVICE" not in env for env in psql_envs))
            # Password values are never passed in argv; only a passfile path is put in env.
            self.assertTrue(all("POSTGRES_PASSWORD=" not in " ".join(x) for x in commands))
            self.assertTrue(all("PGPASSFILE" not in " ".join(x) for x in commands))


    def test_existing_private_parent_gets_new_subdir_and_preexisting_subdir_aborts(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "private"
            root.mkdir(mode=0o700)
            runner = Runner()
            with (patch.object(wizard, "PRIVATE_DIR", root),
                  patch.object(wizard, "TRIAL_DIR", root / "postgres"),
                  patch.object(wizard, "ADMIN_FILE", root / "postgres/admin-password"),
                  patch.object(wizard, "PASSFILE", root / "postgres/pgpass"),
                  patch.object(wizard, "RECEIPT", root / "postgres/receipt.json"),
                  patch.object(wizard, "BACKUP", root / "postgres/empty.dump"),
                  patch.object(wizard, "POLICY", root / "policy"),
                  patch.object(wizard.shutil, "which", return_value="docker"),
                  patch.object(wizard, "ports_free", return_value=[])):
                wizard.POLICY.touch()
                wizard.create_empty(runner=runner, sleep=lambda _: None)
                self.assertTrue((root / "postgres/receipt.json").exists())
                self.assertEqual((root.stat().st_mode & 0o777), 0o700)
                (root / "postgres").rename(root / "postgres-old")
                (root / "postgres").mkdir(mode=0o700)
                calls = len(runner.calls)
                with self.assertRaises(FileExistsError):
                    wizard.create_empty(runner=runner, sleep=lambda _: None)
                self.assertEqual(len(runner.calls), calls)


    def test_failure_receipt_is_private_redacted_and_durable(self) -> None:
        import json
        with tempfile.TemporaryDirectory() as temp:
            private = Path(temp) / "private"; private.mkdir(mode=0o700)
            wizard.write_failure_receipt(private, "container-start", "command-confirmed", RuntimeError("sentinel-message"))
            receipt = private / "create-empty-failure.json"
            data = json.loads(receipt.read_text())
            self.assertEqual(data["error_class"], "RuntimeError")
            self.assertNotIn("sentinel", receipt.read_text())
            self.assertEqual(receipt.stat().st_mode & 0o777, 0o600)

    def test_failure_receipt_rejects_unsafe_private_parent(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            parent = Path(temp) / "private"; parent.mkdir(mode=0o755)
            with self.assertRaisesRegex(RuntimeError, "mode 0700"):
                wizard.write_failure_receipt(parent, "x", "intent", RuntimeError())


    def test_preflight_failure_creates_no_receipt_or_trial_directory(self) -> None:
        import json
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp); private = root / "private"; private.mkdir(mode=0o700)
            runner = Runner(daemon_failure=True)
            with (patch.object(wizard, "PRIVATE_DIR", private), patch.object(wizard, "TRIAL_DIR", private / "postgres"),
                  patch.object(wizard, "ADMIN_FILE", private / "postgres/admin-password"), patch.object(wizard, "PASSFILE", private / "postgres/pgpass"),
                  patch.object(wizard, "RECEIPT", private / "postgres/receipt.json"), patch.object(wizard, "BACKUP", private / "postgres/dump"),
                  patch.object(wizard, "POLICY", root / "policy"), patch.object(wizard.shutil, "which", return_value="docker"),
                  patch.object(wizard, "ports_free", return_value=[])):
                wizard.POLICY.touch()
                with self.assertRaisesRegex(RuntimeError, "required local command failed"):
                    wizard.create_empty(runner)
            self.assertFalse((private / "create-empty-failure.json").exists())
            self.assertFalse((private / "postgres").exists())
            self.assertFalse((wizard.ROOT / ".local/testing/wizard-improvements/create-empty-failure.json").exists())


if __name__ == "__main__":
    unittest.main()
