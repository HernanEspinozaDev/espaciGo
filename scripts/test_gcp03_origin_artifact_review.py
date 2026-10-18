"""Synthetic tests for private TOC and owner/ACL plan preparation."""
import hashlib
import io
import os
from pathlib import Path
import stat
import tempfile
import unittest

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

import gcp03_origin_import as transport
from gcp03_origin_artifact_review import (
    ArtifactReviewError, _private_artifact, build_review_plan,
)
from gcp03_origin_postrestore_verify import table_names_from_toc


def metadata_fixture():
    relation_names = {
        "schema_migrations": ("version", "name", "checksum"),
        "sesion": ("revocada_en", "expira_en"),
        "token_accion": ("consumido_en", "invalidado_en", "expira_en"),
        "aviso_local": ("estado",),
        "outbox_evento_local": ("lease_hasta", "entregada_en", "cancelada_en", "fallo_terminal_en"),
        "sample": ("id",),
    }
    return {
        "database_report": {
            "relation_owners_acls": [{"schema": "public", "name": name, "kind": "r",
                "owner": "source_owner", "acl": "{source_owner=arwdDxt/source_owner}"}
                for name in relation_names],
            "routine_owners_acls": [{"schema": "public", "name": "sample",
                "identity_args": "integer", "owner": "source_owner", "acl": None}],
        }
    }


class OriginArtifactReviewTests(unittest.TestCase):
    def test_plan_excludes_extension_data_and_grants_observer_only_columns(self):
        toc = """; Archive created at 2026-10-17 UTC
1; 0 0 TABLE public schema_migrations source_owner
2; 0 0 TABLE DATA public schema_migrations source_owner
3; 0 0 TABLE public sesion source_owner
4; 0 0 TABLE public token_accion source_owner
5; 0 0 TABLE public aviso_local source_owner
6; 0 0 TABLE public outbox_evento_local source_owner
7; 0 0 TABLE public sample source_owner
8; 0 0 TABLE DATA public sample source_owner
9; 0 0 INDEX public sample_pkey source_owner
10; 0 0 FUNCTION public sample(integer) source_owner
11; 0 0 TABLE DATA public spatial_ref_sys source_owner
12; 0 0 EXTENSION - postgis
13; 0 0 ACL public TABLE sample source_owner
14; 0 0 TEXT SEARCH DICTIONARY public synthdict source_owner
"""
        toc_bytes, normalization, owner_map = build_review_plan(
            toc, metadata_fixture(), "a" * 64)
        text = toc_bytes.decode()
        self.assertIn("TABLE DATA public sample", text)
        self.assertNotIn("TABLE DATA public spatial_ref_sys", text)
        self.assertNotIn("EXTENSION - postgis", text)
        self.assertEqual(table_names_from_toc(toc_bytes), {
            "schema_migrations", "sesion", "token_accion", "aviso_local",
            "outbox_evento_local", "sample"})
        sql = normalization.decode()
        self.assertIn("ALTER TABLE public.sample OWNER TO espacigo_runtime;", sql)
        self.assertIn("ALTER FUNCTION public.sample(integer) OWNER TO espacigo_runtime;", sql)
        self.assertIn("ALTER TEXT SEARCH DICTIONARY public.synthdict OWNER TO espacigo_runtime;", sql)
        self.assertIn("GRANT SELECT (version, name, checksum) ON TABLE public.schema_migrations TO espacigo_gcp03_observer;", sql)
        self.assertLess(sql.index("GRANT SELECT (version, name, checksum)"),
                        sql.index("ALTER TABLE public.sample OWNER TO espacigo_runtime"))
        self.assertLess(sql.index("GRANT CREATE ON SCHEMA public"),
                        sql.index("ALTER TABLE public.sample OWNER TO espacigo_runtime"))
        self.assertGreater(sql.index("REVOKE CREATE ON SCHEMA public"),
                           sql.index("ALTER TABLE public.sample OWNER TO espacigo_runtime"))
        self.assertEqual(owner_map["observer_column_grant_count"], 5)
        self.assertEqual(owner_map["public_table_count"], 6)
        serialized_map = str(owner_map)
        self.assertNotIn("arwdDxt", serialized_map)
        self.assertTrue(all(item["source_acl_replaced"] for item in owner_map["owner_actions"]))

    def test_owned_sequences_follow_parent_transfer_but_standalone_sequences_remain_explicit(self):
        toc = """1; 0 0 TABLE public schema_migrations source_owner
2; 0 0 TABLE public sesion source_owner
3; 0 0 TABLE public token_accion source_owner
4; 0 0 TABLE public aviso_local source_owner
5; 0 0 TABLE public outbox_evento_local source_owner
6; 0 0 TABLE public sample source_owner
7; 0 0 SEQUENCE public sample_serial_id_seq source_owner
8; 0 0 SEQUENCE OWNED BY public sample_serial_id_seq source_owner
9; 0 0 SEQUENCE public sample_identity_id_seq source_owner
11; 0 0 SEQUENCE public standalone_probe_seq source_owner
"""
        metadata = metadata_fixture()
        metadata["database_report"]["relation_owners_acls"].extend(
            {"schema": "public", "name": name, "kind": "S", "owner": "source_owner", "acl": None}
            for name in ("sample_serial_id_seq", "sample_identity_id_seq", "standalone_probe_seq"))

        _, normalization, owner_map = build_review_plan(
            toc, metadata, "a" * 64, identity_sequences={("public", "sample_identity_id_seq")})
        sql = normalization.decode()
        self.assertNotIn("ALTER SEQUENCE public.sample_serial_id_seq OWNER TO espacigo_runtime;", sql)
        self.assertNotIn("ALTER SEQUENCE public.sample_identity_id_seq OWNER TO espacigo_runtime;", sql)
        self.assertIn("ALTER SEQUENCE public.standalone_probe_seq OWNER TO espacigo_runtime;", sql)
        by_name = {item["name"]: item for item in owner_map["owner_actions"] if item["type"] == "SEQUENCE"}
        self.assertEqual(by_name["sample_serial_id_seq"]["owner_action"],
                         "owner follows parent table ownership transfer")
        self.assertEqual(by_name["sample_identity_id_seq"]["owner_action"],
                         "owner follows parent table ownership transfer")
        self.assertNotIn("owner_action", by_name["standalone_probe_seq"])
        self.assertEqual(owner_map["parent_owned_sequence_count"], 2)
        self.assertEqual(owner_map["standalone_sequence_count"], 1)

    def test_schema_parser_identifies_quoted_identity_sequence_name(self):
        from gcp03_origin_artifact_review import identity_sequences_from_schema

        schema_sql = '''ALTER TABLE ONLY public.sample ALTER COLUMN identity_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public."sample_identity_id_seq"
    START WITH 1
    INCREMENT BY 1
);'''
        self.assertEqual(identity_sequences_from_schema(schema_sql),
                         {("public", "sample_identity_id_seq")})

    def test_schema_parser_handles_quoted_identity_table_and_column(self):
        from gcp03_origin_artifact_review import identity_sequences_from_schema

        schema_sql = '''ALTER TABLE ONLY "custom-schema"."sample table"
ALTER COLUMN "identity id" ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME "custom-schema"."sample sequence"
);'''
        self.assertEqual(identity_sequences_from_schema(schema_sql),
                         {("custom-schema", "sample sequence")})

    def test_orphan_or_duplicate_sequence_ownership_dependency_fails_closed(self):
        toc = "1; 0 0 SEQUENCE OWNED BY public sample_id_seq source_owner\n"
        with self.assertRaisesRegex(ArtifactReviewError, "no sequence entry"):
            build_review_plan(toc, metadata_fixture(), "a" * 64)

    def test_transport_decrypt_is_restricted_to_dump_and_keeps_plaintext_in_memory(self):
        with tempfile.TemporaryDirectory() as temp:
            stage = Path(temp) / "stage"
            stage.mkdir(mode=0o700)
            os.chmod(stage, 0o700)
            key = bytes(range(32))
            dek = bytes(reversed(range(32)))
            salt = b"s" * 32
            kek = transport.transport_key(key, salt, "g03o-1017")
            nonce = b"n" * 12
            wrapped = AESGCM(kek).encrypt(nonce, dek,
                f"{transport.FORMAT}:g03o-1017:database.dump:dek".encode())
            plain = b"synthetic custom archive bytes only"
            encrypted = io.BytesIO()
            stats = transport.encrypt_chunks(io.BytesIO(plain), encrypted, dek, "g03o-1017", "database.dump")
            artifact_path = stage / "database.dump.gcm"
            artifact_path.write_bytes(encrypted.getvalue())
            artifact_path.chmod(0o600)
            manifest = {"salt_hex": salt.hex(), "artifacts": {"database.dump": {
                "source_name": "database.dump.gpg", "transport_cipher_sha256": hashlib.sha256(encrypted.getvalue()).hexdigest(),
                "transport_cipher_bytes": len(encrypted.getvalue()), "wrap_nonce_hex": nonce.hex(),
                "wrapped_dek_hex": wrapped.hex(), "plain_sha256": stats["plain_sha256"],
                "plain_bytes": stats["plain_bytes"], "chunk_count": stats["chunks"]}}}
            clear = _private_artifact(stage, manifest, key, "database.dump")
            try:
                self.assertEqual(clear.read(), plain)
            finally:
                clear.close()
            with self.assertRaises(ArtifactReviewError):
                _private_artifact(stage, manifest, key, "private-files.tar")


if __name__ == "__main__":
    unittest.main()
