package dbbootstrap

import (
	"context"
	"errors"
)

const (
	runtimeACLF5DatabaseName         = "espacigo_f5_gate_20261101"
	runtimeACLF5BaselineServer       = 180006
	runtimeACLF5PublicACLRows  int64 = 5762
	runtimeACLF5PublicACLHash        = "527e02ed8bf413337ae39caf6e4edaa7"
	// Recreated cluster, 2026-11-04: identical PUBLIC/default schema ACLs,
	// different object OIDs. scripts/verify_f5_restore_schema.py proves the
	// archived V82 schema differs only by the two source-pinned V83 changes.
	// Keep the original fingerprint for its historical rollback previews.
	// Evidence: planning/frontend_next/evidence/f5-restored-acl-baseline-20261104.md.
	runtimeACLF5RestoredPublicACLHash       = "37f89420b0338cdd11a978e57a04204c"
	runtimeACLF5DefaultACLRows        int64 = 0
	runtimeACLF5DefaultACLHash              = "d41d8cd98f00b204e9800998ecf8427e"
)

// Historical diagnostics only: never use this OID-sensitive fingerprint as
// the operational gate. The semantic reference below replaces that guard.
// runtimeACLF5PublicBaselineQuery reproduces the complete effective PUBLIC
// ACL fingerprint approved for the F5 target. It does not create temp objects;
// pg_my_temp_schema() is excluded so it also remains safe in caller sessions
// that already have their own temporary namespace.
const runtimeACLF5PublicBaselineQuery = `WITH effective_public_acl AS (
    SELECT 'pg_catalog.pg_database'::regclass::oid AS classid, d.oid AS objid,
           0::smallint AS objsubid, a.grantor, 0::oid AS grantee,
           a.privilege_type::text AS privilege_type, a.is_grantable
      FROM pg_catalog.pg_database d
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba))) a
     WHERE d.datname=pg_catalog.current_database() AND a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_namespace'::regclass::oid, n.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_namespace n
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) a
     WHERE a.grantee=0 AND n.oid<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_class'::regclass::oid, c.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_class c
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault((CASE WHEN c.relkind='S' THEN 's' ELSE 'r' END)::"char",c.relowner))) a
     WHERE a.grantee=0 AND c.relnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_class'::regclass::oid, x.attrelid, x.attnum,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_attribute x
      JOIN pg_catalog.pg_class c ON c.oid=x.attrelid
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(x.attacl,pg_catalog.acldefault('c',c.relowner))) a
     WHERE x.attnum>0 AND NOT x.attisdropped AND a.grantee=0
       AND c.relnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_proc'::regclass::oid, p.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_proc p
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f',p.proowner))) a
     WHERE a.grantee=0 AND p.pronamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_type'::regclass::oid, t.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_type t
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(t.typacl,pg_catalog.acldefault('T',t.typowner))) a
     WHERE a.grantee=0 AND t.typnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_language'::regclass::oid, l.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_language l
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(l.lanacl,pg_catalog.acldefault('l',l.lanowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_foreign_data_wrapper'::regclass::oid, f.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_foreign_data_wrapper f
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(f.fdwacl,pg_catalog.acldefault('F',f.fdwowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_foreign_server'::regclass::oid, s.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_foreign_server s
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(s.srvacl,pg_catalog.acldefault('S',s.srvowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_tablespace'::regclass::oid, s.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_tablespace s
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(s.spcacl,pg_catalog.acldefault('t',s.spcowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_largeobject_metadata'::regclass::oid, l.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_largeobject_metadata l
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(l.lomacl,pg_catalog.acldefault('L',l.lomowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_parameter_acl'::regclass::oid, p.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_parameter_acl p
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.paracl,pg_catalog.acldefault('p',pg_catalog.to_regrole('postgres')::oid))) a
     WHERE a.grantee=0
), public_fingerprint AS (
    SELECT count(*)::bigint AS row_count,
           pg_catalog.md5(COALESCE(pg_catalog.string_agg(
               classid::text || '|' || objid::text || '|' || objsubid::text || '|' ||
               grantor::text || '|' || privilege_type || '|' || is_grantable::text,
               chr(10) ORDER BY classid,objid,objsubid,grantor,privilege_type,is_grantable),'')) AS fingerprint
      FROM effective_public_acl
), default_fingerprint AS (
    SELECT count(*)::bigint AS row_count,
           pg_catalog.md5(COALESCE(pg_catalog.string_agg(
               defaclrole::text || '|' || defaclnamespace::text || '|' || defaclobjtype::text || '|' || COALESCE(defaclacl::text,'NULL'),
               chr(10) ORDER BY defaclrole,defaclnamespace,defaclobjtype),'')) AS fingerprint
      FROM pg_catalog.pg_default_acl
)
SELECT pg_catalog.current_database(), current_setting('server_version_num')::integer,
       public_fingerprint.row_count, public_fingerprint.fingerprint,
       default_fingerprint.row_count, default_fingerprint.fingerprint
  FROM public_fingerprint CROSS JOIN default_fingerprint`

func runtimeACLF5BaselineMatches(database string, serverVersion int, publicRows int64, publicHash string, defaultRows int64, defaultHash string) bool {
	return database == runtimeACLF5DatabaseName && serverVersion == runtimeACLF5BaselineServer &&
		publicRows == runtimeACLF5PublicACLRows &&
		(publicHash == runtimeACLF5PublicACLHash || publicHash == runtimeACLF5RestoredPublicACLHash) &&
		defaultRows == runtimeACLF5DefaultACLRows && defaultHash == runtimeACLF5DefaultACLHash
}

func verifyRuntimeACLF5PublicBaseline(ctx context.Context, conn ScratchPermissionDB) error {
	if conn == nil {
		return errors.New("F5 PUBLIC/default ACL baseline connection is required")
	}
	return verifyRuntimeACLSemanticBaseline(ctx, conn)
}
