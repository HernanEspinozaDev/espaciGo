-- V95: bind newly created identity cases to the authenticated owner's saved
-- declaration revision and persist the separately acknowledged capture consent.
-- Existing cases retain NULL metadata; no historical identity data is inferred.
DO $guard$
DECLARE
    v_case oid := pg_catalog.to_regclass('public.document_verification_case');
    v_declaration oid := pg_catalog.to_regclass('public.identity_declaration_revision');
    v_purge oid := pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()');
    v_runtime oid;
    v_worker oid;
    v_owner oid := (SELECT r.oid FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_schema_owner');
BEGIN
    IF session_user <> 'espacigo_migrator_login'
       OR current_user <> 'espacigo_schema_owner'
       OR current_schema() <> 'public'
       OR NOT pg_catalog.has_schema_privilege(current_user,'public','CREATE') THEN
        RAISE EXCEPTION 'V95 requires the pinned migrator/schema-owner identity and public CREATE';
    END IF;
    SELECT r.oid INTO v_runtime FROM pg_catalog.pg_roles r
     WHERE r.rolname='espacigo_runtime' AND NOT r.rolcanlogin AND NOT r.rolsuper
       AND NOT r.rolinherit AND NOT r.rolcreaterole AND NOT r.rolcreatedb
       AND NOT r.rolreplication AND NOT r.rolbypassrls;
    SELECT r.oid INTO v_worker FROM pg_catalog.pg_roles r
     WHERE r.rolname='espacigo_privacy_cleanup_worker' AND NOT r.rolcanlogin
       AND NOT r.rolsuper AND NOT r.rolinherit AND NOT r.rolcreaterole
       AND NOT r.rolcreatedb AND NOT r.rolreplication AND NOT r.rolbypassrls;
    IF v_case IS NULL OR v_declaration IS NULL OR v_purge IS NULL
       OR v_runtime IS NULL OR v_worker IS NULL OR v_owner IS NULL
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
                   WHERE a.attrelid=v_case AND a.attname IN (
                     'identity_declaration_revision_id','document_format_declared',
                     'capture_consent_version','capture_consented_at')
                     AND a.attnum>0 AND NOT a.attisdropped) THEN
        RAISE EXCEPTION 'V95 predecessor schema or role contract is incomplete or drifted';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles r WHERE r.oid=v_owner
                    AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolinherit
                    AND NOT r.rolcreaterole AND NOT r.rolcreatedb
                    AND NOT r.rolreplication AND NOT r.rolbypassrls)
       OR (SELECT c.relowner FROM pg_catalog.pg_class c WHERE c.oid=v_case) <> v_owner
       OR (SELECT c.relowner FROM pg_catalog.pg_class c WHERE c.oid=v_declaration) <> v_owner
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
                       WHERE p.oid=v_purge AND p.proowner=v_owner AND p.prosecdef
                         AND p.proconfig=ARRAY['search_path=pg_catalog']::text[])
       OR (SELECT count(*) FROM pg_catalog.pg_proc p
            CROSS JOIN LATERAL pg_catalog.aclexplode(
                COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl
            WHERE p.oid=v_purge AND acl.grantee<>p.proowner)<>1
       OR NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
            CROSS JOIN LATERAL pg_catalog.aclexplode(
                COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl
            WHERE p.oid=v_purge AND acl.grantee=v_worker
              AND acl.grantor=p.proowner AND acl.privilege_type='EXECUTE'
              AND NOT acl.is_grantable)
       OR pg_catalog.has_function_privilege(v_runtime,v_purge,'EXECUTE')
    THEN
        RAISE EXCEPTION 'V95 requires the reviewed worker-only V92 purge and owned identity tables';
    END IF;
END
$guard$;

ALTER TABLE public.document_verification_case
    ADD COLUMN identity_declaration_revision_id uuid,
    ADD COLUMN document_format_declared text,
    ADD COLUMN capture_consent_version text,
    ADD COLUMN capture_consented_at timestamptz;

ALTER TABLE public.document_verification_case
    ADD CONSTRAINT document_verification_case_identity_declaration_fk
        FOREIGN KEY (identity_declaration_revision_id, subject_id)
        REFERENCES public.identity_declaration_revision(id, account_id)
        ON DELETE RESTRICT,
    ADD CONSTRAINT document_verification_case_identity_metadata_ck
        CHECK (
            (purpose='space'
                AND identity_declaration_revision_id IS NULL
                AND document_format_declared IS NULL
                AND capture_consent_version IS NULL
                AND capture_consented_at IS NULL)
            OR
            (purpose='identity' AND (
                (identity_declaration_revision_id IS NULL
                    AND document_format_declared IS NULL
                    AND capture_consent_version IS NULL
                    AND capture_consented_at IS NULL)
                OR
                (identity_declaration_revision_id IS NOT NULL OR state IN ('rechazada','vencida','revocada'))
                    AND document_format_declared IS NOT NULL
                    AND capture_consent_version IS NOT NULL
                    AND capture_consented_at IS NOT NULL
                    AND document_format_declared IN ('legacy','new','unsure')
                    AND capture_consent_version='identity-capture-local-v1'
                )
            )
        );

CREATE INDEX document_verification_case_identity_declaration_idx
    ON public.document_verification_case(identity_declaration_revision_id,subject_id)
    WHERE identity_declaration_revision_id IS NOT NULL;

-- Historical all-NULL identity rows remain readable/updateable. New identity
-- rows must provide the authenticated declaration link, declared format and
-- separate server-timestamped capture consent at insertion.
CREATE FUNCTION public.document_verification_identity_case_insert_guard()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
BEGIN
    IF NEW.purpose='identity'
       AND (NEW.identity_declaration_revision_id IS NULL
            OR NEW.document_format_declared IS NULL
            OR NEW.capture_consent_version IS NULL
            OR NEW.capture_consented_at IS NULL) THEN
        RAISE EXCEPTION 'new identity case requires saved declaration and capture consent'
            USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER document_verification_identity_case_insert_guard
BEFORE INSERT ON public.document_verification_case
FOR EACH ROW EXECUTE FUNCTION public.document_verification_identity_case_insert_guard();
REVOKE ALL ON FUNCTION public.document_verification_identity_case_insert_guard() FROM PUBLIC;

-- Runtime may clear an identity declaration link only as part of a terminal
-- transition. It cannot rebind an active case to another same-owner revision.
CREATE FUNCTION public.document_verification_identity_case_link_guard()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
BEGIN
    IF OLD.purpose='identity' AND NEW.purpose='identity'
       AND NEW.identity_declaration_revision_id IS DISTINCT FROM OLD.identity_declaration_revision_id
       AND (NEW.identity_declaration_revision_id IS NOT NULL
            OR NEW.state IS NULL
            OR NEW.state NOT IN ('rechazada','vencida','revocada')) THEN
        RAISE EXCEPTION 'identity declaration link can only be cleared on a terminal case transition'
            USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER document_verification_identity_case_link_guard
BEFORE UPDATE OF identity_declaration_revision_id ON public.document_verification_case
FOR EACH ROW EXECUTE FUNCTION public.document_verification_identity_case_link_guard();
REVOKE ALL ON FUNCTION public.document_verification_identity_case_link_guard() FROM PUBLIC;

-- V92 catches lock_not_available inside its per-revision subtransaction, so a
-- linked declaration skips the entire purge group and rolls back child cleanup.
-- This trigger is the race-safe backstop after V92's account advisory locks.
CREATE FUNCTION public.identity_declaration_case_retention_guard()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM public.document_verification_case c
         WHERE c.identity_declaration_revision_id=OLD.id
           AND c.subject_id=OLD.account_id
    ) THEN
        RAISE lock_not_available;
    END IF;
    RETURN OLD;
END;
$$;
CREATE TRIGGER identity_declaration_case_retention_guard
BEFORE DELETE ON public.identity_declaration_revision
FOR EACH ROW EXECUTE FUNCTION public.identity_declaration_case_retention_guard();
REVOKE ALL ON FUNCTION public.identity_declaration_case_retention_guard() FROM PUBLIC;
REVOKE ALL ON FUNCTION public.identity_declaration_case_retention_guard() FROM espacigo_runtime;

DO $runtime$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
        GRANT INSERT (identity_declaration_revision_id,document_format_declared,capture_consent_version,capture_consented_at)
            ON public.document_verification_case TO espacigo_runtime;
        GRANT UPDATE (identity_declaration_revision_id)
            ON public.document_verification_case TO espacigo_runtime;
    END IF;
END
$runtime$;
