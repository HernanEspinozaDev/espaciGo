-- Rollback-only acceptance for V86 bounded declaration-revision purge.
-- Invoke with psql -X -v ON_ERROR_STOP=1 and exact journaled variables:
--   -v fixture_account_id='<authorized UUID>'
--   -v fixture_revision_ids='{<101 canonical UUIDs>}'
-- This proves SQL behavior only; ciphertext below is constraint-only synthetic
-- bytes, not application encryption or evidence of encryption correctness.
\set ON_ERROR_STOP on
BEGIN;
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '10s';

DO $target$
DECLARE
    v_count bigint;
    v_checksum text;
BEGIN
    IF current_database() <> 'espacigo_dms_stage_20261020'
       OR (SELECT oid FROM pg_catalog.pg_database
            WHERE datname = current_database()) <> 16388
       OR current_setting('server_version_num') <> '180006'
       OR session_user <> 'postgres' OR current_user <> 'postgres' THEN
        RAISE EXCEPTION 'unexpected native target';
    END IF;
    SELECT count(*), min(checksum)
      INTO v_count, v_checksum
      FROM public.schema_migrations;
    IF v_count <> 91 OR v_checksum IS NULL THEN
        RAISE EXCEPTION 'unexpected migration ledger count';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM public.schema_migrations
         WHERE version = 91
           AND checksum = '6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250'
    ) THEN
        RAISE EXCEPTION 'V91 ledger checksum mismatch';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_proc p
         WHERE p.oid = 'public.purge_expired_identity_declaration_revisions()'::regprocedure
           AND p.prosecdef
           AND pg_catalog.pg_get_userbyid(p.proowner) = 'espacigo_schema_owner'
    ) THEN
        RAISE EXCEPTION 'V86 purge function missing or unexpected owner/security mode';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles r
         WHERE r.rolname = 'espacigo_privacy_cleanup_worker'
           AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit
           AND NOT r.rolcreaterole AND NOT r.rolcreatedb
           AND NOT r.rolreplication AND NOT r.rolbypassrls
    ) OR NOT pg_catalog.has_function_privilege(
        'espacigo_privacy_cleanup_worker',
        'public.purge_expired_identity_declaration_revisions()', 'EXECUTE'
    ) THEN
        RAISE EXCEPTION 'required non-LOGIN cleanup worker role/EXECUTE contract absent';
    END IF;
END
$target$;

-- Hold the fixture and referenced-attempt writer boundaries for this short
-- rollback-only rehearsal. Do not let another writer change eligibility.
LOCK TABLE public.identity_declaration_revision IN EXCLUSIVE MODE;
LOCK TABLE public.identity_provider_attempt IN SHARE MODE;
CREATE TEMP TABLE n08_fixture_scope (
    ordinal integer PRIMARY KEY,
    revision_id uuid UNIQUE NOT NULL
) ON COMMIT DROP;
CREATE TEMP TABLE n08_fixture_account (account_id uuid PRIMARY KEY) ON COMMIT DROP;
INSERT INTO n08_fixture_account VALUES (:'fixture_account_id'::uuid);
INSERT INTO n08_fixture_scope (ordinal, revision_id)
SELECT ord::integer, id
  FROM unnest(:'fixture_revision_ids'::uuid[]) WITH ORDINALITY AS supplied(id, ord);

DO $fixture_guard$
DECLARE
    v_account uuid := (SELECT account_id FROM pg_temp.n08_fixture_account);
BEGIN
    IF (SELECT count(*) FROM pg_temp.n08_fixture_scope) <> 101
       OR (SELECT min(ordinal) FROM pg_temp.n08_fixture_scope) <> 1
       OR (SELECT max(ordinal) FROM pg_temp.n08_fixture_scope) <> 101 THEN
        RAISE EXCEPTION 'journal allowlist must contain exactly 101 distinct UUIDs';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM public.usuario WHERE id = v_account AND estado='activo') THEN
        RAISE EXCEPTION 'authorized fixture account is absent';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_temp.n08_fixture_scope s
        JOIN public.identity_declaration_revision d ON d.id = s.revision_id
    ) THEN
        RAISE EXCEPTION 'allowlisted revision UUID already exists';
    END IF;
    IF EXISTS (SELECT 1 FROM public.identity_declaration_revision WHERE account_id = v_account)
       OR EXISTS (SELECT 1 FROM public.identity_provider_attempt WHERE account_id = v_account)
       OR EXISTS (SELECT 1 FROM public.identity_rut_claim WHERE account_id = v_account)
       OR EXISTS (SELECT 1 FROM public.evento_auditoria_local
                   WHERE actor_id = v_account AND recurso_tipo='identity_declaration') THEN
        RAISE EXCEPTION 'authorized fixture account has pre-existing identity artifacts';
    END IF;
END
$fixture_guard$;

-- Serialize against writers before inspecting the global eligible set. No
-- foreign eligible rows may be present; otherwise the real purge could delete them.
DO $foreign_guard$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM public.identity_declaration_revision d
         WHERE d.retention_at <= pg_catalog.clock_timestamp() + interval '2 minutes'
           AND NOT EXISTS (
               SELECT 1 FROM public.identity_provider_attempt a
                WHERE a.declaration_id = d.id AND a.account_id = d.account_id
           )
           AND NOT EXISTS (
               SELECT 1 FROM pg_temp.n08_fixture_scope s WHERE s.revision_id = d.id
           )
    ) THEN
        RAISE EXCEPTION 'foreign eligible expired declaration revisions exist';
    END IF;
END
$foreign_guard$;

WITH stamp AS (SELECT pg_catalog.clock_timestamp() - interval '31 days' AS created)
INSERT INTO public.identity_declaration_revision
    (id, account_id, revision, key_version, ciphertext, schema_version,
     consent_version, consent_given_at, retention_at, created_at)
SELECT s.revision_id, :'fixture_account_id'::uuid, s.ordinal,
       'n08fixture', decode(repeat('ab', 29), 'hex'), '1',
       'id-consent-20261108', stamp.created,
       stamp.created + interval '30 days', stamp.created
  FROM pg_temp.n08_fixture_scope s CROSS JOIN stamp;

SET LOCAL ROLE espacigo_privacy_cleanup_worker;
SELECT public.purge_expired_identity_declaration_revisions() = 100 AS first_ok \gset
\if :first_ok
\else
  ROLLBACK;
  \echo 'FAIL: first V86 purge call did not return 100'
  \quit 1
\endif
RESET ROLE;

DO $after_first$
BEGIN
    IF (SELECT count(*) FROM public.identity_declaration_revision d
         JOIN pg_temp.n08_fixture_scope s ON s.revision_id = d.id) <> 1
       THEN
        RAISE EXCEPTION 'after first purge, expected exactly one allowlisted row';
    END IF;
END
$after_first$;

SET LOCAL ROLE espacigo_privacy_cleanup_worker;
SELECT public.purge_expired_identity_declaration_revisions() = 1 AS second_ok \gset
\if :second_ok
\else
  ROLLBACK;
  \echo 'FAIL: second V86 purge call did not return 1'
  \quit 1
\endif
RESET ROLE;

DO $after_second$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.identity_declaration_revision d
        JOIN pg_temp.n08_fixture_scope s ON s.revision_id = d.id
    ) THEN
        RAISE EXCEPTION 'second purge left allowlisted fixture revisions';
    END IF;
END
$after_second$;

ROLLBACK;
