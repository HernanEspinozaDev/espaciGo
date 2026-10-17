"""Opt-in integration tests run only from the network-isolated PG18 harness."""

import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

import gcp03_origin_import as transport
from gcp03_origin_cloud_import_executor import (
    ImportStopped, ObjectRef, PsqlPgRestoreRunner, decrypt_object, validate_normalization_sql, validate_toc,
)
from gcp03_origin_artifact_review import _run_pg_restore_list, build_review_plan
from gcp03_origin_postrestore_verify import READ_ONLY_SQL
from gcp03_origin_postrestore_verify import table_names_from_toc

SOCKET = os.environ.get("GCP03_ORIGIN_EXECUTOR_TEST_SOCKET")


class EncryptedArchiveStore:
    def __init__(self, ref, payload):
        self.ref, self.payload = ref, payload

    def open(self, bucket, ref):
        if ref != self.ref:
            raise AssertionError("unexpected object reference")
        return type("Download", (), {"generation": ref.generation, "stream": io.BytesIO(self.payload)})()


@unittest.skipUnless(SOCKET, "run scripts/test-gcp03-origin-postgres.sh for isolated PostgreSQL integration")
class CloudImportPostgresIntegrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix="g03-executor-test-")
        cls.password_file = Path(cls.temp.name) / "postgres-password"
        cls.password_file.write_text("synthetic-only\n")
        cls.password_file.chmod(0o600)
        cls.runner = PsqlPgRestoreRunner(cls.password_file, host=SOCKET)

    @classmethod
    def tearDownClass(cls):
        cls.runner.close()
        cls.temp.cleanup()

    def psql(self, sql, database="espacigo_scratch", user="postgres"):
        return subprocess.run(["psql", "-X", "-qAt", "--set=ON_ERROR_STOP=1", "--host=" + SOCKET,
            "--port=5432", "--username=" + user, "--dbname=" + database, "-c", sql],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True, text=True).stdout.strip()

    def reset_target(self):
        # The harness database is disposable tmpfs, created without a persistent volume.
        self.psql("DROP DATABASE IF EXISTS espacigo_scratch WITH (FORCE)", "postgres")
        self.psql("CREATE DATABASE espacigo_scratch", "postgres")
        self.psql("""CREATE EXTENSION btree_gist;
CREATE EXTENSION fuzzystrmatch;
CREATE EXTENSION postgis;
CREATE EXTENSION postgis_tiger_geocoder;
CREATE EXTENSION postgis_topology;
CREATE EXTENSION unaccent;
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN CREATE ROLE espacigo_runtime; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_gadm_bootstrap') THEN CREATE ROLE espacigo_gadm_bootstrap; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_gcp03_observer') THEN CREATE ROLE espacigo_gcp03_observer; END IF;
END $$;
ALTER ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
ALTER ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
ALTER ROLE espacigo_gcp03_observer NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;""")

    def setUp(self):
        self.reset_target()

    def test_postgis_extension_schemas_pass_and_application_objects_fail_preflight(self):
        report = self.runner.preflight()
        self.assertEqual(report["database"], "espacigo_scratch")
        self.assertTrue(report["empty"])
        self.assertTrue(report["roles_ready"])
        self.assertTrue(report["extensions_ready"])
        self.assertTrue(report["application_schemas_empty"])
        self.assertTrue(report["postgres_version"])
        self.assertTrue(report["admin_role"])

        self.psql("CREATE SCHEMA application_probe")
        report = self.runner.preflight()
        self.assertFalse(report["application_schemas_empty"])
        self.psql("DROP SCHEMA application_probe")
        self.psql("CREATE TABLE public.preflight_probe(id integer)")
        report = self.runner.preflight()
        self.assertFalse(report["empty"])

    def make_archive(self):
        self.psql("DROP DATABASE IF EXISTS g03_executor_source_fixture WITH (FORCE)", "postgres")
        self.psql("CREATE DATABASE g03_executor_source_fixture", "postgres")
        self.psql("""CREATE TABLE public.sample(id integer PRIMARY KEY, payload text);
INSERT INTO public.sample SELECT i, md5(i::text) FROM generate_series(1,100000) AS i;
CREATE FUNCTION public.sample(integer) RETURNS integer LANGUAGE SQL IMMUTABLE AS $$ SELECT $1 + 1 $$;""", "g03_executor_source_fixture")
        path = Path(self.temp.name) / "source.custom"
        result = subprocess.run(["pg_dump", "--format=custom", "--compress=0", "--host=" + SOCKET,
            "--port=5432", "--username=postgres", "--dbname=g03_executor_source_fixture", "--file=" + str(path)],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True)
        self.assertGreater(path.stat().st_size, 3 * 1024 * 1024)
        return path

    def encrypted_feed(self, path):
        k_run, dek = bytes(reversed(range(32))), bytes(range(32))
        nonce = b"w" * 12
        wrapped = AESGCM(k_run).encrypt(nonce, dek, f"{transport.FORMAT}:g03o-1017:database.dump:dek".encode())
        transport_cipher = io.BytesIO()
        with path.open("rb") as plain:
            stats = transport.encrypt_chunks(plain, transport_cipher, dek, "g03o-1017", "database.dump")
        payload = transport_cipher.getvalue()
        ref = ObjectRef("runs/g03o-1017/database.dump.gcm", 1, hashlib.sha256(payload).hexdigest(), len(payload))
        item = {"transport_cipher_sha256": ref.sha256, "transport_cipher_bytes": ref.size_bytes,
            "plain_sha256": stats["plain_sha256"], "plain_bytes": stats["plain_bytes"],
            "chunk_count": stats["chunks"], "wrap_nonce_hex": nonce.hex(), "wrapped_dek_hex": wrapped.hex()}
        store = EncryptedArchiveStore(ref, payload)
        def callback(sink):
            result = decrypt_object(store, "synthetic-bucket", ref, item, k_run, "database.dump", sink)
            self.assertEqual(result["plain_bytes"], path.stat().st_size)
            return result
        return callback

    def test_large_custom_dump_lists_and_restores_through_broken_pipe_boundary(self):
        archive = self.make_archive()
        feed = self.encrypted_feed(archive)
        available = self.runner.list_toc(feed)
        self.assertTrue(self.runner.last_list_pipe_closed_early)
        self.assertTrue(any(" TABLE public sample " in line for line in available))
        self.assertTrue(any(" TABLE DATA public sample " in line for line in available))
        self.assertTrue(any(" FUNCTION public sample(integer) " in line for line in available))
        selected = [line for line in available if any(token in line for token in (
            " TABLE public sample ", " TABLE DATA public sample ", " FUNCTION public sample(integer) ",
            " CONSTRAINT public sample ", " INDEX public sample_pkey "))]
        allowlist = ("\n".join(selected) + "\n").encode()
        validate_toc("\n".join(available), allowlist)
        self.runner.restore(feed, allowlist)
        self.assertEqual(self.psql("SELECT count(*) FROM public.sample"), "100000")
        self.assertEqual(self.psql("SELECT public.sample(41)"), "42")

    def test_real_postgres_accepts_acl_privilege_words_and_function_signature(self):
        archive = self.make_archive()
        feed = self.encrypted_feed(archive)
        available = self.runner.list_toc(feed)
        selected = [line for line in available if any(token in line for token in (
            " TABLE public sample ", " TABLE DATA public sample ", " FUNCTION public sample(integer) ",
            " CONSTRAINT public sample ", " INDEX public sample_pkey "))]
        allowlist = ("\n".join(selected) + "\n").encode()
        self.runner.restore(feed, allowlist)
        normalization = (b"GRANT CREATE ON SCHEMA public TO espacigo_runtime;\n"
            b"ALTER TABLE public.sample OWNER TO espacigo_runtime;\n"
            b"ALTER FUNCTION public.sample(integer) OWNER TO espacigo_runtime;\n"
            b"GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.sample TO espacigo_runtime;\n"
            b"GRANT EXECUTE ON FUNCTION public.sample(integer) TO espacigo_runtime;\n"
            b"ALTER DEFAULT PRIVILEGES FOR ROLE postgres IN SCHEMA public GRANT SELECT ON TABLES TO espacigo_runtime;\n")
        validate_normalization_sql(normalization, hashlib.sha256(normalization).hexdigest())
        self.runner.normalize_owners_and_acls(normalization)
        result = self.psql("""SELECT c.relowner=(SELECT oid FROM pg_roles WHERE rolname='espacigo_runtime'),
 p.proowner=(SELECT oid FROM pg_roles WHERE rolname='espacigo_runtime'),
 has_table_privilege('espacigo_runtime','public.sample','SELECT,INSERT,UPDATE,DELETE')
FROM pg_class c CROSS JOIN pg_proc p
WHERE c.oid='public.sample'::regclass AND p.oid='public.sample(integer)'::regprocedure""")
        self.assertEqual(result, "t|t|t")
        self.assertEqual(self.psql("SELECT has_function_privilege('espacigo_runtime','public.sample(integer)','EXECUTE')"), "t")
        self.psql("CREATE TABLE public.future_acl_probe(id integer)")
        self.assertEqual(self.psql("SELECT has_table_privilege('espacigo_runtime','public.future_acl_probe','SELECT')"), "t")
        self.assertEqual(self.psql("SELECT has_table_privilege('espacigo_runtime','public.future_acl_probe','INSERT')"), "f")

        self.psql("GRANT CREATE ON SCHEMA public TO espacigo_gadm_bootstrap")
        with self.assertRaises(ImportStopped):
            self.runner.normalize_owners_and_acls(b"ALTER TABLE public.sample OWNER TO espacigo_gadm_bootstrap; SELECT 1/0;")
        owner_after_rollback = self.psql("SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='public.sample'::regclass")
        self.assertEqual(owner_after_rollback, "espacigo_runtime")

    def test_postrestore_owner_audit_ignores_extension_indexes_but_rejects_app_owner(self):
        self.psql("""CREATE ROLE g03_postrestore_login LOGIN;
GRANT espacigo_gcp03_observer TO g03_postrestore_login;
GRANT USAGE ON SCHEMA public TO espacigo_gcp03_observer;
CREATE TABLE public.schema_migrations(version bigint, name text, checksum text);
CREATE TABLE public.sesion(revocada_en timestamptz, expira_en timestamptz);
CREATE TABLE public.token_accion(consumido_en timestamptz, invalidado_en timestamptz, expira_en timestamptz);
CREATE TABLE public.aviso_local(estado text);
CREATE TABLE public.outbox_evento_local(lease_hasta timestamptz, entregada_en timestamptz, cancelada_en timestamptz, fallo_terminal_en timestamptz);
CREATE TABLE public.sample(id integer PRIMARY KEY);
ALTER TABLE public.schema_migrations OWNER TO espacigo_runtime;
ALTER TABLE public.sesion OWNER TO espacigo_runtime;
ALTER TABLE public.token_accion OWNER TO espacigo_runtime;
ALTER TABLE public.aviso_local OWNER TO espacigo_runtime;
ALTER TABLE public.outbox_evento_local OWNER TO espacigo_runtime;
ALTER TABLE public.sample OWNER TO espacigo_runtime;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO espacigo_gcp03_observer;""")
        extension_index_owner = self.psql(
            "SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='public.spatial_ref_sys_pkey'::regclass")
        report = json.loads(self.psql(READ_ONLY_SQL, user="g03_postrestore_login"))
        self.assertTrue(report["application_owners_valid"])
        self.assertTrue(report["observer_role_active"])
        self.assertIn("sample", report["public_tables"])
        self.assertEqual(report["runtime_role_login"], False)
        self.assertEqual(self.psql(
            "SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='public.spatial_ref_sys_pkey'::regclass"),
            extension_index_owner)

        self.psql("ALTER TABLE public.sample OWNER TO postgres")
        wrong_owner_report = json.loads(self.psql(READ_ONLY_SQL, user="g03_postrestore_login"))
        self.assertFalse(wrong_owner_report["application_owners_valid"])
        self.assertEqual(self.psql(
            "SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid='public.spatial_ref_sys_pkey'::regclass"),
            extension_index_owner)

    def test_real_pg_restore_list_builds_private_safe_allowlist_without_dump_file(self):
        source_db = "g03_artifact_review_source_fixture"
        self.psql("DROP DATABASE IF EXISTS " + source_db + " WITH (FORCE)", "postgres")
        self.psql("CREATE DATABASE " + source_db, "postgres")
        try:
            self.psql("""CREATE EXTENSION postgis;
CREATE TABLE public.schema_migrations(version bigint, name text, checksum text);
CREATE TABLE public.sesion(revocada_en timestamptz, expira_en timestamptz);
CREATE TABLE public.token_accion(consumido_en timestamptz, invalidado_en timestamptz, expira_en timestamptz);
CREATE TABLE public.aviso_local(estado text);
CREATE TABLE public.outbox_evento_local(lease_hasta timestamptz, entregada_en timestamptz, cancelada_en timestamptz, fallo_terminal_en timestamptz);
CREATE TABLE public.sample(id integer PRIMARY KEY);
CREATE FUNCTION public.sample(integer) RETURNS integer LANGUAGE SQL IMMUTABLE AS $$ SELECT $1 + 1 $$;
CREATE TEXT SEARCH DICTIONARY public.synthdict (TEMPLATE = pg_catalog.simple);""", source_db)
            dump = subprocess.run(["pg_dump", "--format=custom", "--compress=0", "--host=" + SOCKET,
                "--port=5432", "--username=postgres", "--dbname=" + source_db],
                stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=True)
            archive = bytearray(dump.stdout)
            toc_text = _run_pg_restore_list(archive, "pg_restore")
            relations = json.loads(self.psql("""SELECT COALESCE(json_agg(json_build_object(
 'schema',n.nspname,'name',c.relname,'kind',c.relkind::text,'owner',r.rolname,'acl',c.relacl::text)), '[]'::json)::text
FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_roles r ON r.oid=c.relowner
WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f')""", source_db))
            routines = json.loads(self.psql("""SELECT COALESCE(json_agg(json_build_object(
 'schema',n.nspname,'name',p.proname,'identity_args',pg_get_function_identity_arguments(p.oid),
 'owner',r.rolname,'acl',p.proacl::text)), '[]'::json)::text
FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_roles r ON r.oid=p.proowner
WHERE n.nspname='public'""", source_db))
            allowlist, normalization, review_map = build_review_plan(toc_text,
                {"database_report": {"relation_owners_acls": relations, "routine_owners_acls": routines}}, "f" * 64)
            names = table_names_from_toc(allowlist)
            self.assertEqual(names, {"schema_migrations", "sesion", "token_accion", "aviso_local",
                "outbox_evento_local", "sample"})
            self.assertNotIn("TABLE DATA public spatial_ref_sys", allowlist.decode())
            self.assertIn(b"GRANT SELECT (version, name, checksum)", normalization)
            self.assertIn(b"ALTER TEXT SEARCH DICTIONARY public.synthdict OWNER TO espacigo_runtime", normalization)
            self.assertIn(b"GRANT CREATE ON SCHEMA public TO espacigo_runtime", normalization)
            self.assertLess(normalization.index(b"GRANT CREATE ON SCHEMA public"),
                normalization.index(b"ALTER TABLE public.sample OWNER TO espacigo_runtime"))
            self.assertGreater(normalization.index(b"REVOKE CREATE ON SCHEMA public"),
                normalization.index(b"ALTER TABLE public.sample OWNER TO espacigo_runtime"))
            self.assertGreater(review_map["selected_toc_entry_count"], len(names))

            # Exercise the complete generated plan as a PostgreSQL 18
            # non-superuser with the bootstrap-provisioned runtime membership.
            self.psql("DROP ROLE IF EXISTS g03_origin_import_admin")
            self.psql("CREATE ROLE g03_origin_import_admin LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT")
            self.psql("ALTER DATABASE espacigo_scratch OWNER TO g03_origin_import_admin")
            admin = PsqlPgRestoreRunner(self.password_file, host=SOCKET,
                database_user="g03_origin_import_admin")
            try:
                missing = admin.preflight()
                self.assertFalse(missing["admin_superuser"])
                self.assertFalse(missing["normalization_prerequisites_available"])
                self.psql("GRANT espacigo_runtime TO g03_origin_import_admin WITH ADMIN FALSE, INHERIT FALSE, SET TRUE")
                ready = admin.preflight()
                self.assertTrue(ready["normalization_prerequisites_available"])

                def feed(sink):
                    sink.write(archive)

                available = admin.list_toc(feed)
                selected_allowlist = validate_toc("\n".join(available), allowlist)
                self.assertEqual(len(selected_allowlist), review_map["selected_toc_entry_count"])
                admin.restore(feed, allowlist)
                admin.normalize_owners_and_acls(normalization)
                final = self.psql("""SELECT
 (SELECT pg_get_userbyid(relowner)='espacigo_runtime' FROM pg_class WHERE oid='public.sample'::regclass),
 (SELECT pg_get_userbyid(proowner)='espacigo_runtime' FROM pg_proc WHERE oid='public.sample(integer)'::regprocedure),
 has_schema_privilege('espacigo_runtime','public','CREATE'),
 pg_has_role('g03_origin_import_admin','espacigo_runtime','MEMBER'),
 (SELECT rolcanlogin OR rolsuper OR rolinherit FROM pg_roles WHERE rolname='espacigo_runtime'),
 has_column_privilege('espacigo_gcp03_observer','public.schema_migrations','version','SELECT'),
 has_column_privilege('espacigo_gcp03_observer','public.schema_migrations','version','UPDATE'),
 has_table_privilege('espacigo_gcp03_observer','public.sample','SELECT'),
 has_table_privilege('espacigo_gcp03_observer','public.sample','INSERT')""")
                self.assertEqual(final, "t|t|f|t|f|t|f|f|f")
            finally:
                admin.close()
                self.psql("ALTER DATABASE espacigo_scratch OWNER TO postgres")
                self.psql("DROP ROLE IF EXISTS g03_origin_import_admin")
                archive[:] = b"\x00" * len(archive)
        finally:
            self.psql("DROP DATABASE IF EXISTS " + source_db + " WITH (FORCE)", "postgres")

    def test_invalid_restore_is_uncertain_and_creates_no_application_object(self):
        self.assertTrue(self.runner.preflight()["empty"])
        payload = b"not a PostgreSQL custom archive"
        def feed(sink):
            sink.write(payload)
        with self.assertRaises(ImportStopped) as caught:
            self.runner.restore(feed, b"1; 0 0 TABLE public failed_restore postgres\n")
        self.assertTrue(caught.exception.mutation_started)
        self.assertEqual(self.psql("SELECT count(*) FROM pg_class WHERE oid=to_regclass('public.failed_restore')"), "0")

    def test_quarantine_updates_synthetic_rows_and_disables_runtime_login(self):
        self.psql("""CREATE TABLE public.sesion(revocada_en timestamptz, creada_en timestamptz, expira_en timestamptz);
CREATE TABLE public.token_accion(invalidado_en timestamptz, consumido_en timestamptz, creado_en timestamptz, expira_en timestamptz);
CREATE TABLE public.aviso_local(id integer, ciclo integer, estado text, cancelada_en timestamptz, creada_en timestamptz, retirar_en timestamptz, lease_hasta timestamptz, codigo_error text);
CREATE TABLE public.aviso_local_ciclo(aviso_id integer, ciclo integer, estado text, finalizada_en timestamptz, iniciada_en timestamptz, codigo_resultado text);
CREATE TABLE public.outbox_evento_local(lease_hasta timestamptz, entregada_en timestamptz, cancelada_en timestamptz, fallo_terminal_en timestamptz);
INSERT INTO public.sesion VALUES (NULL, now()-interval '1 hour', now()+interval '1 hour');
INSERT INTO public.token_accion VALUES (NULL, NULL, now()-interval '1 hour', now()+interval '1 hour');
INSERT INTO public.aviso_local VALUES (1,1,'procesando',NULL,now()-interval '1 hour',NULL,now()+interval '1 hour',NULL);
INSERT INTO public.aviso_local_ciclo VALUES (1,1,'pendiente',NULL,now()-interval '1 hour',NULL);
INSERT INTO public.outbox_evento_local VALUES (now()+interval '1 hour',NULL,NULL,NULL);
ALTER ROLE espacigo_runtime LOGIN;""")
        report = self.runner.sanitize()
        self.assertEqual(report["valid_sessions"], 0)
        self.assertEqual(report["usable_action_tokens"], 0)
        self.assertEqual(report["pending_notices"], 0)
        self.assertEqual(report["identity_event_leases"], 0)
        self.assertFalse(report["runtime_role_login"])


if __name__ == "__main__":
    unittest.main()
