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
import unittest

from gcp03_origin_recovery_plan import build_normalization_recovery_plan, statement_id
from gcp03_origin_recovery_preflight import CatalogReader
from gcp03_origin_recovery_plan import RecoveryPlanError

SOCKET = os.environ.get("G03O_RECOVERY_TEST_SOCKET")
IMPORTER = "g03_recovery_importer"
RUNTIME = "espacigo_runtime"
NORMALIZATION = f"""GRANT CREATE ON SCHEMA public TO {RUNTIME};
ALTER TABLE public.recovery_parent OWNER TO {RUNTIME};
ALTER SEQUENCE public.recovery_standalone_seq OWNER TO {RUNTIME};
ALTER SEQUENCE public.recovery_parent_id_seq OWNER TO {RUNTIME};
ALTER FUNCTION public.recovery_probe(p_at timestamp with time zone, p_count integer) OWNER TO {RUNTIME};
ALTER TEXT SEARCH DICTIONARY public.recovery_dict OWNER TO {RUNTIME};
ALTER TEXT SEARCH CONFIGURATION public.recovery_config OWNER TO {RUNTIME};
GRANT SELECT (state) ON TABLE public.recovery_notice TO espacigo_gcp03_observer;
REVOKE CREATE ON SCHEMA public FROM {RUNTIME};
""".encode()


@unittest.skipUnless(SOCKET, "set G03O_RECOVERY_TEST_SOCKET to an isolated PostgreSQL 18 socket")
class RecoveryPlanPostgres18Tests(unittest.TestCase):
    def psql(self, sql: str, user="fixture_control", database="espacigo_scratch") -> str:
        result = subprocess.run(
            ["psql", "-X", "-qAt", "--set=ON_ERROR_STOP=1", "--host=" + SOCKET,
             "--port=5432", "--username=" + user, "--dbname=" + database, "-c", sql],
            check=True, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        return result.stdout.strip()

    def test_postgres18_skips_cascaded_sequence_and_transfers_standalone(self):
        major = self.psql("SHOW server_version_num")
        self.assertTrue(major.startswith("180"), major)
        self.psql("""DROP TABLE IF EXISTS public.recovery_parent CASCADE;
DROP TABLE IF EXISTS public.recovery_notice CASCADE;
DROP TABLE IF EXISTS public.schema_migrations CASCADE;
DROP SEQUENCE IF EXISTS public.recovery_standalone_seq CASCADE;
DROP FUNCTION IF EXISTS public.recovery_probe(timestamp with time zone,integer);
DROP FUNCTION IF EXISTS public.recovery_probe(integer);
DROP TEXT SEARCH CONFIGURATION IF EXISTS public.recovery_config CASCADE;
DROP TEXT SEARCH DICTIONARY IF EXISTS public.recovery_dict;
REVOKE CREATE ON SCHEMA public FROM espacigo_runtime;
ALTER DATABASE espacigo_scratch OWNER TO fixture_control;
DROP ROLE IF EXISTS postgres;
DROP ROLE IF EXISTS espacigo_gcp03_observer;
DROP ROLE IF EXISTS espacigo_runtime;
CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE espacigo_gcp03_observer NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
CREATE ROLE postgres LOGIN NOSUPERUSER CREATEDB CREATEROLE NOINHERIT;
GRANT espacigo_runtime TO postgres WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
ALTER DATABASE espacigo_scratch OWNER TO postgres;
SET ROLE postgres;
CREATE TABLE public.recovery_parent(id serial PRIMARY KEY, state text);
CREATE TABLE public.recovery_notice(state text);
CREATE TABLE public.schema_migrations(version text, checksum text);
CREATE SEQUENCE public.recovery_standalone_seq;
CREATE FUNCTION public.recovery_probe(p_at timestamp with time zone, p_count integer) RETURNS integer LANGUAGE sql IMMUTABLE AS 'SELECT p_count';
CREATE TEXT SEARCH DICTIONARY public.recovery_dict (TEMPLATE = pg_catalog.simple);
CREATE TEXT SEARCH CONFIGURATION public.recovery_config (PARSER = pg_catalog.default);
ALTER TEXT SEARCH CONFIGURATION public.recovery_config ADD MAPPING FOR asciiword WITH pg_catalog.simple;
RESET ROLE;
""", user="fixture_control")
        statements = [item.strip() for item in NORMALIZATION.decode().split(";") if item.strip()]
        ids = [statement_id(item) for item in statements]
        reader = CatalogReader(host=SOCKET, port=5432, user="postgres", database="espacigo_scratch")
        current = reader.identity_and_inventory()
        self.assertEqual(current["database"], "espacigo_scratch")
        self.assertEqual(current["login"], "postgres")
        self.assertEqual(current["server_version_major"], 18)
        self.assertFalse(current["runtime_public_create"])
        self.assertTrue(current["runtime_membership_set"])
        self.assertFalse(current["runtime_membership_inherit"])
        with self.assertRaises(RecoveryPlanError):
            reader.statement_facts(NORMALIZATION)
        reader.expected_statement_object_counts = {
            "tables": 3, "sequences": 2, "routines": 1,
            "dictionaries": 1, "configurations": 1, "types": 0,
        }
        facts, sequence_parents = reader.statement_facts(NORMALIZATION)
        self.assertEqual([facts[ids[i]] for i in (1, 2, 3, 4, 5, 6)], ["postgres"] * 6)
        plan = build_normalization_recovery_plan(
            NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts,
            owned_sequence_parent_statement=sequence_parents,
            binding={"synthetic_fixture_catalog_sha256": hashlib.sha256(
                json.dumps(current, sort_keys=True).encode()).hexdigest()},
        )
        self.assertNotIn(b"recovery_parent_id_seq OWNER", plan.pending_sql)
        self.assertIn(b"recovery_standalone_seq OWNER", plan.pending_sql)
        self.psql("BEGIN;\n" + plan.pending_sql.decode() + "COMMIT;", user="postgres")
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
