-- Native-only SQL acceptance: historical claim is not a new approval, but a
-- fresh eligible authoritative commit can use it without losing anti-reuse.
-- Constraint-only synthetic bytes, not application encryption evidence.
-- Private journal supplies two account IDs and arrays of two revision/attempt/event IDs.
\set ON_ERROR_STOP on
\if :{?with_previous_attempt}
\else
 \set with_previous_attempt false
\endif
\if :{?rehearsal_owner}
\else
 \set rehearsal_owner false
\endif
\if :rehearsal_owner
\else
 BEGIN;
\endif
SET LOCAL lock_timeout='3s';
SET LOCAL statement_timeout='15s';
DO $target$
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR NOT ((session_user='postgres' AND current_user='postgres')
  OR (session_user='espacigo_migrator_login' AND current_user='espacigo_schema_owner'))
 OR (SELECT count(*) FROM public.schema_migrations) NOT IN (92,93)
 OR NOT EXISTS (SELECT 1 FROM public.schema_migrations WHERE version=92
 AND checksum='bf7db344af0752ac479b4422dfa0e770cd11623c3be2b51c4449dba89c93cc03') THEN
  RAISE EXCEPTION 'native reverification target/ledger mismatch';
 END IF;
END
$target$;
LOCK TABLE public.identity_provider_attempt,public.identity_provider_event,
 public.identity_rut_claim,public.identity_declaration_revision IN SHARE ROW EXCLUSIVE MODE;
SELECT cardinality(:'fixture_revision_ids'::uuid[])=2
 AND cardinality(:'fixture_attempt_ids'::uuid[])=2
 AND cardinality(:'fixture_event_ids'::uuid[])=2 AS ids_ok \gset
\if :ids_ok
\else
 DO $failure$ BEGIN RAISE EXCEPTION 'native reverification journal bounds invalid'; END $failure$;
\endif
CREATE TEMP TABLE reverify_scope(ordinal integer PRIMARY KEY,account_id uuid UNIQUE NOT NULL,
 declaration_id uuid UNIQUE NOT NULL,attempt_id uuid UNIQUE NOT NULL,event_id uuid UNIQUE NOT NULL) ON COMMIT DROP;
INSERT INTO reverify_scope
SELECT n, CASE n WHEN 1 THEN :'fixture_account_id'::uuid ELSE :'fixture_other_account_id'::uuid END,
 (:'fixture_revision_ids'::uuid[])[n],(:'fixture_attempt_ids'::uuid[])[n],(:'fixture_event_ids'::uuid[])[n]
FROM generate_series(1,2) n;
CREATE TEMP TABLE reverify_outbox_before AS SELECT clave_deduplicacion FROM public.outbox_evento_local;
CREATE TEMP TABLE reverify_previous(attempt_id uuid PRIMARY KEY) ON COMMIT DROP;
\if :with_previous_attempt
 INSERT INTO reverify_previous VALUES (:'fixture_previous_attempt_id'::uuid);
\endif
CREATE TEMP TABLE reverify_stamp AS SELECT clock_timestamp()-interval '2 days' AS created,
 clock_timestamp()-interval '1 day' AS historical_verified,clock_timestamp()+interval '28 days' AS retention;
DO $scope$
BEGIN
 IF current_user='postgres' THEN
  IF (SELECT count(*) FROM reverify_scope s JOIN public.usuario u ON u.id=s.account_id AND u.estado='activo')<>2 THEN
   RAISE EXCEPTION 'authorized reverification accounts not active';
  END IF;
 END IF;
 IF EXISTS(SELECT 1 FROM public.identity_declaration_revision d JOIN reverify_scope s USING(account_id))
 OR EXISTS(SELECT 1 FROM public.identity_provider_attempt a JOIN reverify_scope s USING(account_id))
 OR EXISTS(SELECT 1 FROM public.identity_rut_claim c JOIN reverify_scope s USING(account_id)) THEN
  RAISE EXCEPTION 'reverification refuses pre-existing account artifacts';
 END IF;
END
$scope$;
INSERT INTO public.identity_declaration_revision(id,account_id,revision,key_version,ciphertext,schema_version,
 consent_version,consent_given_at,retention_at,created_at)
SELECT s.declaration_id,s.account_id,1,'synthetic-reverification',decode(repeat('ab',29),'hex'),'1',
 'id-consent-20261108',stamp.created,stamp.retention,stamp.created
FROM reverify_scope s CROSS JOIN reverify_stamp stamp;
INSERT INTO public.identity_provider_attempt(id,account_id,declaration_id,provider,environment,application_id,
 workflow_id,workflow_version,idempotency_key,correlation_id,provider_session_id,status,created_at,updated_at,retention_at)
SELECT s.attempt_id,s.account_id,s.declaration_id,'didit','SANDBOX','synthetic-reverification',
 'synthetic-reverification','1',s.attempt_id,s.attempt_id,'synthetic-reverify-'||s.attempt_id::text,
 'PROCESSING',stamp.created,stamp.created,stamp.retention
FROM reverify_scope s CROSS JOIN reverify_stamp stamp;
INSERT INTO public.identity_provider_event(id,attempt_id,provider,environment,application_id,event_id,event_type,
 body_sha256,processing_status,received_at,processed_at,retention_at)
SELECT s.event_id,s.attempt_id,'didit','SANDBOX','synthetic-reverification',s.event_id::text,'status.updated',
 decode(repeat('cd',32),'hex'),'RECEIVED',stamp.created,NULL,stamp.retention
FROM reverify_scope s CROSS JOIN reverify_stamp stamp;
INSERT INTO public.identity_rut_claim(fingerprint,account_id,declaration_id,expires_at,verified_at,created_at,updated_at)
SELECT 'v1:'||repeat('c',64),s.account_id,s.declaration_id,NULL,stamp.historical_verified,stamp.created,stamp.historical_verified
FROM reverify_scope s CROSS JOIN reverify_stamp stamp WHERE s.ordinal=1;

-- Each negative runs in a PL/pgSQL subtransaction. Catch only the intended
-- conflict SQLSTATE; a fixture/SQL error or accepted negative fails the test.
DO $negatives$
DECLARE a record; b record; scenario text; supplied_session text; supplied_event uuid; supplied_at timestamptz;
BEGIN
 SELECT * INTO a FROM reverify_scope WHERE ordinal=1;
 SELECT * INTO b FROM reverify_scope WHERE ordinal=2;
 FOREACH scenario IN ARRAY ARRAY['wrong_owner','wrong_session','wrong_event','terminal_attempt','expired_pending','effective_clock'] LOOP
  BEGIN
   supplied_session:='synthetic-reverify-'||a.attempt_id::text;
   supplied_event:=a.event_id; supplied_at:=clock_timestamp();
   CASE scenario
    WHEN 'wrong_owner' THEN UPDATE public.identity_rut_claim SET account_id=b.account_id,declaration_id=b.declaration_id WHERE account_id=a.account_id;
    WHEN 'wrong_session' THEN supplied_session:='synthetic-reverify-'||b.attempt_id::text;
    WHEN 'wrong_event' THEN supplied_event:=b.event_id;
    WHEN 'terminal_attempt' THEN UPDATE public.identity_provider_attempt SET status='FAILED' WHERE id=a.attempt_id;
    WHEN 'expired_pending' THEN UPDATE public.identity_rut_claim SET verified_at=NULL,expires_at=clock_timestamp()-interval '1 hour' WHERE account_id=a.account_id;
    WHEN 'effective_clock' THEN
     UPDATE public.identity_rut_claim SET verified_at=NULL,expires_at=created_at+interval '1 hour' WHERE account_id=a.account_id;
     UPDATE public.identity_provider_event SET received_at=received_at+interval '2 hours' WHERE id=a.event_id;
     SELECT created+interval '30 minutes' INTO supplied_at FROM reverify_stamp;
   END CASE;
   PERFORM public.commit_verified_identity_decision(a.attempt_id,supplied_event,'v1:'||repeat('c',64),supplied_session,supplied_at);
   RAISE EXCEPTION 'native reverification accepted negative scenario';
  EXCEPTION WHEN unique_violation THEN NULL;
  END;
  IF (SELECT count(*) FROM public.identity_declaration_revision d JOIN reverify_scope s ON s.declaration_id=d.id)<>2
  OR NOT EXISTS(SELECT 1 FROM public.identity_provider_attempt WHERE id=a.attempt_id AND status='PROCESSING' AND declaration_id=a.declaration_id)
  OR NOT EXISTS(SELECT 1 FROM public.identity_provider_event WHERE id=a.event_id AND processing_status='RECEIVED')
  OR NOT EXISTS(SELECT 1 FROM public.identity_rut_claim c CROSS JOIN reverify_stamp stamp WHERE c.account_id=a.account_id AND c.declaration_id=a.declaration_id AND c.verified_at=stamp.historical_verified AND c.expires_at IS NULL)
  OR EXISTS(SELECT 1 FROM public.outbox_evento_local o LEFT JOIN reverify_outbox_before x USING(clave_deduplicacion) WHERE x.clave_deduplicacion IS NULL) THEN
   RAISE EXCEPTION 'native reverification negative left partial writes';
  END IF;
 END LOOP;
END
$negatives$;

SAVEPOINT historical_approval;
-- A retry after TTL can leave another terminal attempt referencing this same
-- snapshot. It must not prevent the new authoritative approval from purging it.
INSERT INTO public.identity_provider_attempt(id,account_id,declaration_id,provider,environment,application_id,
 workflow_id,workflow_version,idempotency_key,correlation_id,provider_session_id,status,created_at,updated_at,retention_at)
SELECT p.attempt_id,s.account_id,s.declaration_id,'didit','SANDBOX','synthetic-reverification',
 'synthetic-reverification','1',p.attempt_id,p.attempt_id,'synthetic-reverify-'||p.attempt_id::text,
 'EXPIRED',stamp.created,stamp.created,stamp.retention
FROM reverify_previous p CROSS JOIN reverify_scope s CROSS JOIN reverify_stamp stamp WHERE s.ordinal=1;
SELECT (public.commit_verified_identity_decision(s.attempt_id,s.event_id,'v1:'||repeat('c',64),
 'synthetic-reverify-'||s.attempt_id::text,clock_timestamp())).id=s.attempt_id AS approved_ok
FROM reverify_scope s WHERE ordinal=1 \gset
\if :approved_ok
\else
 DO $failure$ BEGIN RAISE EXCEPTION 'historical claim authoritative commit failed'; END $failure$;
\endif
DO $historical_result$
DECLARE a record;
BEGIN
 SELECT * INTO a FROM reverify_scope WHERE ordinal=1;
 IF EXISTS(SELECT 1 FROM public.identity_declaration_revision WHERE id=a.declaration_id)
 OR NOT EXISTS(SELECT 1 FROM public.identity_provider_attempt WHERE id=a.attempt_id AND status='APPROVED' AND declaration_id IS NULL AND encrypted_decision IS NULL)
 OR NOT EXISTS(SELECT 1 FROM public.identity_provider_event WHERE id=a.event_id AND processing_status='PROCESSED')
 OR NOT EXISTS(SELECT 1 FROM public.identity_rut_claim c CROSS JOIN reverify_stamp stamp
  WHERE c.account_id=a.account_id AND c.declaration_id IS NULL AND c.verified_at=stamp.historical_verified AND c.expires_at IS NULL)
 OR NOT EXISTS(SELECT 1 FROM public.outbox_evento_local WHERE agregado_id=a.attempt_id AND agregado_tipo='didit_session'
  AND tipo='didit.session_remote_deletion' AND clave_deduplicacion='didit_session_deletion:synthetic-reverify-'||a.attempt_id::text)
 OR (SELECT count(*) FROM public.outbox_evento_local o LEFT JOIN reverify_outbox_before x USING(clave_deduplicacion) WHERE x.clave_deduplicacion IS NULL)<>1+(SELECT count(*) FROM reverify_previous)
 OR EXISTS(SELECT 1 FROM public.identity_provider_attempt prior_attempt JOIN reverify_previous p ON p.attempt_id=prior_attempt.id) THEN
  RAISE EXCEPTION 'historical claim reverification broke atomic result/purge/outbox';
 END IF;
END
$historical_result$;
ROLLBACK TO SAVEPOINT historical_approval;
RELEASE SAVEPOINT historical_approval;

-- Recreate the prior terminal attempt after the historical scenario rollback.
INSERT INTO public.identity_provider_attempt(id,account_id,declaration_id,provider,environment,application_id,
 workflow_id,workflow_version,idempotency_key,correlation_id,provider_session_id,status,created_at,updated_at,retention_at)
SELECT p.attempt_id,s.account_id,s.declaration_id,'didit','SANDBOX','synthetic-reverification',
 'synthetic-reverification','1',p.attempt_id,p.attempt_id,'synthetic-reverify-'||p.attempt_id::text,
 'FAILED',stamp.created,stamp.created,stamp.retention
FROM reverify_previous p CROSS JOIN reverify_scope s CROSS JOIN reverify_stamp stamp WHERE s.ordinal=1;

-- Pending, nonexpired claims retain their original approval path as well.
UPDATE public.identity_rut_claim SET verified_at=NULL,expires_at=(SELECT retention FROM reverify_stamp)
WHERE account_id=(SELECT account_id FROM reverify_scope WHERE ordinal=1);
SAVEPOINT pending_approval;
SELECT (public.commit_verified_identity_decision(s.attempt_id,s.event_id,'v1:'||repeat('c',64),
 'synthetic-reverify-'||s.attempt_id::text,clock_timestamp())).id=s.attempt_id AS pending_ok
FROM reverify_scope s WHERE ordinal=1 \gset
\if :pending_ok
\else
 DO $failure$ BEGIN RAISE EXCEPTION 'pending claim approval compatibility failed'; END $failure$;
\endif
DO $pending_result$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM public.identity_rut_claim c JOIN reverify_scope s USING(account_id)
  WHERE s.ordinal=1 AND c.verified_at IS NOT NULL AND c.expires_at IS NULL AND c.declaration_id IS NULL)
 OR NOT EXISTS(SELECT 1 FROM public.identity_declaration_revision d JOIN reverify_scope s ON s.declaration_id=d.id WHERE s.ordinal=2)
 OR EXISTS(SELECT 1 FROM public.identity_provider_attempt prior_attempt JOIN reverify_previous p ON p.attempt_id=prior_attempt.id)
 OR (SELECT count(*) FROM public.outbox_evento_local o LEFT JOIN reverify_outbox_before x USING(clave_deduplicacion) WHERE x.clave_deduplicacion IS NULL)<>1+(SELECT count(*) FROM reverify_previous) THEN
  RAISE EXCEPTION 'pending approval or foreign-future preservation failed';
 END IF;
END
$pending_result$;
ROLLBACK TO SAVEPOINT pending_approval;
RELEASE SAVEPOINT pending_approval;
\if :with_previous_attempt
-- A failed/uncertain creation has no known remote session ID. Keep its minimal
-- correlation for reconciliation; never report a deletion that cannot be sent.
UPDATE public.identity_provider_attempt prior_attempt SET provider_session_id=NULL
FROM reverify_previous p WHERE p.attempt_id=prior_attempt.id;
SELECT (public.commit_verified_identity_decision(s.attempt_id,s.event_id,'v1:'||repeat('c',64),
 'synthetic-reverify-'||s.attempt_id::text,clock_timestamp())).id=s.attempt_id AS unknown_ok
FROM reverify_scope s WHERE ordinal=1 \gset
\if :unknown_ok
\else
 DO $failure$ BEGIN RAISE EXCEPTION 'uncertain creation blocked safe snapshot purge'; END $failure$;
\endif
DO $unknown_result$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM public.identity_provider_attempt prior_attempt JOIN reverify_previous p ON p.attempt_id=prior_attempt.id
  WHERE prior_attempt.status='EXPIRED' AND prior_attempt.declaration_id IS NULL AND prior_attempt.provider_session_id IS NULL
   AND prior_attempt.encrypted_decision IS NULL AND prior_attempt.decision_key_version IS NULL AND prior_attempt.decision_algorithm IS NULL)
 OR EXISTS(SELECT 1 FROM public.outbox_evento_local o JOIN reverify_previous p ON p.attempt_id=o.agregado_id)
 OR (SELECT count(*) FROM public.outbox_evento_local o LEFT JOIN reverify_outbox_before x USING(clave_deduplicacion) WHERE x.clave_deduplicacion IS NULL)<>1 THEN
  RAISE EXCEPTION 'uncertain creation correlation was lost or remote deletion invented';
 END IF;
END
$unknown_result$;
\endif
ROLLBACK;
\echo 'PASS: native historical/pending reverification, six negatives, atomic purge/outbox, rollback'
