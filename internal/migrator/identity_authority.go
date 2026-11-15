package migrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	v84DiditMigration    = "V000084__didit_identity_verification_schema.sql"
	v85IdentityMigration = "V000085__identity_encrypted_declarations_and_minimal_events.sql"
	v86IdentityMigration = "V000086__identity_declaration_retention_purge.sql"
	v87IdentityMigration = "V000087__identity_declaration_retention_worker.sql"
)

func needsIdentityDDLAuthority(version int64, name string) bool {
	return (version == 84 && name == v84DiditMigration) ||
		(version == 85 && name == v85IdentityMigration) ||
		(version == 86 && name == v86IdentityMigration) ||
		(version == 87 && name == v87IdentityMigration) ||
		(version == 88 && name == "V000088__revoke_runtime_identity_purge_execute.sql")
}

type V84RoleAttributes struct {
	CanLogin    bool `json:"can_login"`
	Inherit     bool `json:"inherit"`
	Superuser   bool `json:"superuser"`
	CreateRole  bool `json:"create_role"`
	CreateDB    bool `json:"create_db"`
	Replication bool `json:"replication"`
	BypassRLS   bool `json:"bypass_rls"`
}

type V84DDLAuthority struct {
	SessionUser         string               `json:"session_user"`
	CurrentUser         string               `json:"current_user"`
	CurrentSchema       string               `json:"current_schema"`
	SessionRole         V84RoleAttributes    `json:"session_role"`
	SchemaOwnerRole     V84RoleAttributes    `json:"schema_owner_role"`
	PublicSchemaOwner   string               `json:"public_schema_owner"`
	PublicUsage         bool                 `json:"public_usage"`
	PublicCreate        bool                 `json:"public_create"`
	UsuarioOwner        string               `json:"usuario_owner"`
	UsuarioIDReferences bool                 `json:"usuario_id_references"`
	LedgerOwner         string               `json:"ledger_owner"`
	LedgerSelect        bool                 `json:"ledger_select"`
	LedgerInsert        bool                 `json:"ledger_insert"`
	Memberships         []V84Membership      `json:"memberships"`
	Catalog             *V84AuthorityCatalog `json:"catalog"`
}

type migrationRowQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func requireV84DDLAuthority(authority V84DDLAuthority) error {
	if authority.SessionUser != "espacigo_migrator_login" || authority.CurrentUser != "espacigo_schema_owner" || authority.CurrentSchema != "public" {
		return errors.New("V84 requires the pinned migrator identity, schema owner role and public search target")
	}
	if !authority.SessionRole.CanLogin || authority.SessionRole.Inherit || authority.SessionRole.Superuser || authority.SessionRole.CreateRole || authority.SessionRole.CreateDB || authority.SessionRole.Replication || authority.SessionRole.BypassRLS {
		return errors.New("V84 migrator login has unsafe role attributes")
	}
	if authority.SchemaOwnerRole.CanLogin || authority.SchemaOwnerRole.Inherit || authority.SchemaOwnerRole.Superuser || authority.SchemaOwnerRole.CreateRole || authority.SchemaOwnerRole.CreateDB || authority.SchemaOwnerRole.Replication || authority.SchemaOwnerRole.BypassRLS {
		return errors.New("V84 schema owner has unsafe role attributes")
	}
	if authority.PublicSchemaOwner == "" || authority.UsuarioOwner == "" || authority.LedgerOwner == "" {
		return errors.New("V84 preflight is missing schema or relation owner evidence")
	}
	if !authority.PublicUsage || !authority.PublicCreate || !authority.UsuarioIDReferences || !authority.LedgerSelect || !authority.LedgerInsert {
		return errors.New("V84 preflight lacks effective schema, REFERENCES or migration-ledger privileges")
	}
	return requireV84Catalog(authority)
}

func requireV86DDLAuthority(authority V84DDLAuthority) error {
	if authority.SessionUser != "espacigo_migrator_login" || authority.CurrentUser != "espacigo_schema_owner" || authority.CurrentSchema != "public" {
		return errors.New("V86 requires the pinned migrator identity, schema owner role and public search target")
	}
	if !authority.SessionRole.CanLogin || authority.SessionRole.Inherit || authority.SessionRole.Superuser || authority.SessionRole.CreateRole || authority.SessionRole.CreateDB || authority.SessionRole.Replication || authority.SessionRole.BypassRLS {
		return errors.New("V86 migrator login has unsafe role attributes")
	}
	if authority.SchemaOwnerRole.CanLogin || authority.SchemaOwnerRole.Inherit || authority.SchemaOwnerRole.Superuser || authority.SchemaOwnerRole.CreateRole || authority.SchemaOwnerRole.CreateDB || authority.SchemaOwnerRole.Replication || authority.SchemaOwnerRole.BypassRLS {
		return errors.New("V86 schema owner has unsafe role attributes")
	}
	if authority.PublicSchemaOwner == "" || authority.UsuarioOwner == "" || authority.LedgerOwner == "" {
		return errors.New("V86 preflight is missing schema or relation owner evidence")
	}
	if !authority.PublicUsage || !authority.PublicCreate || !authority.LedgerSelect || !authority.LedgerInsert {
		return errors.New("V86 preflight lacks effective schema or migration-ledger privileges")
	}
	return requireV84Catalog(authority)
}

func requireV87DDLAuthority(authority V84DDLAuthority) error {
	if authority.SessionUser != "espacigo_migrator_login" || authority.CurrentUser != "espacigo_schema_owner" || authority.CurrentSchema != "public" {
		return errors.New("V87 requires the pinned migrator identity, schema owner role and public search target")
	}
	if !authority.SessionRole.CanLogin || authority.SessionRole.Inherit || authority.SessionRole.Superuser || authority.SessionRole.CreateRole || authority.SessionRole.CreateDB || authority.SessionRole.Replication || authority.SessionRole.BypassRLS {
		return errors.New("V87 migrator login has unsafe role attributes")
	}
	if authority.SchemaOwnerRole.CanLogin || authority.SchemaOwnerRole.Inherit || authority.SchemaOwnerRole.Superuser || authority.SchemaOwnerRole.CreateRole || authority.SchemaOwnerRole.CreateDB || authority.SchemaOwnerRole.Replication || authority.SchemaOwnerRole.BypassRLS {
		return errors.New("V87 schema owner has unsafe role attributes")
	}
	if authority.PublicSchemaOwner == "" || authority.UsuarioOwner == "" || authority.LedgerOwner == "" {
		return errors.New("V87 preflight is missing schema or relation owner evidence")
	}
	if !authority.PublicUsage || !authority.LedgerSelect || !authority.LedgerInsert {
		return errors.New("V87 preflight lacks effective schema usage or migration-ledger privileges")
	}
	return requireV84Catalog(authority)
}

func readV84DDLAuthority(ctx context.Context, queryer migrationRowQueryer) (V84DDLAuthority, error) {
	var authority V84DDLAuthority
	var sessionRoleJSON, schemaOwnerRoleJSON []byte
	err := queryer.QueryRow(ctx, `
		SELECT session_user::text,
		       current_user::text,
		       COALESCE(current_schema(), ''),
		       COALESCE((SELECT pg_catalog.jsonb_build_object(
		                    'can_login', r.rolcanlogin, 'inherit', r.rolinherit,
		                    'superuser', r.rolsuper, 'create_role', r.rolcreaterole,
		                    'create_db', r.rolcreatedb, 'replication', r.rolreplication,
		                    'bypass_rls', r.rolbypassrls)
		                 FROM pg_catalog.pg_roles r WHERE r.rolname=session_user), '{}'::jsonb),
		       COALESCE((SELECT pg_catalog.jsonb_build_object(
		                    'can_login', r.rolcanlogin, 'inherit', r.rolinherit,
		                    'superuser', r.rolsuper, 'create_role', r.rolcreaterole,
		                    'create_db', r.rolcreatedb, 'replication', r.rolreplication,
		                    'bypass_rls', r.rolbypassrls)
		                 FROM pg_catalog.pg_roles r WHERE r.rolname=current_user), '{}'::jsonb),
		       COALESCE((SELECT pg_catalog.pg_get_userbyid(n.nspowner)
		                 FROM pg_catalog.pg_namespace n WHERE n.nspname='public'), ''),
	       pg_catalog.has_schema_privilege(current_user, 'public', 'USAGE'),
	       pg_catalog.has_schema_privilege(current_user, 'public', 'CREATE'),
	       COALESCE((SELECT pg_catalog.pg_get_userbyid(c.relowner)
	                 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
	                 WHERE n.nspname='public' AND c.relname='usuario' AND c.relkind IN ('r','p')), ''),
	       CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
	                         WHERE a.attrelid=pg_catalog.to_regclass('public.usuario')
	                           AND a.attname='id' AND NOT a.attisdropped)
	            THEN pg_catalog.has_column_privilege(current_user, 'public.usuario', 'id', 'REFERENCES')
	            ELSE false END,
	       COALESCE((SELECT pg_catalog.pg_get_userbyid(c.relowner)
	                 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
	                 WHERE n.nspname='public' AND c.relname='schema_migrations' AND c.relkind IN ('r','p')), ''),
	       (pg_catalog.has_table_privilege(current_user, 'public.schema_migrations', 'SELECT') OR
	        (pg_catalog.has_column_privilege(current_user, 'public.schema_migrations', 'version', 'SELECT') AND
	         pg_catalog.has_column_privilege(current_user, 'public.schema_migrations', 'name', 'SELECT') AND
	         pg_catalog.has_column_privilege(current_user, 'public.schema_migrations', 'checksum', 'SELECT'))),
	       (pg_catalog.has_table_privilege(current_user, 'public.schema_migrations', 'INSERT') OR
	        (pg_catalog.has_column_privilege(current_user, 'public.schema_migrations', 'version', 'INSERT') AND
	         pg_catalog.has_column_privilege(current_user, 'public.schema_migrations', 'name', 'INSERT') AND
	         pg_catalog.has_column_privilege(current_user, 'public.schema_migrations', 'checksum', 'INSERT') AND
	         pg_catalog.has_column_privilege(current_user, 'public.schema_migrations', 'runner_checksum', 'INSERT')))
	`).Scan(
		&authority.SessionUser,
		&authority.CurrentUser,
		&authority.CurrentSchema,
		&sessionRoleJSON,
		&schemaOwnerRoleJSON,
		&authority.PublicSchemaOwner,
		&authority.PublicUsage,
		&authority.PublicCreate,
		&authority.UsuarioOwner,
		&authority.UsuarioIDReferences,
		&authority.LedgerOwner,
		&authority.LedgerSelect,
		&authority.LedgerInsert,
	)
	if err != nil {
		return V84DDLAuthority{}, fmt.Errorf("read V84 DDL authority: %w", err)
	}
	if err := json.Unmarshal(sessionRoleJSON, &authority.SessionRole); err != nil {
		return V84DDLAuthority{}, fmt.Errorf("decode V84 migrator role attributes: %w", err)
	}
	if err := json.Unmarshal(schemaOwnerRoleJSON, &authority.SchemaOwnerRole); err != nil {
		return V84DDLAuthority{}, fmt.Errorf("decode V84 schema-owner role attributes: %w", err)
	}
	if err := readV84Catalog(ctx, queryer, &authority); err != nil {
		return V84DDLAuthority{}, err
	}
	return authority, nil
}
