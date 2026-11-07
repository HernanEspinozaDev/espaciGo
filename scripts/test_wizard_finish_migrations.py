import hashlib
import importlib.util
import json
import signal
import tempfile
import unittest
from pathlib import Path

SPEC = importlib.util.spec_from_file_location(
    "finish", Path(__file__).with_name("wizard-improvements-finish-migrations.py")
)
m = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(m)


class FinishMigrationsTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.priv = self.root / ".local" / "testing" / "wizard-improvements"
        self.pg = self.priv / "postgres"
        self.pg.mkdir(parents=True)
        for path in (self.root / ".local", self.root / ".local/testing", self.priv, self.pg):
            path.mkdir(exist_ok=True)
            path.chmod(0o700)

        m.ROOT = self.root
        m.PRIVATE = self.priv
        m.ADMIN = self.pg / "pgpass"
        m.MIGRATOR = self.priv / "espacigo_migrator_login.pgpass"
        m.MANIFEST = self.priv / "reviewed-full-83-manifest.json"
        m.BOOT = self.root / "db/bootstrap/wizard-improvements"
        m.MIGRATIONS = self.root / "db/migrations"
        m.BOOT.mkdir(parents=True)
        m.MIGRATIONS.mkdir(parents=True)
        for path in (m.ADMIN, m.MIGRATOR):
            path.write_text("fixture\n")
            path.chmod(0o600)

        entries = []
        for version in range(1, 84):
            path = m.MIGRATIONS / f"V{version:06d}__canonical.sql"
            path.write_text(f"-- fixture {version}\n")
            entries.append({
                "version": version,
                "file": path.name,
                "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            })
        m.MANIFEST.write_text(json.dumps(entries))
        m.MANIFEST.chmod(0o600)
        for name in ("005_allow_v82_v83_create.sql", "006_revoke_v82_v83_create.sql"):
            (m.BOOT / name).write_text("-- fixture\n")

        self.binary = self.root / "migrator"
        self.binary.write_text("#!/bin/sh\nexit 0\n")
        self.binary.chmod(0o700)
        self.digest = hashlib.sha256(self.binary.read_bytes()).hexdigest()
        self.old_handlers = {sig: signal.getsignal(sig) for sig in (signal.SIGINT, signal.SIGTERM)}

    def tearDown(self):
        self.assertEqual(
            {sig: signal.getsignal(sig) for sig in self.old_handlers}, self.old_handlers,
            "main must restore both pre-existing signal handlers",
        )
        self.tmp.cleanup()

    def invoke(self, runner):
        return m.main([
            "--confirm", m.TARGET,
            "--migrator-binary", str(self.binary),
            "--migrator-sha256", self.digest,
        ], runner)

    def runner(self, *, migration_rc=0, grant_rc=0, cleanup_rc=0, before_mutate=None, after_mutate=None):
        calls = []
        manifest = json.loads(m.MANIFEST.read_text())
        def expected_rows(start, end):
            return [{"version": e["version"], "name": e["file"], "checksum": e["sha256"]}
                    for e in manifest[start - 1:end]]
        def run(cmd, env, timeout):
            calls.append(cmd)
            if "-plan-only" in cmd:
                is_final = len([c for c in calls if "-plan-only" in c]) == 2
                data = {"database": m.TARGET, "read_only": True, "history_exists": True}
                if is_final:
                    data.update(applied=expected_rows(1, 83), pending=None)
                    if after_mutate:
                        after_mutate()
                else:
                    data.update(applied=expected_rows(1, 81), pending=expected_rows(82, 83))
                    if before_mutate:
                        before_mutate(data)
                return m.subprocess.CompletedProcess(cmd, 0, json.dumps(data), "")
            if cmd[0] == "psql" and cmd[-1].endswith("005_allow_v82_v83_create.sql"):
                return m.subprocess.CompletedProcess(cmd, grant_rc, "", "")
            if cmd[0] == "psql" and cmd[-1].endswith("006_revoke_v82_v83_create.sql"):
                return m.subprocess.CompletedProcess(cmd, cleanup_rc, "", "")
            return m.subprocess.CompletedProcess(cmd, migration_rc, "", "")
        return run, calls

    def test_success_checks_final_plan_and_revokes(self):
        run, calls = self.runner()
        self.assertEqual(self.invoke(run), 0)
        self.assertTrue(calls[-1][-1].endswith("006_revoke_v82_v83_create.sql"))

    def test_migration_failure_still_revokes(self):
        run, calls = self.runner(migration_rc=1)
        self.assertEqual(self.invoke(run), 1)
        self.assertTrue(calls[-1][-1].endswith("006_revoke_v82_v83_create.sql"))

    def test_grant_failure_still_revokes(self):
        run, calls = self.runner(grant_rc=1)
        self.assertEqual(self.invoke(run), 1)
        self.assertTrue(calls[-1][-1].endswith("006_revoke_v82_v83_create.sql"))

    def test_cleanup_failure_is_reported(self):
        run, calls = self.runner(cleanup_rc=1)
        self.assertEqual(self.invoke(run), 1)
        self.assertTrue(calls[-1][-1].endswith("006_revoke_v82_v83_create.sql"))

    def test_wrong_confirmation_never_calls_database(self):
        calls = []
        self.assertEqual(m.main(["--confirm", "wrong", "--migrator-binary", str(self.binary),
                                 "--migrator-sha256", self.digest], lambda *args: calls.append(args)), 1)
        self.assertEqual(calls, [])

    def test_wrong_preflight_plan_never_grants(self):
        def corrupt(data): data["database"] = "other_database"
        run, calls = self.runner(before_mutate=corrupt)
        self.assertEqual(self.invoke(run), 1)
        self.assertFalse(any(c[0] == "psql" for c in calls))

    def test_adulterated_applied_checksum_never_grants(self):
        def corrupt(data): data["applied"][0]["checksum"] = "0" * 64
        run, calls = self.runner(before_mutate=corrupt)
        self.assertEqual(self.invoke(run), 1)
        self.assertFalse(any(c[0] == "psql" for c in calls))

    def test_pending_migration_rejects_final_plan_but_revokes(self):
        def corrupt(): pass
        run, calls = self.runner(after_mutate=lambda: None)
        original = run
        def pending_runner(cmd, env, timeout):
            result = original(cmd, env, timeout)
            if "-plan-only" in cmd and len([c for c in calls if "-plan-only" in c]) == 2:
                result.stdout = json.dumps({"database": m.TARGET, "read_only": True, "history_exists": True,
                                            "applied": [{"version": i, "name": f"V{i:06d}__canonical.sql",
                                                         "checksum": json.loads(m.MANIFEST.read_text())[i-1]["sha256"]}
                                                        for i in range(1, 84)],
                                            "pending": [{"version": 84}]})
            return result
        self.assertEqual(self.invoke(pending_runner), 1)
        self.assertTrue(calls[-1][-1].endswith("006_revoke_v82_v83_create.sql"))

    def test_sigterm_during_grant_restores_and_revokes(self):
        observed = {}
        run, calls = self.runner()
        base = run
        def interrupting(cmd, env, timeout):
            if cmd[0] == "psql" and cmd[-1].endswith("005_allow_v82_v83_create.sql"):
                observed[signal.SIGTERM] = signal.getsignal(signal.SIGTERM)
                observed[signal.SIGTERM](signal.SIGTERM, None)
            return base(cmd, env, timeout)
        self.assertEqual(self.invoke(interrupting), 1)
        self.assertIsNot(observed[signal.SIGTERM], signal.SIG_DFL)
        self.assertTrue(calls[-1][-1].endswith("006_revoke_v82_v83_create.sql"))

    def test_sigint_during_migration_restores_and_revokes(self):
        observed = {}
        run, calls = self.runner()
        def interrupting(cmd, env, timeout):
            if cmd[0] == str(self.binary) and "-plan-only" not in cmd:
                observed[signal.SIGINT] = signal.getsignal(signal.SIGINT)
                observed[signal.SIGINT](signal.SIGINT, None)
            return run(cmd, env, timeout)
        self.assertEqual(self.invoke(interrupting), 1)
        self.assertIsNot(observed[signal.SIGINT], signal.SIG_DFL)
        self.assertTrue(calls[-1][-1].endswith("006_revoke_v82_v83_create.sql"))


if __name__ == "__main__":
    unittest.main()
