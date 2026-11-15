-- Native-only rollback acceptance for the complete expired declaration lifecycle.
-- Inputs are generated from the private account policy and prewritten journal:
-- fixture_account_id, fixture_other_account_id, fixture_revision_ids (102 UUIDs),
-- fixture_attempt_ids (5 UUIDs), fixture_event_ids (5 UUIDs).
-- Constraint-valid bytes are synthetic SQL fixtures, not encryption evidence.
\set ON_ERROR_STOP on
\if :{?contention_only}
\else
 \set contention_only false
\endif
\if :{?rehearsal_owner}
\else
 \set rehearsal_owner false
\endif
\if :rehearsal_owner
-- Caller began under the actual migrator login/owner and installed candidate
-- DDL in this transaction. ROLLBACK discards it along with all fixture rows.
-- This mode is not evidence of worker authentication.
\else
 BEGIN;
\endif
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '15s';
DO $target$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR NOT ((session_user='postgres' AND current_user='postgres')
         OR (session_user='espacigo_migrator_login' AND current_user='espacigo_schema_owner'))
 OR (SELECT count(*) FROM public.schema_migrations) NOT IN (91,92)
 OR NOT EXISTS (SELECT 1 FROM public.schema_migrations WHERE version=91
 AND checksum='6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250') THEN
  RAISE EXCEPTION 'native expired lifecycle target/ledger mismatch';
 END IF;
END
$target$;
-- Serialize all production writer boundaries, then reject any foreign candidate.
-- Locks apply only to this short rollback fixture, not the production function.
LOCK TABLE public.identity_provider_attempt, public.identity_provider_event,
 public.identity_rut_claim, public.identity_declaration_revision IN SHARE ROW EXCLUSIVE MODE;
CREATE TEMP TABLE lifecycle_accounts(account_id uuid PRIMARY KEY, slot integer UNIQUE) ON COMMIT DROP;
INSERT INTO lifecycle_accounts VALUES (:'fixture_account_id'::uuid,1),(:'fixture_other_account_id'::uuid,2);
CREATE TEMP TABLE lifecycle_revisions(ordinal integer PRIMARY KEY,id uuid UNIQUE NOT NULL) ON COMMIT DROP;
INSERT INTO lifecycle_revisions SELECT ord::integer,id FROM unnest(:'fixture_revision_ids'::uuid[]) WITH ORDINALITY x(id,ord);
CREATE TEMP TABLE lifecycle_attempts(ordinal integer PRIMARY KEY,id uuid UNIQUE NOT NULL) ON COMMIT DROP;
INSERT INTO lifecycle_attempts SELECT ord::integer,id FROM unnest(:'fixture_attempt_ids'::uuid[]) WITH ORDINALITY x(id,ord);
CREATE TEMP TABLE lifecycle_events(ordinal integer PRIMARY KEY,id uuid UNIQUE NOT NULL) ON COMMIT DROP;
INSERT INTO lifecycle_events SELECT ord::integer,id FROM unnest(:'fixture_event_ids'::uuid[]) WITH ORDINALITY x(id,ord);
CREATE TEMP TABLE lifecycle_outbox_before AS SELECT clave_deduplicacion FROM public.outbox_evento_local;
DO $scope$
BEGIN
 IF (SELECT count(*) FROM lifecycle_revisions)<>102
 OR (SELECT count(*) FROM lifecycle_attempts)<>5
 OR (SELECT count(*) FROM lifecycle_events)<>5
 OR (SELECT count(*) FROM lifecycle_accounts)<>2 THEN
  RAISE EXCEPTION 'native lifecycle journal/account scope invalid';
 END IF;
 -- The owner rehearsal has no account SELECT permission: require the separate
 -- authenticated account-policy/active-state preflight before invoking it.
 -- Do not widen that principal merely to observe account metadata.
 IF current_user='postgres' THEN
  IF (SELECT count(*) FROM lifecycle_accounts a JOIN public.usuario u ON u.id=a.account_id AND u.estado='activo')<>2 THEN
   RAISE EXCEPTION 'authorized native lifecycle accounts are not active';
  END IF;
 END IF;
 IF EXISTS(SELECT 1 FROM public.identity_declaration_revision d JOIN lifecycle_accounts s USING(account_id))
 OR EXISTS(SELECT 1 FROM public.identity_provider_attempt a JOIN lifecycle_accounts s USING(account_id))
 OR EXISTS(SELECT 1 FROM public.identity_rut_claim c JOIN lifecycle_accounts s USING(account_id)) THEN
  RAISE EXCEPTION 'native lifecycle refuses existing account artifacts';
 END IF;
 IF EXISTS(SELECT 1 FROM public.identity_declaration_revision WHERE retention_at<=clock_timestamp()+interval '2 minutes') THEN
  RAISE EXCEPTION 'native lifecycle refuses foreign eligible revisions';
 END IF;
 IF EXISTS(SELECT 1 FROM public.identity_declaration_revision d JOIN lifecycle_revisions s ON s.id=d.id)
 OR EXISTS(SELECT 1 FROM public.identity_provider_attempt a JOIN lifecycle_attempts s ON s.id=a.id)
 OR EXISTS(SELECT 1 FROM public.identity_provider_event e JOIN lifecycle_events s ON s.id=e.id) THEN
  RAISE EXCEPTION 'native lifecycle refuses preexisting journal IDs';
 END IF;
END
$scope$;
WITH stamps AS (SELECT clock_timestamp()-interval '31 days' AS old,clock_timestamp() AS fresh)
INSERT INTO public.identity_declaration_revision(id,account_id,revision,key_version,ciphertext,schema_version,
 consent_version,consent_given_at,retention_at,created_at)
SELECT s.id,:'fixture_account_id'::uuid,s.ordinal,'synthetic-retention',decode(repeat('ab',29),'hex'),'1',
 'id-consent-20261108',CASE WHEN ordinal=102 THEN fresh ELSE old END,
 CASE WHEN ordinal=102 THEN fresh ELSE old END+interval '30 days',
 CASE WHEN ordinal=102 THEN fresh ELSE old END
FROM lifecycle_revisions s CROSS JOIN stamps;
INSERT INTO public.identity_provider_attempt(id,account_id,declaration_id,provider,environment,application_id,
 workflow_id,workflow_version,idempotency_key,correlation_id,provider_session_id,status,created_at,updated_at,retention_at)
SELECT a.id,d.account_id,d.id,'didit','SANDBOX','synthetic-retention','synthetic-retention','1',a.id,a.id,
 'synthetic-retention-'||a.id::text,
 CASE a.ordinal WHEN 1 THEN 'PENDING_USER_INPUT' WHEN 2 THEN 'FAILED' WHEN 3 THEN 'EXPIRED' WHEN 4 THEN 'REJECTED' ELSE 'CREATED' END,
 d.created_at,d.created_at,d.retention_at
FROM lifecycle_attempts a JOIN lifecycle_revisions s ON s.ordinal=CASE WHEN a.ordinal=5 THEN 102 ELSE a.ordinal END
JOIN public.identity_declaration_revision d ON d.id=s.id;
INSERT INTO public.identity_provider_event(id,attempt_id,provider,environment,application_id,event_id,event_type,
 body_sha256,processing_status,received_at,processed_at,retention_at)
SELECT e.id,a.id,a.provider,a.environment,a.application_id,e.id::text,'status.updated',decode(repeat('cd',32),'hex'),
 'RECEIVED',a.created_at,NULL,a.retention_at
FROM lifecycle_events e JOIN lifecycle_attempts s USING(ordinal) JOIN public.identity_provider_attempt a ON a.id=s.id;
INSERT INTO public.identity_rut_claim(fingerprint,account_id,declaration_id,expires_at,verified_at,created_at,updated_at)
SELECT 'synthetic-pending:'||repeat('a',64),d.account_id,d.id,d.retention_at,NULL,d.created_at,d.created_at
FROM public.identity_declaration_revision d JOIN lifecycle_revisions s ON s.id=d.id WHERE s.ordinal=1;
INSERT INTO public.identity_rut_claim(fingerprint,account_id,declaration_id,expires_at,verified_at,created_at,updated_at)
SELECT 'synthetic-verified:'||repeat('b',64),account_id,NULL,NULL,clock_timestamp(),clock_timestamp()-interval '1 minute',clock_timestamp()
FROM lifecycle_accounts WHERE slot=2;
\if :contention_only
-- Another authenticated connection holds this fixture account's namespace 1.
-- A failed try-lock must release namespace 0 as well, before the caller ends
-- its outer transaction. Merely returning zero does not prove this property.
DO $holder_before$
BEGIN
 IF pg_catalog.pg_try_advisory_xact_lock(pg_catalog.hashtextextended(
    (SELECT account_id::text FROM lifecycle_accounts WHERE slot=1),1)) THEN
  RAISE EXCEPTION 'contention rehearsal requires an external namespace-1 holder';
 END IF;
END
$holder_before$;
\if :rehearsal_owner
\else
 SET LOCAL ROLE espacigo_privacy_cleanup_worker;
\endif
SELECT public.purge_expired_identity_declaration_revisions() = 0 AS busy_ok \gset
\if :busy_ok
\else
 DO $failure$ BEGIN RAISE EXCEPTION 'busy account was not skipped'; END $failure$;
\endif
\if :rehearsal_owner
\else
 RESET ROLE;
\endif
DO $busy$
BEGIN
 IF pg_catalog.pg_try_advisory_xact_lock(pg_catalog.hashtextextended(
    (SELECT account_id::text FROM lifecycle_accounts WHERE slot=1),1)) THEN
  RAISE EXCEPTION 'external namespace-1 holder ended before observation';
 END IF;
 IF EXISTS(SELECT 1 FROM pg_catalog.pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory')
 OR (SELECT count(*) FROM public.identity_declaration_revision d JOIN lifecycle_revisions s ON s.id=d.id)<>102
 OR (SELECT count(*) FROM public.identity_provider_attempt a JOIN lifecycle_attempts s ON s.id=a.id)<>5
 OR (SELECT count(*) FROM public.identity_provider_event e JOIN lifecycle_events s ON s.id=e.id)<>5
 OR (SELECT count(*) FROM public.outbox_evento_local o LEFT JOIN lifecycle_outbox_before b USING(clave_deduplicacion) WHERE b.clave_deduplicacion IS NULL)<>0 THEN
  RAISE EXCEPTION 'busy group retained locks or partial lifecycle mutations';
 END IF;
END
$busy$;
ROLLBACK;
\echo 'PASS: native busy group skipped without retained advisory locks or partial writes'
\quit
\endif
-- A conflicting durable intent must abort the entire invocation, including
-- earlier groups. The existing key is a fixture, not a foreign outbox row.
SAVEPOINT collision_fixture;
INSERT INTO public.outbox_evento_local(id,agregado_tipo,agregado_id,tipo,clave_deduplicacion,version,creada_en,disponible_en)
SELECT gen_random_uuid(),'didit_session',(SELECT id FROM lifecycle_attempts WHERE ordinal=5),
 'didit.session_remote_deletion','didit_session_deletion:synthetic-retention-'||id::text,1,clock_timestamp(),clock_timestamp()
FROM lifecycle_attempts WHERE ordinal=1;
\if :rehearsal_owner
\else
 SET LOCAL ROLE espacigo_privacy_cleanup_worker;
\endif
DO $collision$
BEGIN
 BEGIN
  PERFORM public.purge_expired_identity_declaration_revisions();
  RAISE EXCEPTION 'expired lifecycle accepted a conflicting outbox binding';
 EXCEPTION WHEN unique_violation THEN
  NULL;
 END;
END
$collision$;
\if :rehearsal_owner
\else
 RESET ROLE;
\endif
DO $atomicity$
BEGIN
 IF (SELECT count(*) FROM public.identity_declaration_revision d JOIN lifecycle_revisions s ON s.id=d.id)<>102
 OR (SELECT count(*) FROM public.identity_provider_attempt a JOIN lifecycle_attempts s ON s.id=a.id)<>5
 OR (SELECT count(*) FROM public.identity_provider_event e JOIN lifecycle_events s ON s.id=e.id)<>5
 OR (SELECT count(*) FROM public.identity_rut_claim c JOIN lifecycle_accounts a USING(account_id))<>2
 OR (SELECT count(*) FROM public.outbox_evento_local o LEFT JOIN lifecycle_outbox_before b USING(clave_deduplicacion) WHERE b.clave_deduplicacion IS NULL)<>1 THEN
  RAISE EXCEPTION 'expired lifecycle collision left partial writes';
 END IF;
END
$atomicity$;
ROLLBACK TO SAVEPOINT collision_fixture;
RELEASE SAVEPOINT collision_fixture;
\if :rehearsal_owner
\else
 SET LOCAL ROLE espacigo_privacy_cleanup_worker;
\endif
SELECT public.purge_expired_identity_declaration_revisions() = 100 AS first_ok \gset
\if :first_ok
\else
 DO $failure$ BEGIN RAISE EXCEPTION 'expired lifecycle first batch must purge 100 including referenced revisions'; END $failure$;
\endif
SELECT public.purge_expired_identity_declaration_revisions() = 1 AS second_ok \gset
\if :second_ok
\else
 DO $failure$ BEGIN RAISE EXCEPTION 'expired lifecycle second batch must purge the final expired revision'; END $failure$;
\endif
SELECT public.purge_expired_identity_declaration_revisions() = 0 AS third_ok \gset
\if :third_ok
\else
 DO $failure$ BEGIN RAISE EXCEPTION 'expired lifecycle purged a future revision'; END $failure$;
\endif
\if :rehearsal_owner
\else
 RESET ROLE;
\endif
DO $verify$
BEGIN
 IF (SELECT count(*) FROM public.identity_declaration_revision d JOIN lifecycle_revisions s ON s.id=d.id)<>1
 OR NOT EXISTS(SELECT 1 FROM public.identity_declaration_revision d JOIN lifecycle_revisions s ON s.id=d.id WHERE s.ordinal=102)
 OR (SELECT count(*) FROM public.identity_provider_attempt a JOIN lifecycle_attempts s ON s.id=a.id)<>1
 OR (SELECT count(*) FROM public.identity_provider_event e JOIN lifecycle_events s ON s.id=e.id)<>1 THEN
  RAISE EXCEPTION 'expired lifecycle did not preserve exactly the future dependency chain';
 END IF;
 IF EXISTS(SELECT 1 FROM public.identity_rut_claim c JOIN lifecycle_accounts a USING(account_id) WHERE a.slot=1)
 OR NOT EXISTS(SELECT 1 FROM public.identity_rut_claim c JOIN lifecycle_accounts a USING(account_id)
 WHERE a.slot=2 AND c.verified_at IS NOT NULL AND c.declaration_id IS NULL) THEN
  RAISE EXCEPTION 'expired lifecycle pending/verified claim policy mismatch';
 END IF;
 IF (SELECT count(*) FROM public.outbox_evento_local o JOIN lifecycle_attempts a ON a.id=o.agregado_id
 WHERE a.ordinal<=4 AND o.agregado_tipo='didit_session' AND o.tipo='didit.session_remote_deletion'
 AND o.clave_deduplicacion='didit_session_deletion:synthetic-retention-'||a.id::text
 )<>4 THEN
  RAISE EXCEPTION 'expired lifecycle missing exact durable remote deletion intents';
 END IF;
 IF (SELECT count(*) FROM public.outbox_evento_local o LEFT JOIN lifecycle_outbox_before b USING(clave_deduplicacion) WHERE b.clave_deduplicacion IS NULL)<>4
 OR EXISTS(SELECT 1 FROM lifecycle_outbox_before b LEFT JOIN public.outbox_evento_local o USING(clave_deduplicacion) WHERE o.clave_deduplicacion IS NULL) THEN
  RAISE EXCEPTION 'expired lifecycle changed unrelated outbox or added unexpected intents';
 END IF;
END
$verify$;
ROLLBACK;
\echo 'PASS: native expired lifecycle rollback acceptance; no committed fixture deletion'
