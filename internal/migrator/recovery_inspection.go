package migrator

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

type RecoveryInspectionReport struct {
	Format               string             `json:"format"`
	ReadOnly             bool               `json:"read_only"`
	TransactionIsolation string             `json:"transaction_isolation"`
	Database             string             `json:"database"`
	ServerVersion        string             `json:"server_version"`
	SessionUser          string             `json:"session_user"`
	CurrentUser          string             `json:"current_user"`
	Applied              []HistoryEntry     `json:"applied"`
	Pending              []MigrationSummary `json:"pending"`
	Authority            RecoveryAuthority  `json:"authority"`
}
type RecoveryRole struct {
	Name        string `json:"name"`
	Exists      bool   `json:"exists"`
	Superuser   bool   `json:"superuser"`
	CreateRole  bool   `json:"create_role"`
	CreateDB    bool   `json:"create_db"`
	Replication bool   `json:"replication"`
	BypassRLS   bool   `json:"bypass_rls"`
	Login       bool   `json:"login"`
	Inherit     bool   `json:"inherit"`
}
type RecoveryAuthority struct {
	PublicOwner  string          `json:"public_schema_owner"`
	UsuarioOwner string          `json:"usuario_owner"`
	Roles        []RecoveryRole  `json:"roles"`
	Memberships  json.RawMessage `json:"memberships"`
	Effective    json.RawMessage `json:"effective_privileges"`
	ACL          json.RawMessage `json:"acl_metadata"`
}

var recoveryRoles = []string{"espacigo_migrator_login", "espacigo_schema_owner", "espacigo_runtime", "espacigo_document_verification_fn"}

func InspectRecovery(ctx context.Context, databaseURL, migrationDir, setRole string) (RecoveryInspectionReport, error) {
	c, e := pgx.ParseConfig(databaseURL)
	if e != nil {
		return RecoveryInspectionReport{}, errors.New("invalid recovery inspection connection configuration")
	}
	return inspectRecoveryWithConfig(ctx, c, migrationDir, setRole)
}
func InspectRecoveryCloudSQL(ctx context.Context, databaseURL, password, migrationDir, instance string, privateIP bool, setRole string) (RecoveryInspectionReport, error) {
	c, e := cloudsql.NewPGXConfig(databaseURL, password)
	if e != nil {
		return RecoveryInspectionReport{}, e
	}
	closeConnector, e := cloudsql.ConfigurePGX(ctx, c, instance, privateIP)
	if e != nil {
		return RecoveryInspectionReport{}, e
	}
	defer closeConnector()
	return inspectRecoveryWithConfig(ctx, c, migrationDir, setRole)
}
func inspectRecoveryWithConfig(ctx context.Context, c *pgx.ConnConfig, dir, setRole string) (RecoveryInspectionReport, error) {
	migrations, e := DiscoverMigrations(dir)
	if e != nil {
		return RecoveryInspectionReport{}, e
	}
	conn, e := pgx.ConnectConfig(ctx, c)
	if e != nil {
		return RecoveryInspectionReport{}, errors.New("connect recovery inspection database")
	}
	defer conn.Close(context.Background())
	tx, e := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return RecoveryInspectionReport{}, errors.New("begin repeatable-read read-only recovery inspection")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if setRole != "" {
		if !validDatabaseRoleName(setRole) {
			return RecoveryInspectionReport{}, errors.New("invalid recovery inspection role")
		}
		if _, e = tx.Exec(ctx, "SET ROLE "+pgx.Identifier{setRole}.Sanitize()); e != nil {
			return RecoveryInspectionReport{}, errors.New("could not assume configured recovery inspection role")
		}
	}
	r := RecoveryInspectionReport{Format: "espacigo.recovery-inspection.v1", Applied: []HistoryEntry{}, Pending: []MigrationSummary{}}
	if e = tx.QueryRow(ctx, `SELECT current_database(),current_setting('server_version'),session_user,current_user,current_setting('transaction_read_only')='on',current_setting('transaction_isolation')`).Scan(&r.Database, &r.ServerVersion, &r.SessionUser, &r.CurrentUser, &r.ReadOnly, &r.TransactionIsolation); e != nil {
		return r, errors.New("read recovery inspection identity")
	}
	if !r.ReadOnly || r.TransactionIsolation != "repeatable read" {
		return r, errors.New("recovery inspection transaction is not read-only")
	}
	if setRole == "espacigo_schema_owner" && (r.Database != "espacigo_dms_stage_20261020" || r.SessionUser != "espacigo_migrator_login" || r.CurrentUser != "espacigo_schema_owner") {
		return r, errors.New("recovery inspection database or role identity differs from pinned target")
	}
	var kind string
	if e = tx.QueryRow(ctx, `SELECT COALESCE((SELECT c.relkind::text FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='schema_migrations'),'')`).Scan(&kind); e != nil {
		return r, errors.New("inspect migration ledger relation")
	}
	if kind != "" && kind != "r" {
		return r, errors.New("migration ledger exists but is not a regular table")
	}
	if kind == "r" {
		r.Applied, e = readHistory(ctx, tx)
		if e != nil {
			return r, e
		}
		if e = verifyHistory(migrations, r.Applied); e != nil {
			return r, e
		}
	}
	for _, m := range migrations[len(r.Applied):] {
		r.Pending = append(r.Pending, MigrationSummary{Version: m.Version, Name: m.Name, Checksum: m.Checksum})
	}
	r.Authority, e = readRecoveryAuthority(ctx, tx, kind == "r")
	if e != nil {
		return r, e
	}
	return r, nil
}
func readRecoveryAuthority(ctx context.Context, tx pgx.Tx, ledger bool) (RecoveryAuthority, error) {
	a := RecoveryAuthority{Roles: []RecoveryRole{}, Memberships: json.RawMessage(`[]`), Effective: json.RawMessage(`{}`), ACL: json.RawMessage(`[]`)}
	e := tx.QueryRow(ctx, `SELECT COALESCE((SELECT r.rolname FROM pg_catalog.pg_namespace n JOIN pg_catalog.pg_roles r ON r.oid=n.nspowner WHERE n.nspname='public'),''),COALESCE((SELECT r.rolname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_roles r ON r.oid=c.relowner WHERE n.nspname='public' AND c.relname='usuario'),'')`).Scan(&a.PublicOwner, &a.UsuarioOwner)
	if e != nil {
		return a, errors.New("read recovery schema and usuario owners")
	}
	for _, name := range recoveryRoles {
		var x RecoveryRole
		x.Name = name
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=$1),COALESCE((SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname=$1),false),COALESCE((SELECT rolcreaterole FROM pg_catalog.pg_roles WHERE rolname=$1),false),COALESCE((SELECT rolcreatedb FROM pg_catalog.pg_roles WHERE rolname=$1),false),COALESCE((SELECT rolreplication FROM pg_catalog.pg_roles WHERE rolname=$1),false),COALESCE((SELECT rolbypassrls FROM pg_catalog.pg_roles WHERE rolname=$1),false),COALESCE((SELECT rolcanlogin FROM pg_catalog.pg_roles WHERE rolname=$1),false),COALESCE((SELECT rolinherit FROM pg_catalog.pg_roles WHERE rolname=$1),false)`, name).Scan(&x.Exists, &x.Superuser, &x.CreateRole, &x.CreateDB, &x.Replication, &x.BypassRLS, &x.Login, &x.Inherit)
		if e != nil {
			return a, errors.New("read recovery role attributes")
		}
		a.Roles = append(a.Roles, x)
	}
	var memberships string
	e = tx.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_array(parent.rolname,member.rolname,m.admin_option,m.inherit_option,m.set_option) ORDER BY parent.rolname,member.rolname),'[]'::jsonb)::text FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles parent ON parent.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE parent.rolname=ANY($1::text[]) OR member.rolname=ANY($1::text[])`, recoveryRoles).Scan(&memberships)
	if e != nil {
		return a, errors.New("read protected-role memberships")
	}
	a.Memberships = json.RawMessage(memberships)
	var effective string
	e = tx.QueryRow(ctx, `SELECT jsonb_build_object(
 'session_public_usage',pg_catalog.has_schema_privilege(session_user,'public','USAGE'),
 'schema_owner_public_usage',pg_catalog.has_schema_privilege(current_user,'public','USAGE'),
 'schema_public_create',pg_catalog.has_schema_privilege(current_user,'public','CREATE'),
 'usuario_references_id',CASE WHEN pg_catalog.to_regclass('public.usuario') IS NOT NULL THEN COALESCE(pg_catalog.has_column_privilege(current_user,'public.usuario','id','REFERENCES'),false) ELSE false END,
 'ledger_select',CASE WHEN $1 THEN pg_catalog.has_table_privilege(current_user,'public.schema_migrations','SELECT') ELSE false END,
 'ledger_insert',CASE WHEN $1 THEN pg_catalog.has_table_privilege(current_user,'public.schema_migrations','INSERT') ELSE false END)::text`, ledger).Scan(&effective)
	if e != nil {
		return a, errors.New("read recovery effective privileges")
	}
	a.Effective = json.RawMessage(effective)
	var acl string
	e = tx.QueryRow(ctx, recoveryACLQuery, ledger).Scan(&acl)
	if e != nil {
		return a, errors.New("read bounded recovery ACL metadata")
	}
	a.ACL = json.RawMessage(acl)
	return a, nil
}

const recoveryACLQuery = `SELECT COALESCE(jsonb_agg(item ORDER BY item->>'class',item->>'schema',item->>'object',item->>'column',item->>'grantee',item->>'grantor',item->>'privilege'),'[]'::jsonb)::text FROM (
 SELECT jsonb_build_object('class','relation','schema',n.nspname,'object',c.relname,'owner',pg_catalog.pg_get_userbyid(c.relowner),'acl_source',CASE WHEN c.relacl IS NULL THEN 'implicit-default' WHEN cardinality(c.relacl)=0 THEN 'explicit-empty' ELSE 'explicit' END,'grantee',COALESCE(g.rolname,'PUBLIC'),'grantor',gr.rolname,'privilege',x.privilege_type,'grantable',x.is_grantable) item
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault((CASE c.relkind WHEN 'S' THEN 'S' ELSE 'r' END)::"char",c.relowner))) x LEFT JOIN pg_catalog.pg_roles g ON g.oid=x.grantee JOIN pg_catalog.pg_roles gr ON gr.oid=x.grantor
 WHERE n.nspname IN ('public','document_verification_private') AND c.relkind IN ('r','p','v','m','f','S') AND (c.relname IN ('schema_migrations','usuario','espacio') OR c.relname LIKE 'document_verification_%') AND (c.relname<>'schema_migrations' OR $1)
 UNION ALL SELECT jsonb_build_object('class','column','schema',n.nspname,'object',c.relname,'column',a.attname,'owner',pg_catalog.pg_get_userbyid(c.relowner),'acl_source','explicit-column','grantee',COALESCE(g.rolname,'PUBLIC'),'grantor',gr.rolname,'privilege',x.privilege_type,'grantable',x.is_grantable)
 FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) x LEFT JOIN pg_catalog.pg_roles g ON g.oid=x.grantee JOIN pg_catalog.pg_roles gr ON gr.oid=x.grantor
 WHERE a.attnum>0 AND NOT a.attisdropped AND n.nspname IN ('public','document_verification_private') AND (c.relname IN ('schema_migrations','usuario','espacio') OR c.relname LIKE 'document_verification_%')
 UNION ALL SELECT jsonb_build_object('class','schema','schema',n.nspname,'object',n.nspname,'owner',pg_catalog.pg_get_userbyid(n.nspowner),'acl_source',CASE WHEN n.nspacl IS NULL THEN 'implicit-default' WHEN cardinality(n.nspacl)=0 THEN 'explicit-empty' ELSE 'explicit' END,'grantee',COALESCE(g.rolname,'PUBLIC'),'grantor',gr.rolname,'privilege',x.privilege_type,'grantable',x.is_grantable)
 FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n'::"char",n.nspowner))) x LEFT JOIN pg_catalog.pg_roles g ON g.oid=x.grantee JOIN pg_catalog.pg_roles gr ON gr.oid=x.grantor WHERE n.nspname IN ('public','document_verification_private')
 UNION ALL SELECT jsonb_build_object('class','function','schema',n.nspname,'object',p.oid::regprocedure::text,'owner',pg_catalog.pg_get_userbyid(p.proowner),'acl_source',CASE WHEN p.proacl IS NULL THEN 'implicit-default' WHEN cardinality(p.proacl)=0 THEN 'explicit-empty' ELSE 'explicit' END,'grantee',COALESCE(g.rolname,'PUBLIC'),'grantor',gr.rolname,'privilege',x.privilege_type,'grantable',x.is_grantable)
 FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f'::"char",p.proowner))) x LEFT JOIN pg_catalog.pg_roles g ON g.oid=x.grantee JOIN pg_catalog.pg_roles gr ON gr.oid=x.grantor WHERE n.nspname IN ('public','document_verification_private') AND (p.proname LIKE 'document_verification_%' OR p.proname LIKE 'verify_%')
 UNION ALL SELECT jsonb_build_object('class','default','schema',COALESCE(n.nspname,''),'object',d.defaclobjtype::text,'owner',pg_catalog.pg_get_userbyid(d.defaclrole),'acl_source','explicit-default','grantee',COALESCE(g.rolname,'PUBLIC'),'grantor',gr.rolname,'privilege',x.privilege_type,'grantable',x.is_grantable)
 FROM pg_catalog.pg_default_acl d LEFT JOIN pg_catalog.pg_namespace n ON n.oid=d.defaclnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(d.defaclacl)>0 THEN d.defaclacl ELSE NULL::aclitem[] END) x LEFT JOIN pg_catalog.pg_roles g ON g.oid=x.grantee JOIN pg_catalog.pg_roles gr ON gr.oid=x.grantor WHERE (n.nspname IN ('public','document_verification_private') OR d.defaclnamespace=0) AND d.defaclobjtype IN ('r','S','f','n')
) q`
