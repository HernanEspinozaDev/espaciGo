package documentverificationpg

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This opt-in read-only assertion runs after V83 is applied. It verifies that
// the runtime can issue reviewer-grant FOR SHARE reads with only the immutable
// id-column UPDATE privilege required by PostgreSQL's row-locking contract.
func TestPostgresV83ReviewerGrantLockPrivilegeIsMinimal(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	expectedAddress := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_EXPECTED_SERVER_ADDRESS")
	if net.ParseIP(expectedAddress) == nil {
		t.Fatal("ESPACIGO_DOCUMENT_VERIFICATION_EXPECTED_SERVER_ADDRESS must be a separately supplied IP address")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	cc := cfg.ConnConfig
	if cc.Host != "127.0.0.1" || cc.Port != 15434 || cc.Database != "espacigo_f5_gate_20261101" || len(cc.Fallbacks) != 0 {
		t.Fatal("refusing integration target outside the exact loopback F5 endpoint or with fallback hosts")
	}
	if cc.Config.TLSConfig != nil {
		t.Fatal("F5 integration DSN must explicitly disable TLS")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot create F5 pool")
	}
	t.Cleanup(pool.Close)
	var database, sessionUser, currentUser string
	var address net.IP
	if err := pool.QueryRow(ctx, `SELECT current_database(),session_user,current_user,inet_server_addr()`).Scan(&database, &sessionUser, &currentUser, &address); err != nil {
		t.Fatal("cannot verify PostgreSQL target identity")
	}
	if database != "espacigo_f5_gate_20261101" || address == nil || address.String() != expectedAddress || sessionUser == "espacigo_runtime" || currentUser == "espacigo_runtime" {
		t.Fatal("connected PostgreSQL target or setup identity mismatch")
	}
	const migrationName = "V000083__document_verification_lock_and_transfer_fix.sql"
	const migrationChecksum = "405c2f02f8a29e962421a44cbbd07e6e7fae4aa4696483e9d45ab209e82e9333"
	var appliedName, appliedChecksum string
	if err := pool.QueryRow(ctx, `SELECT name,checksum FROM public.schema_migrations WHERE version=83`).Scan(&appliedName, &appliedChecksum); err != nil {
		t.Fatalf("V83 is not recorded as applied; refusing to assert its ACL contract: %v", err)
	}
	if appliedName != migrationName || appliedChecksum != migrationChecksum {
		t.Fatalf("V83 identity mismatch: name=%q checksum=%q", appliedName, appliedChecksum)
	}
	var runtimeExists, runtimeSuperuser, runtimeCanLogin, runtimeInherit bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'),COALESCE((SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'),true),COALESCE((SELECT rolcanlogin FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'),true),COALESCE((SELECT rolinherit FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'),true)`).Scan(&runtimeExists, &runtimeSuperuser, &runtimeCanLogin, &runtimeInherit); err != nil {
		t.Fatal("cannot inspect runtime-role flags")
	}
	if !runtimeExists || runtimeSuperuser || runtimeCanLogin || runtimeInherit {
		t.Fatalf("runtime role is not restricted: exists=%t superuser=%t can_login=%t inherit=%t", runtimeExists, runtimeSuperuser, runtimeCanLogin, runtimeInherit)
	}
	var apiLoginExists, apiLoginCanLogin, apiLoginSuperuser, apiLoginCreateDB, apiLoginCreateRole, apiLoginReplication, apiLoginBypassRLS, apiLoginInherit bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'),COALESCE((SELECT rolcanlogin FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'),false),COALESCE((SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'),true),COALESCE((SELECT rolcreatedb FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'),true),COALESCE((SELECT rolcreaterole FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'),true),COALESCE((SELECT rolreplication FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'),true),COALESCE((SELECT rolbypassrls FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'),true),COALESCE((SELECT rolinherit FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'),true)`).Scan(&apiLoginExists, &apiLoginCanLogin, &apiLoginSuperuser, &apiLoginCreateDB, &apiLoginCreateRole, &apiLoginReplication, &apiLoginBypassRLS, &apiLoginInherit); err != nil {
		t.Fatal("cannot inspect API-login role flags")
	}
	if !apiLoginExists || !apiLoginCanLogin || apiLoginSuperuser || apiLoginCreateDB || apiLoginCreateRole || apiLoginReplication || apiLoginBypassRLS || apiLoginInherit {
		t.Fatalf("API-login role is not restricted: exists=%t can_login=%t superuser=%t createdb=%t createrole=%t replication=%t bypassrls=%t inherit=%t", apiLoginExists, apiLoginCanLogin, apiLoginSuperuser, apiLoginCreateDB, apiLoginCreateRole, apiLoginReplication, apiLoginBypassRLS, apiLoginInherit)
	}
	var tableSelect, idUpdate, tableUpdate bool
	var effectiveUpdateColumns []string
	var directColumnUpdateCount, directIdUpdateCount, directTableUpdateCount int
	var publicColumnUpdateCount, publicTableUpdateCount, runtimeOutgoingMembershipCount int
	var runtimeRelatedMembershipCount, expectedRuntimeMembershipCount int
	var tableOwner, idUpdateGrantor string
	var idUpdateGrantable bool
	if err := pool.QueryRow(ctx, `SELECT
		has_table_privilege('espacigo_runtime','public.document_verification_reviewer_grant','SELECT'),
		has_column_privilege('espacigo_runtime','public.document_verification_reviewer_grant','id','UPDATE'),
		has_table_privilege('espacigo_runtime','public.document_verification_reviewer_grant','UPDATE'),
		(SELECT COALESCE(array_agg(a.attname::text ORDER BY a.attnum),ARRAY[]::text[]) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant' AND pg_catalog.has_column_privilege('espacigo_runtime',c.oid,a.attnum,'UPDATE')),
		(SELECT owner.rolname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant'),
		(SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.member=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime')),
		(SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE granted.rolname IN ('espacigo_runtime','espacigo_api_login') OR member.rolname IN ('espacigo_runtime','espacigo_api_login')),
		(SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member JOIN pg_catalog.pg_roles grantor ON grantor.oid=m.grantor WHERE granted.rolname='espacigo_runtime' AND member.rolname='espacigo_api_login' AND grantor.rolname='postgres' AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option),
		(SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('r',c.relowner))) acl WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant' AND acl.grantee=0 AND acl.privilege_type='UPDATE'),
		(SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(a.attacl,pg_catalog.acldefault('c',c.relowner))) acl WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant' AND acl.grantee=0 AND acl.privilege_type='UPDATE'),
		(SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('r',c.relowner))) acl JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant' AND r.rolname='espacigo_runtime' AND acl.privilege_type='UPDATE'),
		(SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(a.attacl,pg_catalog.acldefault('c',c.relowner))) acl JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant' AND r.rolname='espacigo_runtime' AND acl.privilege_type='UPDATE'),
		(SELECT count(*) FILTER (WHERE a.attname='id') FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(a.attacl,pg_catalog.acldefault('c',c.relowner))) acl JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant' AND r.rolname='espacigo_runtime' AND acl.privilege_type='UPDATE'),
		COALESCE((SELECT bool_or(acl.is_grantable) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(a.attacl,pg_catalog.acldefault('c',c.relowner))) acl JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant' AND a.attname='id' AND r.rolname='espacigo_runtime' AND acl.privilege_type='UPDATE'),false),
		COALESCE((SELECT grantor.rolname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(a.attacl,pg_catalog.acldefault('c',c.relowner))) acl JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee JOIN pg_catalog.pg_roles grantor ON grantor.oid=acl.grantor WHERE n.nspname='public' AND c.relname='document_verification_reviewer_grant' AND a.attname='id' AND r.rolname='espacigo_runtime' AND acl.privilege_type='UPDATE' ORDER BY grantor.rolname LIMIT 1),'')
	`).Scan(&tableSelect, &idUpdate, &tableUpdate, &effectiveUpdateColumns, &tableOwner, &runtimeOutgoingMembershipCount, &runtimeRelatedMembershipCount, &expectedRuntimeMembershipCount, &publicTableUpdateCount, &publicColumnUpdateCount, &directTableUpdateCount, &directColumnUpdateCount, &directIdUpdateCount, &idUpdateGrantable, &idUpdateGrantor); err != nil {
		t.Fatal("cannot inspect reviewer-grant locking privileges")
	}
	if !tableSelect || !idUpdate || tableUpdate || len(effectiveUpdateColumns) != 1 || effectiveUpdateColumns[0] != "id" || tableOwner == "" || runtimeOutgoingMembershipCount != 0 || runtimeRelatedMembershipCount != 1 || expectedRuntimeMembershipCount != 1 || publicTableUpdateCount != 0 || publicColumnUpdateCount != 0 || directTableUpdateCount != 0 || directColumnUpdateCount != 1 || directIdUpdateCount != 1 || idUpdateGrantable || idUpdateGrantor != tableOwner {
		t.Fatalf("unexpected reviewer-grant runtime ACL: select=%t id_update=%t effective_update_columns=%v table_update=%t table_owner=%q runtime_outgoing_memberships=%d runtime_related_memberships=%d expected_runtime_memberships=%d public_table_updates=%d public_column_updates=%d direct_table_updates=%d direct_column_updates=%d direct_id_updates=%d id_update_grantable=%t id_update_grantor=%q", tableSelect, idUpdate, effectiveUpdateColumns, tableUpdate, tableOwner, runtimeOutgoingMembershipCount, runtimeRelatedMembershipCount, expectedRuntimeMembershipCount, publicTableUpdateCount, publicColumnUpdateCount, directTableUpdateCount, directColumnUpdateCount, directIdUpdateCount, idUpdateGrantable, idUpdateGrantor)
	}
}
