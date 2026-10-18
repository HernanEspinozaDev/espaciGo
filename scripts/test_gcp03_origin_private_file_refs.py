from __future__ import annotations

import unittest

from gcp03_origin_private_file_refs import PRIVATE_FILE_REFS_SQL
from gcp03_origin_recovery_plan import (
    build_private_file_continuation_plan,
    execute_private_file_continuation,
    REQUIRED_OWNER_POSTCHECKS,
    REQUIRED_SANITATION_COUNTS,
)
from gcp03_origin_private_files import PrivateFile


class PrivateFileReferenceTests(unittest.TestCase):
    def test_live_evidence_uses_archive_id_and_retirement_queue_stays_live(self):
        self.assertIn("SELECT archivo_id FROM public.verificacion_evidencia_sintetica WHERE archivo_id IS NOT NULL", PRIVATE_FILE_REFS_SQL)
        self.assertNotIn("SELECT id FROM public.verificacion_evidencia_sintetica", PRIVATE_FILE_REFS_SQL)
        self.assertIn("SELECT archivo_id FROM public.verificacion_archivo_retiro_pendiente_local WHERE archivo_id IS NOT NULL", PRIVATE_FILE_REFS_SQL)

    def test_continuation_rechecks_plan_and_only_copies(self):
        file_id = "0f8fad5b-d9cb-469f-a165-70867728950e"
        item = PrivateFile(f"runs/g03o-1017/{file_id}.blob", b"synthetic", "b3cc0475bb78a5026098858e9889acf666d31062d513d303314eca31d36e72f2")
        checks = {key: True for key in REQUIRED_OWNER_POSTCHECKS}
        sanitation = {key: 0 for key in REQUIRED_SANITATION_COUNTS} | {"runtime_role_login": False}
        migration_sha = "a" * 64
        role_checks = {"session_user_is_postgres": True, "current_user_is_runtime": True,
            "runtime_no_login": True, "postgres_runtime_set_only": True}
        plan = build_private_file_continuation_plan(references={file_id}, private_files=[item],
            run_id="g03o-1017", normalized_checks=checks, sanitation_counts=sanitation,
            migration_history_sha256=migration_sha, role_checks=role_checks)

        class Store:
            def __init__(self): self.created = []
            def create(self, object_name, data, sha256): self.created.append(object_name)

        class Database:
            def normalize_owners_and_acls(self, *_): raise AssertionError("continuation cannot normalize")
            def sanitize(self, *_): raise AssertionError("continuation cannot sanitize")

        store = Store()
        result = execute_private_file_continuation(plan=plan, approved_plan_sha256=plan["plan_sha256"],
            references={file_id}, private_files=[item], normalized_checks=checks,
            sanitation_counts=sanitation, run_id="g03o-1017", store_factory=lambda: store,
            migration_history_sha256=migration_sha, role_checks=role_checks)
        self.assertTrue(result["phase_success"])
        self.assertEqual(store.created, [item.object_name])


if __name__ == "__main__":
    unittest.main()
