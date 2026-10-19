import copy
import hashlib
import json
import unittest

from gcp04_verify_baseline_bundle import (
    FORMAT,
    LEGACY_FORMAT,
    canonical_json,
    validate_manifest_format,
    verify_gadm_bootstrap_manifest,
)


class GadmManifestContractTests(unittest.TestCase):
    def test_legacy_format_is_preserved(self):
        manifest = {"format": LEGACY_FORMAT, "role_definitions_and_memberships_in_dump": False}
        self.assertEqual(LEGACY_FORMAT, validate_manifest_format(manifest))

    def test_current_format_is_preserved(self):
        manifest = {"format": FORMAT, "role_definitions_and_memberships_in_dump": False}
        self.assertEqual(FORMAT, validate_manifest_format(manifest))

    def setUp(self):
        self.comment = "NOLOGIN GADM local bootstrap group; temporary isolated login membership only."
        self.catalog = {"roles": [{"name": "espacigo_gadm_bootstrap", "login": False,
                                   "super": False, "comment": self.comment}]}
        self.manifest = {
            "gadm_bootstrap": {
                "helper": "db/bootstrap/gcp04-runtime/004_gadm_global_role.sql",
                "actor_contract": "LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT",
                "exact_comment": self.comment,
                "created_and_commented_by_controlled_actor": True,
                "zero_memberships_and_settings_after_restore": True,
                "executor_absent_after_commit": True,
                "failure_injection_rollback_clean": True,
                "reserved_executor_collision_rejected": True,
                "invalid_actor_and_setting_rejected": True,
            }
        }

    def test_matching_manifest_and_catalog_pass(self):
        verify_gadm_bootstrap_manifest(self.manifest, self.catalog)

    def test_rehashed_manifest_with_contradictory_comment_fails_contract(self):
        tampered = copy.deepcopy(self.manifest)
        tampered["gadm_bootstrap"]["exact_comment"] = "Global bootstrap role; no login or memberships."
        body_digest = hashlib.sha256(canonical_json(tampered)).hexdigest()
        tampered["manifest_body_sha256"] = body_digest
        recomputed_body = {key: value for key, value in tampered.items() if key != "manifest_body_sha256"}
        self.assertEqual(body_digest, hashlib.sha256(canonical_json(recomputed_body)).hexdigest())
        with self.assertRaisesRegex(ValueError, "manifest comment differs from the catalog and V60 contract"):
            verify_gadm_bootstrap_manifest(tampered, self.catalog)


if __name__ == "__main__":
    unittest.main()
