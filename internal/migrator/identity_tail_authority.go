package migrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	v95IdentityMigration = "V000095__identity_document_case_declaration_binding.sql"
	v96IdentityMigration = "V000096__identity_reviewer_observation_audit.sql"
)

// IdentityTailDDLAuthority is a narrowly scoped read-only snapshot of the
// prerequisites guarded by the identity-tail migrations V95 and V96.
type IdentityTailDDLAuthority struct {
	SessionUser                string            `json:"session_user"`
	CurrentUser                string            `json:"current_user"`
	CurrentSchema              string            `json:"current_schema"`
	SessionRole                V84RoleAttributes `json:"session_role"`
	PublicCreate               bool              `json:"public_create"`
	SchemaOwnerExists          bool              `json:"schema_owner_exists"`
	SchemaOwnerRole            V84RoleAttributes `json:"schema_owner_role"`
	RuntimeExists              bool              `json:"runtime_exists"`
	RuntimeRole                V84RoleAttributes `json:"runtime_role"`
	PrivacyCleanupWorkerExists bool              `json:"privacy_cleanup_worker_exists"`
	PrivacyCleanupWorkerRole   V84RoleAttributes `json:"privacy_cleanup_worker_role"`
	CaseOwner                  string            `json:"case_owner"`
	DeclarationOwner           string            `json:"declaration_owner"`
	EventOwner                 string            `json:"event_owner"`
	PurgeOwner                 string            `json:"purge_owner"`
	PurgeSecurityDefiner       bool              `json:"purge_security_definer"`
	PurgeSearchPath            []string          `json:"purge_search_path"`
	PurgeNonOwnerACLCount      int               `json:"purge_non_owner_acl_count"`
	PurgeWorkerExecute         bool              `json:"purge_worker_execute"`
	PurgeWorkerGrantorOwner    bool              `json:"purge_worker_grantor_owner"`
	PurgeWorkerGrantable       bool              `json:"purge_worker_grantable"`
	RuntimePurgeExecute        bool              `json:"runtime_purge_execute"`
	RuntimeCaseSelect          bool              `json:"runtime_case_select"`
	RuntimeCaseExpiryUpdate    bool              `json:"runtime_case_expiry_update"`
	RuntimeEventInsert         bool              `json:"runtime_event_insert"`
}

func needsIdentityTailDDLAuthority(version int64, name string) bool {
	return (version == 95 && name == v95IdentityMigration) ||
		(version == 96 && name == v96IdentityMigration)
}

func identityTailRoleIsRestricted(role V84RoleAttributes, canLogin bool) bool {
	return role.CanLogin == canLogin && !role.Inherit && !role.Superuser &&
		!role.CreateRole && !role.CreateDB && !role.Replication && !role.BypassRLS
}

func ValidateIdentityTailDDLAuthority(authority IdentityTailDDLAuthority, version int64) error {
	if version != 95 && version != 96 {
		return errors.New("identity-tail DDL authority supports only V95 and V96")
	}
	if authority.SessionUser != "espacigo_migrator_login" || authority.CurrentUser != "espacigo_schema_owner" || authority.CurrentSchema != "public" {
		return fmt.Errorf("V%d requires the pinned migrator/schema-owner identity and public search target", version)
	}
	if !identityTailRoleIsRestricted(authority.SessionRole, true) {
		return fmt.Errorf("V%d migrator login role is missing or unsafe", version)
	}
	if !authority.PublicCreate {
		return fmt.Errorf("V%d preflight lacks effective public CREATE", version)
	}
	if !authority.SchemaOwnerExists || !identityTailRoleIsRestricted(authority.SchemaOwnerRole, false) {
		return fmt.Errorf("V%d schema-owner role is missing or unsafe", version)
	}
	if !authority.RuntimeExists || !identityTailRoleIsRestricted(authority.RuntimeRole, false) {
		return fmt.Errorf("V%d runtime role is missing or unsafe", version)
	}
	if version == 95 {
		if !authority.PrivacyCleanupWorkerExists || !identityTailRoleIsRestricted(authority.PrivacyCleanupWorkerRole, false) {
			return errors.New("V95 privacy cleanup worker role is missing or unsafe")
		}
		if authority.CaseOwner != "espacigo_schema_owner" || authority.DeclarationOwner != "espacigo_schema_owner" || authority.PurgeOwner != "espacigo_schema_owner" {
			return errors.New("V95 requires schema-owned identity tables and purge function")
		}
		if !authority.PurgeSecurityDefiner || len(authority.PurgeSearchPath) != 1 || authority.PurgeSearchPath[0] != "search_path=pg_catalog" {
			return errors.New("V95 purge function definer/search-path contract differs")
		}
		if authority.PurgeNonOwnerACLCount != 1 || !authority.PurgeWorkerExecute || !authority.PurgeWorkerGrantorOwner || authority.PurgeWorkerGrantable || authority.RuntimePurgeExecute {
			return errors.New("V95 requires the reviewed worker-only V92 purge ACL")
		}
		return nil
	}
	if authority.CaseOwner != "espacigo_schema_owner" || authority.EventOwner != "espacigo_schema_owner" {
		return errors.New("V96 requires schema-owned verification tables")
	}
	if !authority.RuntimeCaseSelect || !authority.RuntimeCaseExpiryUpdate || !authority.RuntimeEventInsert {
		return errors.New("V96 requires the reviewed runtime read/write path")
	}
	return nil
}

func readIdentityTailDDLAuthority(ctx context.Context, queryer migrationRowQueryer) (IdentityTailDDLAuthority, error) {
	var authority IdentityTailDDLAuthority
	var sessionRoleJSON, schemaOwnerJSON, runtimeJSON, workerJSON []byte
	err := queryer.QueryRow(ctx, `
		SELECT session_user::text,
		       current_user::text,
		       COALESCE(current_schema(), ''),
	       COALESCE((SELECT pg_catalog.jsonb_build_object(
	                    'can_login',r.rolcanlogin,'inherit',r.rolinherit,
	                    'superuser',r.rolsuper,'create_role',r.rolcreaterole,
	                    'create_db',r.rolcreatedb,'replication',r.rolreplication,
	                    'bypass_rls',r.rolbypassrls)
	                 FROM pg_catalog.pg_roles r WHERE r.rolname=session_user), '{}'::jsonb),
		       pg_catalog.has_schema_privilege(current_user, 'public', 'CREATE'),
		       EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner'),
	       COALESCE((SELECT pg_catalog.jsonb_build_object(
	                    'can_login',r.rolcanlogin,'inherit',r.rolinherit,
	                    'superuser',r.rolsuper,'create_role',r.rolcreaterole,
	                    'create_db',r.rolcreatedb,'replication',r.rolreplication,
	                    'bypass_rls',r.rolbypassrls)
	                 FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_schema_owner'), '{}'::jsonb),
	       EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'),
	       COALESCE((SELECT pg_catalog.jsonb_build_object(
	                    'can_login',r.rolcanlogin,'inherit',r.rolinherit,
	                    'superuser',r.rolsuper,'create_role',r.rolcreaterole,
	                    'create_db',r.rolcreatedb,'replication',r.rolreplication,
	                    'bypass_rls',r.rolbypassrls)
	                 FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_runtime'), '{}'::jsonb),
	       EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker'),
	       COALESCE((SELECT pg_catalog.jsonb_build_object(
	                    'can_login',r.rolcanlogin,'inherit',r.rolinherit,
	                    'superuser',r.rolsuper,'create_role',r.rolcreaterole,
	                    'create_db',r.rolcreatedb,'replication',r.rolreplication,
	                    'bypass_rls',r.rolbypassrls)
	                 FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_privacy_cleanup_worker'), '{}'::jsonb),
	       COALESCE((SELECT pg_catalog.pg_get_userbyid(c.relowner) FROM pg_catalog.pg_class c
	                 WHERE c.oid=pg_catalog.to_regclass('public.document_verification_case')), ''),
	       COALESCE((SELECT pg_catalog.pg_get_userbyid(c.relowner) FROM pg_catalog.pg_class c
	                 WHERE c.oid=pg_catalog.to_regclass('public.identity_declaration_revision')), ''),
	       COALESCE((SELECT pg_catalog.pg_get_userbyid(c.relowner) FROM pg_catalog.pg_class c
	                 WHERE c.oid=pg_catalog.to_regclass('public.document_verification_event')), ''),
	       COALESCE((SELECT pg_catalog.pg_get_userbyid(p.proowner) FROM pg_catalog.pg_proc p
	                 WHERE p.oid=pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()')), ''),
	       COALESCE((SELECT p.prosecdef FROM pg_catalog.pg_proc p
	                 WHERE p.oid=pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()')), false),
	       COALESCE((SELECT p.proconfig FROM pg_catalog.pg_proc p
	                 WHERE p.oid=pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()')), '{}'::text[]),
	       COALESCE((SELECT count(*)::integer FROM pg_catalog.pg_proc p
	                 CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl
	                 WHERE p.oid=pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()')
	                   AND acl.grantee<>p.proowner), 0),
	       EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
	               CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl
	               JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee
	               WHERE p.oid=pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()')
	                 AND r.rolname='espacigo_privacy_cleanup_worker' AND acl.privilege_type='EXECUTE'),
	       EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
	               CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl
	               JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee
	               WHERE p.oid=pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()')
	                 AND r.rolname='espacigo_privacy_cleanup_worker' AND acl.privilege_type='EXECUTE'
	                 AND acl.grantor=p.proowner),
	       EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
	               CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) acl
	               JOIN pg_catalog.pg_roles r ON r.oid=acl.grantee
	               WHERE p.oid=pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()')
	                 AND r.rolname='espacigo_privacy_cleanup_worker' AND acl.privilege_type='EXECUTE' AND acl.is_grantable),
	       CASE WHEN pg_catalog.to_regprocedure('public.purge_expired_identity_declaration_revisions()') IS NOT NULL
	                  AND EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime')
	            THEN pg_catalog.has_function_privilege('espacigo_runtime','public.purge_expired_identity_declaration_revisions()','EXECUTE')
	            ELSE false END,
	       CASE WHEN pg_catalog.to_regclass('public.document_verification_case') IS NOT NULL
	                  AND EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime')
	            THEN pg_catalog.has_table_privilege('espacigo_runtime','public.document_verification_case','SELECT') ELSE false END,
	       CASE WHEN pg_catalog.to_regclass('public.document_verification_case') IS NOT NULL
	                  AND EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime')
	                  AND EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
	                             WHERE a.attrelid=pg_catalog.to_regclass('public.document_verification_case')
	                               AND a.attname='document_expires_at' AND a.attnum>0 AND NOT a.attisdropped)
	            THEN pg_catalog.has_column_privilege('espacigo_runtime','public.document_verification_case','document_expires_at','UPDATE')
	            ELSE false END,
	       CASE WHEN pg_catalog.to_regclass('public.document_verification_event') IS NOT NULL
	                  AND EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime')
	            THEN pg_catalog.has_table_privilege('espacigo_runtime','public.document_verification_event','INSERT') ELSE false END
	`).Scan(
		&authority.SessionUser, &authority.CurrentUser, &authority.CurrentSchema, &sessionRoleJSON, &authority.PublicCreate,
		&authority.SchemaOwnerExists, &schemaOwnerJSON, &authority.RuntimeExists, &runtimeJSON,
		&authority.PrivacyCleanupWorkerExists, &workerJSON,
		&authority.CaseOwner, &authority.DeclarationOwner, &authority.EventOwner, &authority.PurgeOwner,
		&authority.PurgeSecurityDefiner, &authority.PurgeSearchPath, &authority.PurgeNonOwnerACLCount,
		&authority.PurgeWorkerExecute, &authority.PurgeWorkerGrantorOwner, &authority.PurgeWorkerGrantable,
		&authority.RuntimePurgeExecute, &authority.RuntimeCaseSelect, &authority.RuntimeCaseExpiryUpdate, &authority.RuntimeEventInsert,
	)
	if err != nil {
		return IdentityTailDDLAuthority{}, fmt.Errorf("read V95/V96 identity DDL authority: %w", err)
	}
	if err := json.Unmarshal(sessionRoleJSON, &authority.SessionRole); err != nil {
		return IdentityTailDDLAuthority{}, fmt.Errorf("decode V95/V96 migrator login role attributes: %w", err)
	}
	if err := json.Unmarshal(schemaOwnerJSON, &authority.SchemaOwnerRole); err != nil {
		return IdentityTailDDLAuthority{}, fmt.Errorf("decode V95/V96 schema-owner role attributes: %w", err)
	}
	if err := json.Unmarshal(runtimeJSON, &authority.RuntimeRole); err != nil {
		return IdentityTailDDLAuthority{}, fmt.Errorf("decode V95/V96 runtime role attributes: %w", err)
	}
	if err := json.Unmarshal(workerJSON, &authority.PrivacyCleanupWorkerRole); err != nil {
		return IdentityTailDDLAuthority{}, fmt.Errorf("decode V95/V96 privacy-worker role attributes: %w", err)
	}
	if authority.PurgeSearchPath == nil {
		authority.PurgeSearchPath = []string{}
	}
	return authority, nil
}
