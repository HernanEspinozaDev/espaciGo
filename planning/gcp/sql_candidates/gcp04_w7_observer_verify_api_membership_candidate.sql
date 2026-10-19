-- REVIEW CANDIDATE ONLY. Read-only catalog verification after a separately
-- authorized Cloud SQL users.insert call. Never normalizes or repairs state.
\set ON_ERROR_STOP on
BEGIN READ ONLY;
DO $verify$
DECLARE
    login_role pg_catalog.pg_roles%ROWTYPE;
    membership record;
    executor_oid oid;
    expected_group_members integer;
    login_found boolean;
BEGIN
    IF current_database() <> 'espacigo' THEN RAISE EXCEPTION 'W7 verify requires database espacigo'; END IF;
    IF has_database_privilege('public', current_database(), 'TEMP') THEN
        RAISE EXCEPTION 'W7 PUBLIC TEMP baseline differs from the reviewed runtime contract';
    END IF;
    SELECT * INTO login_role FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer';
    login_found := FOUND;
    IF NOT login_found THEN
        RAISE EXCEPTION 'W7 API-created login is absent';
    END IF;
    SELECT oid INTO executor_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor';
    IF NOT login_role.rolcanlogin OR NOT login_role.rolinherit OR login_role.rolsuper
       OR login_role.rolcreatedb OR login_role.rolcreaterole
       OR login_role.rolreplication OR login_role.rolbypassrls THEN
        RAISE EXCEPTION 'W7 API-created login must be LOGIN INHERIT with elevated attributes disabled';
    END IF;
    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles member ON member.oid=m.member
        WHERE member.oid=login_role.oid) <> 1 THEN
        RAISE EXCEPTION 'W7 login has unexpected additional role memberships';
    END IF;
    SELECT m.admin_option, m.inherit_option, m.set_option, granted.rolname AS granted
      INTO membership
      FROM pg_catalog.pg_auth_members m
      JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid
     WHERE m.member=login_role.oid;
    IF NOT FOUND OR membership.granted <> 'espacigo_db_observer'
       OR membership.admin_option OR NOT membership.inherit_option OR membership.set_option THEN
        RAISE EXCEPTION 'W7 API membership must be exactly ADMIN=false INHERIT=true SET=false: %', row_to_json(membership);
    END IF;
    expected_group_members := CASE WHEN executor_oid IS NULL THEN 1 ELSE 2 END;
    IF (SELECT count(*) FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid
        WHERE granted.rolname='espacigo_db_observer') <> expected_group_members THEN
        RAISE EXCEPTION 'W7 observer group has unexpected additional members';
    END IF;
    IF executor_oid IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        WHERE m.roleid=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_db_observer')
          AND m.member=executor_oid AND m.admin_option AND NOT m.inherit_option AND NOT m.set_option
    ) THEN
        RAISE EXCEPTION 'W7 temporary executor membership differs from its expected ADMIN-only edge';
    END IF;
    IF NOT has_database_privilege('espacigo_gcp04_w7_observer','espacigo','CONNECT')
       OR has_database_privilege('espacigo_gcp04_w7_observer','espacigo','TEMP')
       OR has_database_privilege('espacigo_gcp04_w7_observer','espacigo','CREATE')
       OR NOT has_schema_privilege('espacigo_gcp04_w7_observer','public','USAGE')
       OR has_schema_privilege('espacigo_gcp04_w7_observer','public','CREATE') THEN
        RAISE EXCEPTION 'W7 API login effective database/schema access differs from read-only contract';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND (NOT has_table_privilege('espacigo_gcp04_w7_observer','public.schema_migrations','SELECT')
            OR has_table_privilege('espacigo_gcp04_w7_observer','public.schema_migrations','INSERT')
            OR has_table_privilege('espacigo_gcp04_w7_observer','public.schema_migrations','UPDATE')
            OR has_table_privilege('espacigo_gcp04_w7_observer','public.schema_migrations','DELETE')) THEN
        RAISE EXCEPTION 'W7 API login ledger access is not SELECT-only';
    END IF;
END
$verify$;
COMMIT;
