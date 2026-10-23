-- PROPOSAL ONLY: separate reviewed owner operation, not a migration replay.
-- Removes only the reported PUBLIC TEMPORARY grant; preserves CONNECT,
-- ownership, role memberships and every schema_migrations row.
BEGIN;
DO $preflight$
DECLARE
    database_oid oid;
    database_owner oid;
    database_acl aclitem[];
BEGIN
    IF current_database() <> 'espacigo_dms_stage_20261020'
       OR session_user <> 'postgres' OR current_user <> 'postgres' THEN
        RAISE EXCEPTION 'unexpected database/session for IMG-01 TEMP ACL reconciliation';
    END IF;
    SELECT oid, datdba, COALESCE(datacl, acldefault('d', datdba))
      INTO database_oid, database_owner, database_acl
      FROM pg_catalog.pg_database WHERE datname = current_database();
    IF database_owner <> 'postgres'::regrole THEN
        RAISE EXCEPTION 'database owner differs; stop for ACL review';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM aclexplode(database_acl)
                   WHERE grantee=0 AND grantor=database_owner
                     AND privilege_type='TEMPORARY' AND NOT is_grantable)
       OR NOT EXISTS (SELECT 1 FROM aclexplode(database_acl)
                      WHERE grantee=0 AND grantor=database_owner
                        AND privilege_type='CONNECT' AND NOT is_grantable)
       OR EXISTS (SELECT 1 FROM aclexplode(database_acl)
                   WHERE privilege_type='TEMPORARY'
                     AND (grantee NOT IN (0,database_owner) OR grantor<>database_owner OR is_grantable)) THEN
        RAISE EXCEPTION 'database TEMP/CONNECT ACL differs from reviewed PUBLIC baseline';
    END IF;
    IF to_regclass('public.schema_migrations') IS NULL
       OR (SELECT count(*) FROM public.schema_migrations) <> 67
       OR (SELECT max(version) FROM public.schema_migrations) <> 67 THEN
        RAISE EXCEPTION 'expected pre-upgrade V67 ledger; stop for review';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN
        RAISE EXCEPTION 'runtime role absent; stop for ACL review';
    END IF;
END
$preflight$;

REVOKE TEMPORARY ON DATABASE espacigo_dms_stage_20261020 FROM PUBLIC;

DO $postcheck$
BEGIN
    IF has_database_privilege('espacigo_runtime', current_database(), 'TEMPORARY') THEN
        RAISE EXCEPTION 'runtime still has TEMPORARY; rollback for ACL review';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_database d,
             LATERAL aclexplode(COALESCE(d.datacl,acldefault('d',d.datdba))) a
        WHERE d.datname=current_database() AND a.grantee=0
          AND a.grantor='postgres'::regrole AND a.privilege_type='CONNECT'
          AND NOT a.is_grantable
    ) OR NOT has_database_privilege('postgres', current_database(), 'TEMPORARY') THEN
        RAISE EXCEPTION 'CONNECT/owner TEMP invariant failed; rollback';
    END IF;
END
$postcheck$;
COMMIT;
