package migrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// V84Membership is an edge touching any principal reachable by the migrator.
type V84Membership struct {
	Role    string `json:"role"`
	Member  string `json:"member"`
	Admin   bool   `json:"admin"`
	Inherit bool   `json:"inherit"`
	Set     bool   `json:"set"`
}

// V84ACL preserves semantic identities rather than target-specific catalog OIDs.
type V84ACL struct {
	Object    string `json:"object"`
	Column    string `json:"column"`
	Owner     string `json:"owner"`
	Grantee   string `json:"grantee"`
	Grantor   string `json:"grantor"`
	Privilege string `json:"privilege"`
	Grantable bool   `json:"grantable"`
}

type V84AuthorityCatalog struct {
	Schemas        []string          `json:"schemas"`
	HelperRole     V84RoleAttributes `json:"helper_role"`
	DatabaseOwner  string            `json:"database_owner"`
	DatabaseCreate bool              `json:"database_create"`
	ACL            []V84ACL          `json:"acl"`
}

var v84MembershipContract = []V84Membership{
	{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login", Set: true},
	{Role: "espacigo_document_verification_fn", Member: "espacigo_schema_owner", Set: true},
}

func requireV84Memberships(actual []V84Membership) error {
	got := append([]V84Membership(nil), actual...)
	want := append([]V84Membership(nil), v84MembershipContract...)
	key := func(v V84Membership) string { b, _ := json.Marshal(v); return string(b) }
	sort.Slice(got, func(i, j int) bool { return key(got[i]) < key(got[j]) })
	sort.Slice(want, func(i, j int) bool { return key(want[i]) < key(want[j]) })
	if len(got) != len(want) {
		return errors.New("V84 memberships touching protected roles differ from exact contract")
	}
	for i := range got {
		if got[i] != want[i] {
			return errors.New("V84 memberships touching protected roles differ from exact contract")
		}
	}
	return nil
}

// This is a V84 dependency/exposure contract, NOT a cluster-wide ACL audit.
// Other owners/grant options require a reviewed contract, never automatic repair.
func requireV84Catalog(a V84DDLAuthority) error {
	if err := requireV84Memberships(a.Memberships); err != nil {
		return err
	}
	c := a.Catalog
	if c == nil || len(c.Schemas) != 2 || c.Schemas[0] != "pg_catalog" || c.Schemas[1] != "public" {
		return errors.New("V84 effective search path or catalog evidence differs")
	}
	if c.HelperRole != (V84RoleAttributes{}) || c.DatabaseOwner == "" || c.DatabaseCreate || c.DatabaseOwner == a.SessionUser || c.DatabaseOwner == a.CurrentUser {
		return errors.New("V84 reachable helper or database authority is unsafe")
	}
	if a.PublicSchemaOwner != "pg_database_owner" || a.UsuarioOwner != a.LedgerOwner || (a.UsuarioOwner != "postgres" && a.UsuarioOwner != "espacigo_schema_owner") {
		return errors.New("V84 object owners require a reviewed authority profile")
	}
	if len(c.ACL) == 0 {
		return errors.New("V84 ACL evidence missing")
	}
	for _, acl := range c.ACL {
		if !allowedV84ACL(acl) {
			return fmt.Errorf("V84 unexpected ACL: %s %s %s %s", acl.Object, acl.Column, acl.Grantee, acl.Privilege)
		}
	}
	return nil
}

func allowedV84ACL(a V84ACL) bool {
	if a.Owner == "" || a.Grantor == "" || a.Grantee == "" {
		return false
	}
	if a.Grantee == a.Owner {
		return true
	} // ownership has intrinsic grant authority
	if len(a.Object) >= 8 && a.Object[:8] == "default:" {
		return false
	} // V84 creates only tables: no inherited non-owner defaults
	if a.Grantee == "PUBLIC" {
		if a.Grantable || a.Column != "" {
			return false
		}
		switch a.Object {
		case "public":
			return a.Privilege == "USAGE"
		case "database":
			return a.Privilege == "CONNECT" || (a.Privilege == "TEMPORARY" && a.Grantor == a.Owner)
		default:
			return false
		}
	}
	protected := a.Grantee == "espacigo_migrator_login" || a.Grantee == "espacigo_schema_owner" || a.Grantee == "espacigo_document_verification_fn"
	if !protected {
		return true
	} // recorded, but unrelated runtime/worker ACLs are not redefined by this gate
	if a.Grantable || a.Grantor != a.Owner {
		return false
	}
	switch a.Object {
	case "database":
		return a.Column == "" && ((a.Grantee == "espacigo_migrator_login" && a.Privilege == "CONNECT") || (a.Grantee == "espacigo_schema_owner" && (a.Privilege == "CONNECT" || a.Privilege == "TEMPORARY")))
	case "public":
		return a.Column == "" && ((a.Grantee == "espacigo_schema_owner" && (a.Privilege == "USAGE" || a.Privilege == "CREATE")) || (a.Grantee == "espacigo_document_verification_fn" && a.Privilege == "USAGE"))
	case "public.usuario":
		return (a.Grantee == "espacigo_schema_owner" && a.Privilege == "REFERENCES" && (a.Column == "" || a.Column == "id")) || (a.Grantee == "espacigo_document_verification_fn" && ((a.Privilege == "SELECT" && (a.Column == "id" || a.Column == "estado")) || (a.Privilege == "UPDATE" && a.Column == "id")))
	case "public.schema_migrations":
		return a.Grantee == "espacigo_schema_owner" && ((a.Privilege == "SELECT" && (a.Column == "" || a.Column == "version" || a.Column == "name" || a.Column == "checksum")) || (a.Privilege == "INSERT" && (a.Column == "" || a.Column == "version" || a.Column == "name" || a.Column == "checksum" || a.Column == "runner_checksum")))
	}
	return false
}

func readV84Catalog(ctx context.Context, q migrationRowQueryer, a *V84DDLAuthority) error {
	var catalogJSON, membershipsJSON []byte
	err := q.QueryRow(ctx, `WITH objects AS (
 SELECT 'database'::text AS object, ''::text AS col, d.datdba AS owner, COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba)) AS acl
 FROM pg_catalog.pg_database d WHERE d.datname=current_database()
 UNION ALL
 SELECT 'public','',n.nspowner,COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner)) FROM pg_catalog.pg_namespace n WHERE n.nspname='public'
 UNION ALL
 SELECT 'public.'||c.relname,'',c.relowner,COALESCE(c.relacl,pg_catalog.acldefault('r',c.relowner)) FROM pg_catalog.pg_class c
 WHERE c.oid IN (pg_catalog.to_regclass('public.usuario'),pg_catalog.to_regclass('public.schema_migrations'))
 UNION ALL
 SELECT 'public.'||c.relname,a.attname,c.relowner,CASE WHEN pg_catalog.cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::pg_catalog.aclitem[] END
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid
 WHERE c.oid IN (pg_catalog.to_regclass('public.usuario'),pg_catalog.to_regclass('public.schema_migrations')) AND a.attnum>0 AND NOT a.attisdropped
 UNION ALL
 SELECT 'default:'||COALESCE(n.nspname,'global')||':r','',d.defaclrole,d.defaclacl FROM pg_catalog.pg_default_acl d
 LEFT JOIN pg_catalog.pg_namespace n ON n.oid=d.defaclnamespace
 WHERE d.defaclrole=(SELECT oid FROM pg_catalog.pg_roles WHERE rolname=current_user) AND d.defaclobjtype='r' AND (d.defaclnamespace=0 OR n.nspname='public')
), acl_rows AS (
 SELECT o.object,o.col AS "column",pg_catalog.pg_get_userbyid(o.owner)::text AS owner,
 CASE WHEN x.grantee=0 THEN 'PUBLIC' ELSE pg_catalog.pg_get_userbyid(x.grantee)::text END AS grantee,
 pg_catalog.pg_get_userbyid(x.grantor)::text AS grantor,x.privilege_type AS privilege,x.is_grantable AS grantable
 FROM objects o CROSS JOIN LATERAL pg_catalog.aclexplode(o.acl) x
), memberships AS (
 SELECT r.rolname::text AS role,u.rolname::text AS member,m.admin_option AS admin,m.inherit_option AS inherit,m.set_option AS set
 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid JOIN pg_catalog.pg_roles u ON u.oid=m.member
 WHERE r.rolname IN ('espacigo_migrator_login','espacigo_schema_owner','espacigo_document_verification_fn') OR u.rolname IN ('espacigo_migrator_login','espacigo_schema_owner','espacigo_document_verification_fn')
)
SELECT pg_catalog.jsonb_build_object(
 'schemas',pg_catalog.current_schemas(true),
 'helper_role',(SELECT pg_catalog.jsonb_build_object('can_login',r.rolcanlogin,'inherit',r.rolinherit,'superuser',r.rolsuper,'create_role',r.rolcreaterole,'create_db',r.rolcreatedb,'replication',r.rolreplication,'bypass_rls',r.rolbypassrls) FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_document_verification_fn'),
 'database_owner',(SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_catalog.pg_database WHERE datname=current_database()),
 'database_create',pg_catalog.has_database_privilege(current_user,current_database(),'CREATE'),
 'acl',COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.to_jsonb(a) ORDER BY object,"column",grantee,grantor,privilege,grantable) FROM acl_rows a),'[]'::jsonb)),
 COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.to_jsonb(m) ORDER BY role,member,admin,inherit,"set") FROM memberships m),'[]'::jsonb)
`).Scan(&catalogJSON, &membershipsJSON)
	if err != nil {
		return fmt.Errorf("read V84 authority catalog: %w", err)
	}
	if err := json.Unmarshal(catalogJSON, &a.Catalog); err != nil {
		return fmt.Errorf("decode V84 authority catalog: %w", err)
	}
	if err := json.Unmarshal(membershipsJSON, &a.Memberships); err != nil {
		return fmt.Errorf("decode V84 memberships: %w", err)
	}
	return nil
}
