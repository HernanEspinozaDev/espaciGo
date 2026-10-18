"""Opt-in PostgreSQL 18 integration for recovery owner planning.

Run only against a dedicated, network-isolated fixture. This test creates
synthetic catalog objects and never restores or reads an application dump.
"""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

from gcp03_origin_private_files import PrivateFile
from gcp03_origin_recovery_plan import build_normalization_recovery_plan, execute_reviewed_recovery, statement_id
from gcp03_origin_recovery_preflight import CatalogReader
from gcp03_origin_recovery_preflight import EXPECTED_CURRENT
from gcp03_origin_recovery_runner import LocalPsqlRecoveryDatabase

SOCKET = os.environ.get("G03O_RECOVERY_TEST_SOCKET")
IMPORTER = "g03_recovery_importer"
RUNTIME = "espacigo_runtime"
TABLES = (
    "recovery_parent", "recovery_notice", "schema_migrations", "sesion",
    "token_accion", "aviso_local", "aviso_local_ciclo", "outbox_evento_local",
    "foto_perfil_sintetica_local", "espacio_galeria_sintetica_local",
    "verificacion_evidencia_sintetica", "espacio_galeria_archivo_candidato_local",
    "operacion_arriendo_archivo_candidato_local", "verificacion_archivo_retiro_pendiente_local",
)
SERIAL_TABLES = (
    "recovery_parent", "recovery_notice", "sesion", "token_accion", "aviso_local",
    "outbox_evento_local", "foto_perfil_sintetica_local",
    "espacio_galeria_sintetica_local", "espacio_galeria_archivo_candidato_local",
    "operacion_arriendo_archivo_candidato_local",
)
NORMALIZATION = ("GRANT CREATE ON SCHEMA public TO espacigo_runtime;\n" +
    "GRANT SELECT (state) ON TABLE public.recovery_notice TO espacigo_gcp03_observer;\n" +
    "\n".join(f"ALTER TABLE public.{name} OWNER TO espacigo_runtime;" for name in TABLES) + "\n" +
    "\n".join(f"ALTER SEQUENCE public.{name}_id_seq OWNER TO espacigo_runtime;" for name in SERIAL_TABLES) + "\n" +
    "ALTER SEQUENCE public.recovery_standalone_seq OWNER TO espacigo_runtime;\n" +
    "ALTER FUNCTION public.recovery_probe(p_at timestamp with time zone, p_count integer) OWNER TO espacigo_runtime;\n" +
    "ALTER TEXT SEARCH DICTIONARY public.recovery_dict OWNER TO espacigo_runtime;\n" +
    "ALTER TEXT SEARCH CONFIGURATION public.recovery_config OWNER TO espacigo_runtime;\n" +
    "REVOKE CREATE ON SCHEMA public FROM espacigo_runtime;\n").encode()


@unittest.skipUnless(SOCKET, "set G03O_RECOVERY_TEST_SOCKET to an isolated PostgreSQL 18 socket")
class RecoveryPlanPostgres18Tests(unittest.TestCase):
    def psql(self, sql: str, user="fixture_control", database="espacigo_scratch") -> str:
        result = subprocess.run(
            ["psql", "-X", "-qAt", "--set=ON_ERROR_STOP=1", "--host=" + SOCKET,
             "--port=5432", "--username=" + user, "--dbname=" + database, "-c", sql],
            check=True, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        return result.stdout.strip()

    def test_postgres18_full_recovery_with_nosuperuser_and_no_temp(self):
        major = self.psql("SHOW server_version_num")
        self.assertTrue(major.startswith("180"), major)
        self.psql("""DROP TABLE IF EXISTS public.verificacion_archivo_retiro_pendiente_local CASCADE;
DROP TABLE IF EXISTS public.operacion_arriendo_archivo_candidato_local CASCADE;
DROP TABLE IF EXISTS public.espacio_galeria_archivo_candidato_local CASCADE;
DROP TABLE IF EXISTS public.verificacion_evidencia_sintetica CASCADE;
DROP TABLE IF EXISTS public.espacio_galeria_sintetica_local CASCADE;
DROP TABLE IF EXISTS public.foto_perfil_sintetica_local CASCADE;
DROP TABLE IF EXISTS public.outbox_evento_local CASCADE;
DROP TABLE IF EXISTS public.aviso_local_ciclo CASCADE;
DROP TABLE IF EXISTS public.aviso_local CASCADE;
DROP TABLE IF EXISTS public.token_accion CASCADE;
DROP TABLE IF EXISTS public.sesion CASCADE;
DROP TABLE IF EXISTS public.recovery_parent CASCADE;
DROP TABLE IF EXISTS public.recovery_notice CASCADE;
DROP TABLE IF EXISTS public.schema_migrations CASCADE;
DROP SEQUENCE IF EXISTS public.recovery_standalone_seq CASCADE;
DROP FUNCTION IF EXISTS public.recovery_probe(timestamp with time zone,integer);
DROP FUNCTION IF EXISTS public.recovery_probe(integer);
DROP TEXT SEARCH CONFIGURATION IF EXISTS public.recovery_config CASCADE;
DROP TEXT SEARCH DICTIONARY IF EXISTS public.recovery_dict;
REVOKE CREATE ON SCHEMA public FROM espacigo_runtime;
ALTER SCHEMA public OWNER TO fixture_control;
REVOKE CREATE ON SCHEMA public FROM postgres;
ALTER DATABASE espacigo_scratch OWNER TO fixture_control;
DROP ROLE IF EXISTS postgres;
DROP ROLE IF EXISTS espacigo_gcp03_observer;
DROP ROLE IF EXISTS espacigo_runtime;
CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_gcp03_observer NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE postgres LOGIN NOSUPERUSER CREATEDB CREATEROLE NOINHERIT;
GRANT espacigo_runtime TO postgres WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
ALTER DATABASE espacigo_scratch OWNER TO postgres;
ALTER SCHEMA public OWNER TO postgres;
REVOKE TEMP ON DATABASE espacigo_scratch FROM PUBLIC;
SET ROLE postgres;
CREATE TABLE public.recovery_parent(id serial PRIMARY KEY, state text);
CREATE TABLE public.recovery_notice(id serial PRIMARY KEY, state text);
CREATE TABLE public.schema_migrations(version text, checksum text);
CREATE TABLE public.sesion(id serial PRIMARY KEY, revocada_en timestamptz, creada_en timestamptz, expira_en timestamptz);
CREATE TABLE public.token_accion(id serial PRIMARY KEY, invalidado_en timestamptz, consumido_en timestamptz, creado_en timestamptz, expira_en timestamptz);
CREATE TABLE public.aviso_local(id serial PRIMARY KEY, ciclo integer, estado text, creada_en timestamptz, cancelada_en timestamptz, retirar_en timestamptz, lease_hasta timestamptz, codigo_error text);
CREATE TABLE public.aviso_local_ciclo(aviso_id integer, ciclo integer, estado text, iniciada_en timestamptz, finalizada_en timestamptz, codigo_resultado text);
CREATE TABLE public.outbox_evento_local(id serial PRIMARY KEY, lease_hasta timestamptz, entregada_en timestamptz, cancelada_en timestamptz, fallo_terminal_en timestamptz);
CREATE TABLE public.foto_perfil_sintetica_local(id serial PRIMARY KEY, archivo_id uuid);
CREATE TABLE public.espacio_galeria_sintetica_local(id serial PRIMARY KEY, archivo_id uuid);
CREATE TABLE public.verificacion_evidencia_sintetica(id uuid PRIMARY KEY);
CREATE TABLE public.espacio_galeria_archivo_candidato_local(id serial PRIMARY KEY, archivo_id uuid);
CREATE TABLE public.operacion_arriendo_archivo_candidato_local(id serial PRIMARY KEY, archivo_id uuid);
CREATE TABLE public.verificacion_archivo_retiro_pendiente_local(id integer PRIMARY KEY, archivo_id uuid);
CREATE SEQUENCE public.recovery_standalone_seq;
CREATE FUNCTION public.recovery_probe(p_at timestamp with time zone, p_count integer) RETURNS integer LANGUAGE sql IMMUTABLE AS 'SELECT p_count';
CREATE TEXT SEARCH DICTIONARY public.recovery_dict (TEMPLATE = pg_catalog.simple);
CREATE TEXT SEARCH CONFIGURATION public.recovery_config (PARSER = pg_catalog.default);
ALTER TEXT SEARCH CONFIGURATION public.recovery_config ADD MAPPING FOR asciiword WITH pg_catalog.simple;
INSERT INTO public.sesion(revocada_en, creada_en, expira_en) VALUES (NULL, now() - interval '1 hour', now() + interval '1 hour');
INSERT INTO public.token_accion(invalidado_en, consumido_en, creado_en, expira_en) VALUES (NULL, NULL, now() - interval '1 hour', now() + interval '1 hour');
INSERT INTO public.aviso_local(ciclo, estado, creada_en) VALUES (1, 'pendiente', now() - interval '1 hour');
INSERT INTO public.aviso_local_ciclo(aviso_id, ciclo, estado, iniciada_en) VALUES (1, 1, 'pendiente', now() - interval '1 hour');
INSERT INTO public.outbox_evento_local(lease_hasta) VALUES (now() + interval '1 hour');
INSERT INTO public.foto_perfil_sintetica_local(archivo_id) VALUES ('0f8fad5b-d9cb-469f-a165-70867728950e');
RESET ROLE;
""", user="fixture_control")
        reader = CatalogReader(host=SOCKET, port=5432, user="postgres", database="espacigo_scratch")
        current = reader.identity_and_inventory()
        self.assertEqual(current["database"], "espacigo_scratch")
        self.assertEqual(current["login"], "postgres")
        self.assertEqual(current["server_version_major"], 18)
        self.assertFalse(current["runtime_public_create"])
        self.assertTrue(current["runtime_membership_set"])
        self.assertFalse(current["runtime_membership_inherit"])
        self.assertNotEqual(current, EXPECTED_CURRENT)
        facts, sequence_parents = reader.statement_facts(NORMALIZATION)
        self.assertTrue(set(facts.values()) <= {"postgres", "satisfied", "missing"})
        plan = build_normalization_recovery_plan(
            NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts,
            owned_sequence_parent_statement=sequence_parents,
            binding={"synthetic_fixture_catalog_sha256": hashlib.sha256(
                json.dumps(current, sort_keys=True).encode()).hexdigest()},
        )
        self.assertNotIn(b"recovery_parent_id_seq OWNER", plan.pending_sql)
        self.assertIn(b"recovery_standalone_seq OWNER", plan.pending_sql)
        output_dir = Path(os.environ.get("G03O_RECOVERY_TEST_OUTPUT", ".local/testing/g03o-1017-recovery-20261018"))
        output_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
        fd, passfile_name = tempfile.mkstemp(prefix="recovery-pgpass-", dir=output_dir)
        passfile = Path(passfile_name)
        os.fchmod(fd, 0o600)
        os.close(fd)
        self.addCleanup(lambda: passfile.unlink(missing_ok=True))
        database = LocalPsqlRecoveryDatabase(socket=SOCKET, passfile=passfile)

        class ObservedDatabase:
            def normalize_owners_and_acls(inner, sql):
                database.normalize_owners_and_acls(sql)
                with self.assertRaises(subprocess.CalledProcessError):
                    self.psql("SELECT count(*) FROM public.recovery_parent", user="postgres")
                self.assertEqual(self.psql("SELECT has_database_privilege('espacigo_runtime','espacigo_scratch','TEMP')", user="fixture_control"), "f")

            def verify_normalized(inner): return database.verify_normalized()
            def sanitize(inner): return database.sanitize()
            def private_file_refs(inner):
                return database.private_file_refs()

        class MemoryCreateOnlyStore:
            def __init__(inner): inner.created = []
            def create(inner, object_name, data, sha256):
                if any(name == object_name for name, _, _ in inner.created):
                    raise AssertionError("duplicate create-only object")
                inner.created.append((object_name, data, sha256))

        file_id = "0f8fad5b-d9cb-469f-a165-70867728950e"
        file_data = b"synthetic private evidence"
        private_file = PrivateFile(f"runs/g03o-1017/{file_id}.blob", file_data,
                                  hashlib.sha256(file_data).hexdigest())
        store = MemoryCreateOnlyStore()
        report = execute_reviewed_recovery(plan=plan,
            approved_plan_sha256=plan.summary["plan_sha256"],
            database=ObservedDatabase(), private_files=[private_file],
            private_file_store=store, run_id="g03o-1017")
        self.assertTrue(report["phase_success"], report)
        self.assertTrue(report["report_complete"], report)
        self.assertEqual(report["database_reference_count"], 1)
        self.assertEqual(report["private_file_count"], 1)
        self.assertEqual(len(store.created), 1)
        owners = self.psql("""SELECT string_agg(name || '=' || owner, ',' ORDER BY name) FROM (
SELECT c.relname name, pg_get_userbyid(c.relowner) owner FROM pg_class c WHERE c.relname IN ('recovery_parent','recovery_parent_id_seq','recovery_standalone_seq')
UNION ALL SELECT p.proname, pg_get_userbyid(p.proowner) FROM pg_proc p WHERE p.proname='recovery_probe'
UNION ALL SELECT d.dictname, pg_get_userbyid(d.dictowner) FROM pg_ts_dict d WHERE d.dictname='recovery_dict'
UNION ALL SELECT c.cfgname, pg_get_userbyid(c.cfgowner) FROM pg_ts_config c WHERE c.cfgname='recovery_config'
) x""")
        self.assertEqual(owners, "recovery_config=espacigo_runtime,recovery_dict=espacigo_runtime,recovery_parent=espacigo_runtime,recovery_parent_id_seq=espacigo_runtime,recovery_probe=espacigo_runtime,recovery_standalone_seq=espacigo_runtime")
        options = self.psql("""SELECT set_option || '/' || inherit_option || '/' || admin_option
FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid JOIN pg_roles g ON g.oid=m.member
WHERE r.rolname='espacigo_runtime' AND g.rolname='postgres'""")
        self.assertEqual(options, "true/false/false")
        self.assertEqual(self.psql("SELECT has_schema_privilege('espacigo_runtime','public','CREATE')"), "f")


if __name__ == "__main__":
    unittest.main()
