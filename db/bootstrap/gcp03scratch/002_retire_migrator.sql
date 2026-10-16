-- GCP-03 scratch executor phase: retire temporary login, schema revision 1.
-- This phase disables the login first. The operator then removes the remaining
-- cloudsqlsuperuser membership through Cloud SQL Admin API users.update.
DO $guard$
BEGIN
    IF current_database() <> 'espacigo_scratch' THEN
        RAISE EXCEPTION 'GCP-03 retirement refused unexpected database';
    END IF;
    IF current_user <> 'espacigo_scratch_migrator' THEN
        RAISE EXCEPTION 'GCP-03 retirement refused unexpected login';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
        JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
        WHERE member_role.rolname = 'espacigo_scratch_migrator'
          AND granted_role.rolname <> 'cloudsqlsuperuser'
    ) THEN
        RAISE EXCEPTION 'GCP-03 temporary login has an unexpected role membership';
    END IF;
END
$guard$;

ALTER ROLE espacigo_scratch_migrator NOLOGIN NOCREATEROLE NOCREATEDB NOINHERIT;

DO $verify$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles
        WHERE rolname = 'espacigo_scratch_migrator'
          AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit)
    ) THEN
        RAISE EXCEPTION 'GCP-03 temporary login still has active privileges';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
        JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
        WHERE (member_role.rolname = 'espacigo_scratch_migrator'
               AND granted_role.rolname <> 'cloudsqlsuperuser')
           OR granted_role.rolname = 'espacigo_scratch_migrator'
    ) THEN
        RAISE EXCEPTION 'GCP-03 temporary login has an unexpected role membership';
    END IF;
END
$verify$;
