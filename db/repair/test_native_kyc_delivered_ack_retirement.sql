-- Rollback-only acceptance for candidate V94 delivered-session outbox retirement.
-- Peer-admin rehearsal starts BEGIN, assumes the migrator session and owner role
-- to apply candidate V94, then RESET ROLE / RESET SESSION AUTHORIZATION before
-- this fixture setup. Candidate migration guards still require the migrator.
-- Supply exactly 110 trusted, journaled UUIDs (101 eligible + 9 protected):
--   psql -X -v ON_ERROR_STOP=1 -v rehearsal_owner=true \
--     -v fixture_outbox_ids='{uuid,...}' -f this-file.sql
-- rehearsal_owner=true means the caller already opened BEGIN and returned to
-- its original postgres peer identity. Otherwise this script opens BEGIN.
-- All fixture timestamps are clock-relative eligibility fixtures, not evidence
-- that 30 days elapsed. This script never applies a migration or commits.
\set ON_ERROR_STOP on
\if :{?rehearsal_owner}
\else
  \set rehearsal_owner false
\endif
\if :{?fixture_outbox_ids}
\else
  DO $failure$ BEGIN RAISE EXCEPTION 'fixture_outbox_ids requires 110 journaled UUIDs'; END $failure$;
\endif
\if :rehearsal_owner
\else
BEGIN;
\endif
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '20s';

DO $target$
DECLARE v_count bigint;
BEGIN
    IF current_database() <> 'espacigo_dms_stage_20261020'
       OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database()) <> 16388
       OR current_setting('server_version_num') <> '180006'
       OR current_setting('port') <> '5432'
       OR session_user <> 'postgres' OR current_user <> 'postgres' THEN
        RAISE EXCEPTION 'unexpected native PostgreSQL target or session identity';
    END IF;
    SELECT count(*) INTO v_count FROM public.schema_migrations;
    IF v_count NOT IN (93,94) THEN RAISE EXCEPTION 'unexpected migration ledger count'; END IF;
    IF (SELECT min(version) FROM public.schema_migrations) <> 1
       OR (SELECT max(version) FROM public.schema_migrations) <> v_count
       OR (SELECT checksum FROM public.schema_migrations WHERE version=93)
          IS DISTINCT FROM '885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292' THEN
        RAISE EXCEPTION 'unexpected native ledger boundary or applied V93 checksum';
    END IF;
    IF v_count=93 AND EXISTS (SELECT 1 FROM public.schema_migrations WHERE version=94) THEN
        RAISE EXCEPTION 'ledger count/version disagreement';
    END IF;
    IF v_count=94 AND NOT EXISTS (SELECT 1 FROM public.schema_migrations WHERE version=94) THEN
        RAISE EXCEPTION 'ledger count/version disagreement';
    END IF;
    IF to_regprocedure('public.purge_delivered_identity_session_outbox()') IS NULL THEN
        RAISE EXCEPTION 'candidate V94 function absent';
    END IF;
END
$target$;

LOCK TABLE public.outbox_evento_local IN SHARE ROW EXCLUSIVE MODE;
CREATE TEMP TABLE ack_fixture_ids (ordinal integer PRIMARY KEY, id uuid UNIQUE NOT NULL) ON COMMIT DROP;
INSERT INTO pg_temp.ack_fixture_ids(ordinal,id)
SELECT ord::integer, supplied.id
FROM unnest(:'fixture_outbox_ids'::uuid[]) WITH ORDINALITY AS supplied(id,ord);
DO $fixture_guard$
BEGIN
    IF (SELECT count(*) FROM pg_temp.ack_fixture_ids) <> 110
       OR (SELECT min(ordinal) FROM pg_temp.ack_fixture_ids) <> 1
       OR (SELECT max(ordinal) FROM pg_temp.ack_fixture_ids) <> 110 THEN
        RAISE EXCEPTION 'fixture journal must contain exactly 110 unique UUIDs';
    END IF;
    IF EXISTS (SELECT 1 FROM public.outbox_evento_local o JOIN pg_temp.ack_fixture_ids f ON f.id=o.id) THEN
        RAISE EXCEPTION 'a journaled UUID already exists in outbox';
    END IF;
    IF EXISTS (SELECT 1 FROM public.outbox_evento_local o JOIN pg_temp.ack_fixture_ids f
               ON o.clave_deduplicacion IN ('didit_session_deletion:'||f.id::text,'unrelated:'||f.id::text)) THEN
        RAISE EXCEPTION 'a journaled deduplication key already exists in outbox';
    END IF;
    IF EXISTS (
        SELECT 1 FROM public.outbox_evento_local o
        WHERE o.agregado_tipo='didit_session' AND o.tipo='didit.session_remote_deletion'
          AND starts_with(o.clave_deduplicacion,'didit_session_deletion:')
          AND char_length(o.clave_deduplicacion)>char_length('didit_session_deletion:')
          AND char_length(o.clave_deduplicacion)<=120
          AND substr(o.clave_deduplicacion,char_length('didit_session_deletion:')+1)
                = btrim(substr(o.clave_deduplicacion,char_length('didit_session_deletion:')+1))
          AND o.entregada_en IS NOT NULL AND o.retirar_en IS NOT NULL
          AND o.retirar_en<=clock_timestamp()
          AND o.retirar_en>=o.entregada_en+interval '30 days'
          AND o.cancelada_en IS NULL AND o.fallo_terminal_en IS NULL
    ) THEN RAISE EXCEPTION 'foreign eligible rows exist; refusing to exercise purge'; END IF;
END
$fixture_guard$;

CREATE TEMP TABLE ack_foreign_snapshot ON COMMIT DROP AS
SELECT o.id,o.creada_en,o.disponible_en,o.lease_hasta,o.entregada_en,o.cancelada_en,
       o.fallo_terminal_en,o.retirar_en,o.intentos
FROM public.outbox_evento_local o
WHERE NOT EXISTS (SELECT 1 FROM pg_temp.ack_fixture_ids f WHERE f.id=o.id);
CREATE TEMP TABLE ack_fixture_snapshot ON COMMIT DROP AS
SELECT * FROM public.outbox_evento_local WHERE false;

-- Eligible rows 1..101. These timestamps satisfy the delivered-row CHECK and
-- retirement predicate without modifying any pre-existing row.
INSERT INTO public.outbox_evento_local
 (id,agregado_tipo,agregado_id,tipo,clave_deduplicacion,version,creada_en,disponible_en,intentos,entregada_en,retirar_en)
SELECT id,'didit_session',id,'didit.session_remote_deletion',
       'didit_session_deletion:'||id::text,1,now()-interval '45 days',now()-interval '45 days',1,
       now()-interval '40 days',now()-interval '10 days'
FROM pg_temp.ack_fixture_ids WHERE ordinal<=101;

-- Nine protected cases: pending; future retirement; cancelled; terminal failure;
-- wrong aggregate; wrong event type; wrong prefix; less-than-30-day retention;
-- NULL retirement. Terminal/cancelled forms follow the applied V25/V27 CHECKs.
INSERT INTO public.outbox_evento_local
 (id,agregado_tipo,agregado_id,tipo,clave_deduplicacion,version,creada_en,disponible_en,intentos,
  entregada_en,cancelada_en,motivo_cancelacion_codigo,fallo_terminal_en,codigo_fallo_terminal,retirar_en)
SELECT id,'didit_session',id,'didit.session_remote_deletion','didit_session_deletion:'||id::text,
       1,now()-interval '45 days',now()-interval '45 days',1,
       CASE WHEN ordinal IN (102,104,105) THEN NULL ELSE now()-interval '40 days' END,
       CASE WHEN ordinal=104 THEN now()-interval '20 days' ELSE NULL END,
       CASE WHEN ordinal=104 THEN 'baja_local_sin_finalidad' ELSE NULL END,
       CASE WHEN ordinal=105 THEN now()-interval '40 days' ELSE NULL END,
       CASE WHEN ordinal=105 THEN 'mailpit_delivery_failed' ELSE NULL END,
       CASE WHEN ordinal=102 THEN NULL
            WHEN ordinal=103 THEN now()+interval '1 day'
            WHEN ordinal=104 THEN now()+interval '10 days'
            WHEN ordinal=105 THEN now()-interval '10 days'
            WHEN ordinal=109 THEN now()-interval '25 days'
            WHEN ordinal=110 THEN NULL
            ELSE now()-interval '10 days' END
FROM pg_temp.ack_fixture_ids WHERE ordinal BETWEEN 102 AND 110;
UPDATE public.outbox_evento_local SET agregado_tipo='another_aggregate' WHERE id=(SELECT id FROM pg_temp.ack_fixture_ids WHERE ordinal=106);
UPDATE public.outbox_evento_local SET tipo='didit.session_other_event' WHERE id=(SELECT id FROM pg_temp.ack_fixture_ids WHERE ordinal=107);
UPDATE public.outbox_evento_local SET clave_deduplicacion='unrelated:'||id::text WHERE id=(SELECT id FROM pg_temp.ack_fixture_ids WHERE ordinal=108);
INSERT INTO pg_temp.ack_fixture_snapshot SELECT * FROM public.outbox_evento_local WHERE id IN (SELECT id FROM pg_temp.ack_fixture_ids WHERE ordinal>101);

SET LOCAL ROLE espacigo_runtime;
DO $rights$
DECLARE v_state text;
BEGIN
    BEGIN
        PERFORM public.purge_delivered_identity_session_outbox();
        RAISE EXCEPTION 'runtime unexpectedly executed purge';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        DELETE FROM public.outbox_evento_local WHERE false;
        RAISE EXCEPTION 'runtime unexpectedly deleted directly';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
END
$rights$;
SET LOCAL ROLE espacigo_privacy_cleanup_worker;
DO $worker_rights$
BEGIN
 BEGIN
  DELETE FROM public.outbox_evento_local WHERE false;
  RAISE EXCEPTION 'worker unexpectedly deleted directly';
 EXCEPTION WHEN insufficient_privilege THEN NULL;
 END;
END
$worker_rights$;
DO $batch1$
DECLARE n bigint;
BEGIN
    n:=public.purge_delivered_identity_session_outbox();
    IF n<>100 THEN RAISE EXCEPTION 'first purge batch returned %, expected 100',n; END IF;
END
$batch1$;
RESET ROLE;
DO $read1$
BEGIN
    IF (SELECT count(*) FROM public.outbox_evento_local o JOIN pg_temp.ack_fixture_ids f ON f.id=o.id WHERE f.ordinal<=101)<>1 THEN
        RAISE EXCEPTION 'first batch did not remove exactly 100 eligible rows';
    END IF;
END
$read1$;
SET LOCAL ROLE espacigo_privacy_cleanup_worker;
DO $batch2$
DECLARE n bigint;
BEGIN
    n:=public.purge_delivered_identity_session_outbox();
    IF n<>1 THEN RAISE EXCEPTION 'second purge batch returned %, expected 1',n; END IF;
    n:=public.purge_delivered_identity_session_outbox();
    IF n<>0 THEN RAISE EXCEPTION 'third purge batch returned %, expected 0',n; END IF;
END
$batch2$;
RESET ROLE;
DO $readback$
BEGIN
    IF EXISTS (SELECT 1 FROM public.outbox_evento_local o JOIN pg_temp.ack_fixture_ids f ON f.id=o.id WHERE f.ordinal<=101) THEN
        RAISE EXCEPTION 'eligible fixture rows remain after 100+1+0 calls';
    END IF;
    IF EXISTS (
        (SELECT id,creada_en,disponible_en,lease_hasta,entregada_en,cancelada_en,fallo_terminal_en,retirar_en,intentos
         FROM public.outbox_evento_local WHERE NOT EXISTS (SELECT 1 FROM pg_temp.ack_fixture_ids f WHERE f.id=outbox_evento_local.id)
         EXCEPT SELECT * FROM pg_temp.ack_foreign_snapshot)
        UNION ALL
        (SELECT * FROM pg_temp.ack_foreign_snapshot EXCEPT
         SELECT id,creada_en,disponible_en,lease_hasta,entregada_en,cancelada_en,fallo_terminal_en,retirar_en,intentos
         FROM public.outbox_evento_local WHERE NOT EXISTS (SELECT 1 FROM pg_temp.ack_fixture_ids f WHERE f.id=outbox_evento_local.id))
    ) THEN RAISE EXCEPTION 'foreign rows/timestamps changed'; END IF;
    IF EXISTS (
        (SELECT id,creada_en,disponible_en,lease_hasta,entregada_en,cancelada_en,fallo_terminal_en,retirar_en,intentos
         FROM public.outbox_evento_local WHERE id IN (SELECT id FROM pg_temp.ack_fixture_ids WHERE ordinal>101)
         EXCEPT SELECT id,creada_en,disponible_en,lease_hasta,entregada_en,cancelada_en,fallo_terminal_en,retirar_en,intentos FROM pg_temp.ack_fixture_snapshot)
        UNION ALL
        (SELECT id,creada_en,disponible_en,lease_hasta,entregada_en,cancelada_en,fallo_terminal_en,retirar_en,intentos FROM pg_temp.ack_fixture_snapshot
         EXCEPT SELECT id,creada_en,disponible_en,lease_hasta,entregada_en,cancelada_en,fallo_terminal_en,retirar_en,intentos
         FROM public.outbox_evento_local WHERE id IN (SELECT id FROM pg_temp.ack_fixture_ids WHERE ordinal>101))
    ) THEN RAISE EXCEPTION 'protected fixture rows changed'; END IF;
END
$readback$;
\echo 'PASS: native V94 bounded ACK retirement acceptance (rollback-only)'
ROLLBACK;
