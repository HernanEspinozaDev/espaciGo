-- Execute with psql -X -v ON_ERROR_STOP=1 -f scripts/acl_semantic_snapshot.sql.
-- Entire inspection is read-only; emitted JSON contains semantic names, never OIDs.
BEGIN TRANSACTION READ ONLY;
WITH
-- TOAST relation/index names contain the parent's OID even in textual object
-- identity. Resolve through catalog relationships, never strip name digits.
relation_identities AS (
 SELECT c.oid,
 CASE WHEN parent.oid IS NOT NULL THEN 'TOAST OF '||pi.identity
      WHEN iparent.oid IS NOT NULL THEN 'TOAST INDEX OF '||ipi.identity
      ELSE ci.identity END identity
 FROM pg_catalog.pg_class c
 CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_class'::regclass,c.oid,0) ci
 LEFT JOIN pg_catalog.pg_class parent ON parent.reltoastrelid=c.oid
 LEFT JOIN LATERAL pg_catalog.pg_identify_object('pg_class'::regclass,parent.oid,0) pi ON parent.oid IS NOT NULL
 LEFT JOIN pg_catalog.pg_index ix ON ix.indexrelid=c.oid
 LEFT JOIN pg_catalog.pg_class iparent ON iparent.reltoastrelid=ix.indrelid
 LEFT JOIN LATERAL pg_catalog.pg_identify_object('pg_class'::regclass,iparent.oid,0) ipi ON iparent.oid IS NOT NULL
),
acl_inventory AS (
 SELECT array_agg(c.relname||'.'||a.attname ORDER BY c.relname,a.attnum) v
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid
 WHERE n.nspname='pg_catalog' AND a.attnum>0 AND NOT a.attisdropped
 AND a.atttypid='pg_catalog.aclitem[]'::regtype
),
owner_inventory AS (
 SELECT array_agg(c.relname||'.'||a.attname ORDER BY c.relname,a.attnum) v
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid
 WHERE n.nspname='pg_catalog' AND a.attnum>0 AND NOT a.attisdropped
 AND a.atttypid='pg_catalog.oid'::regtype AND (a.attname LIKE '%owner' OR a.attname='datdba')
),
acl_src(class_name,classid,objid,objsubid,acl,source) AS (
 SELECT 'pg_attribute','pg_class'::regclass,a.attrelid,a.attnum,a.attacl,'effective' FROM pg_catalog.pg_attribute a WHERE a.attnum>0 AND NOT a.attisdropped
 UNION ALL SELECT 'pg_class','pg_class'::regclass,c.oid,0,c.relacl,'effective' FROM pg_catalog.pg_class c
 UNION ALL SELECT 'pg_database','pg_database'::regclass,d.oid,0,d.datacl,'effective' FROM pg_catalog.pg_database d
 UNION ALL SELECT 'pg_default_acl','pg_default_acl'::regclass,d.oid,0,d.defaclacl,'effective' FROM pg_catalog.pg_default_acl d
 UNION ALL SELECT 'pg_foreign_data_wrapper','pg_foreign_data_wrapper'::regclass,f.oid,0,f.fdwacl,'effective' FROM pg_catalog.pg_foreign_data_wrapper f
 UNION ALL SELECT 'pg_foreign_server','pg_foreign_server'::regclass,f.oid,0,f.srvacl,'effective' FROM pg_catalog.pg_foreign_server f
 UNION ALL SELECT 'pg_language','pg_language'::regclass,l.oid,0,l.lanacl,'effective' FROM pg_catalog.pg_language l
 UNION ALL SELECT 'pg_largeobject_metadata','pg_largeobject_metadata'::regclass,l.oid,0,l.lomacl,'effective' FROM pg_catalog.pg_largeobject_metadata l
 UNION ALL SELECT 'pg_namespace','pg_namespace'::regclass,n.oid,0,n.nspacl,'effective' FROM pg_catalog.pg_namespace n
 UNION ALL SELECT 'pg_parameter_acl','pg_parameter_acl'::regclass,p.oid,0,p.paracl,'effective' FROM pg_catalog.pg_parameter_acl p
 UNION ALL SELECT 'pg_proc','pg_proc'::regclass,p.oid,0,p.proacl,'effective' FROM pg_catalog.pg_proc p
 UNION ALL SELECT 'pg_tablespace','pg_tablespace'::regclass,t.oid,0,t.spcacl,'effective' FROM pg_catalog.pg_tablespace t
 UNION ALL SELECT 'pg_type','pg_type'::regclass,t.oid,0,t.typacl,'effective' FROM pg_catalog.pg_type t
 UNION ALL SELECT 'pg_init_privs',i.classoid,i.objoid,i.objsubid,i.initprivs,'initial:'||i.privtype::text FROM pg_catalog.pg_init_privs i
),
acl_named AS (
 SELECT s.*,
 CASE WHEN s.classid='pg_class'::regclass AND s.objsubid=0 THEN i.type||' '||ri.identity
      WHEN s.classid='pg_class'::regclass AND ri.identity LIKE 'TOAST %' THEN 'column '||ri.identity||'.'||quote_ident(a.attname)
      ELSE i.type||' '||i.identity END identity,
 CASE WHEN s.class_name='pg_attribute' THEN a.attname ELSE '' END column_name,
 -- NULL is not an empty ACL: expand built-in owner/PUBLIC defaults with the
 -- pinned server's acldefault, including pg_catalog and extension objects.
 COALESCE(s.acl,CASE s.class_name
 WHEN 'pg_attribute' THEN (SELECT pg_catalog.acldefault('c',o.relowner) FROM pg_catalog.pg_class o WHERE o.oid=s.objid)
 WHEN 'pg_class' THEN (SELECT pg_catalog.acldefault((CASE WHEN o.relkind='S' THEN 's' ELSE 'r' END)::"char",o.relowner) FROM pg_catalog.pg_class o WHERE o.oid=s.objid)
 WHEN 'pg_database' THEN (SELECT pg_catalog.acldefault('d',o.datdba) FROM pg_catalog.pg_database o WHERE o.oid=s.objid)
 WHEN 'pg_foreign_data_wrapper' THEN (SELECT pg_catalog.acldefault('F',o.fdwowner) FROM pg_catalog.pg_foreign_data_wrapper o WHERE o.oid=s.objid)
 WHEN 'pg_foreign_server' THEN (SELECT pg_catalog.acldefault('S',o.srvowner) FROM pg_catalog.pg_foreign_server o WHERE o.oid=s.objid)
 WHEN 'pg_language' THEN (SELECT pg_catalog.acldefault('l',o.lanowner) FROM pg_catalog.pg_language o WHERE o.oid=s.objid)
 WHEN 'pg_largeobject_metadata' THEN (SELECT pg_catalog.acldefault('L',o.lomowner) FROM pg_catalog.pg_largeobject_metadata o WHERE o.oid=s.objid)
 WHEN 'pg_namespace' THEN (SELECT pg_catalog.acldefault('n',o.nspowner) FROM pg_catalog.pg_namespace o WHERE o.oid=s.objid)
 WHEN 'pg_parameter_acl' THEN pg_catalog.acldefault('p','postgres'::regrole)
 WHEN 'pg_proc' THEN (SELECT pg_catalog.acldefault('f',o.proowner) FROM pg_catalog.pg_proc o WHERE o.oid=s.objid)
 WHEN 'pg_tablespace' THEN (SELECT pg_catalog.acldefault('t',o.spcowner) FROM pg_catalog.pg_tablespace o WHERE o.oid=s.objid)
 WHEN 'pg_type' THEN (SELECT pg_catalog.acldefault('T',o.typowner) FROM pg_catalog.pg_type o WHERE o.oid=s.objid)
 END) effective_acl
 FROM acl_src s
 CROSS JOIN LATERAL pg_catalog.pg_identify_object(s.classid,s.objid,s.objsubid) i
 LEFT JOIN relation_identities ri ON s.classid='pg_class'::regclass AND ri.oid=s.objid
 LEFT JOIN pg_catalog.pg_attribute a ON s.classid='pg_class'::regclass AND a.attrelid=s.objid AND a.attnum=s.objsubid
),
acl_states AS (
 SELECT jsonb_agg(jsonb_build_object('class',s.class_name,'identity',s.identity,'column',s.column_name,'source',s.source,
 'state',CASE WHEN s.acl IS NULL THEN 'null' WHEN cardinality(s.acl)=0 THEN 'empty' ELSE 'explicit' END)) v
 FROM acl_named s
),
acl_rows AS (
 SELECT jsonb_agg(jsonb_build_object('class',s.class_name,'identity',s.identity,
 'column',s.column_name,
 'grantee',CASE WHEN x.grantee=0 THEN 'PUBLIC' ELSE gr.rolname END,
 'grantor',go.rolname,'privilege',x.privilege_type,'grantable',x.is_grantable,'source',s.source)
 ORDER BY s.class_name,s.identity,x.grantee,go.rolname,x.privilege_type,x.is_grantable,s.source) v
 FROM acl_named s CROSS JOIN LATERAL pg_catalog.aclexplode(s.effective_acl) x
 LEFT JOIN pg_catalog.pg_roles gr ON gr.oid=x.grantee
 JOIN pg_catalog.pg_roles go ON go.oid=x.grantor
),
owners AS (
 SELECT jsonb_agg(jsonb_build_object('class',class_name,'identity',identity,'owner',owner) ORDER BY class_name,identity,owner) v
 FROM (
 SELECT 'pg_class' class_name,i.identity,r.rolname owner FROM pg_catalog.pg_class o JOIN relation_identities i ON i.oid=o.oid JOIN pg_catalog.pg_roles r ON r.oid=o.relowner
 UNION ALL SELECT 'pg_collation',i.identity,r.rolname FROM pg_catalog.pg_collation o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_collation'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.collowner
 UNION ALL SELECT 'pg_conversion',i.identity,r.rolname FROM pg_catalog.pg_conversion o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_conversion'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.conowner
 UNION ALL SELECT 'pg_database',i.identity,r.rolname FROM pg_catalog.pg_database o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_database'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.datdba
 UNION ALL SELECT 'pg_event_trigger',i.identity,r.rolname FROM pg_catalog.pg_event_trigger o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_event_trigger'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.evtowner
 UNION ALL SELECT 'pg_extension',i.identity,r.rolname FROM pg_catalog.pg_extension o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_extension'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.extowner
 UNION ALL SELECT 'pg_foreign_data_wrapper',i.identity,r.rolname FROM pg_catalog.pg_foreign_data_wrapper o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_foreign_data_wrapper'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.fdwowner
 UNION ALL SELECT 'pg_foreign_server',i.identity,r.rolname FROM pg_catalog.pg_foreign_server o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_foreign_server'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.srvowner
 UNION ALL SELECT 'pg_language',i.identity,r.rolname FROM pg_catalog.pg_language o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_language'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.lanowner
 UNION ALL SELECT 'pg_largeobject_metadata',i.identity,r.rolname FROM pg_catalog.pg_largeobject_metadata o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_largeobject_metadata'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.lomowner
 UNION ALL SELECT 'pg_namespace',i.identity,r.rolname FROM pg_catalog.pg_namespace o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_namespace'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.nspowner
 UNION ALL SELECT 'pg_opclass',i.identity,r.rolname FROM pg_catalog.pg_opclass o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_opclass'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.opcowner
 UNION ALL SELECT 'pg_operator',i.identity,r.rolname FROM pg_catalog.pg_operator o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_operator'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.oprowner
 UNION ALL SELECT 'pg_opfamily',i.identity,r.rolname FROM pg_catalog.pg_opfamily o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_opfamily'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.opfowner
 UNION ALL SELECT 'pg_proc',i.identity,r.rolname FROM pg_catalog.pg_proc o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_proc'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.proowner
 UNION ALL SELECT 'pg_publication',i.identity,r.rolname FROM pg_catalog.pg_publication o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_publication'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.pubowner
 UNION ALL SELECT 'pg_statistic_ext',i.identity,r.rolname FROM pg_catalog.pg_statistic_ext o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_statistic_ext'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.stxowner
 UNION ALL SELECT 'pg_subscription',i.identity,r.rolname FROM pg_catalog.pg_subscription o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_subscription'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.subowner
 UNION ALL SELECT 'pg_tablespace',i.identity,r.rolname FROM pg_catalog.pg_tablespace o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_tablespace'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.spcowner
 UNION ALL SELECT 'pg_ts_config',i.identity,r.rolname FROM pg_catalog.pg_ts_config o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_ts_config'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.cfgowner
 UNION ALL SELECT 'pg_ts_dict',i.identity,r.rolname FROM pg_catalog.pg_ts_dict o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_ts_dict'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.dictowner
 UNION ALL SELECT 'pg_type',i.identity,r.rolname FROM pg_catalog.pg_type o CROSS JOIN LATERAL pg_catalog.pg_identify_object('pg_type'::regclass,o.oid,0)i JOIN pg_catalog.pg_roles r ON r.oid=o.typowner
 ) q(class_name,identity,owner)
),
default_objects AS (
 SELECT jsonb_agg(jsonb_build_object('role',ro.rolname,'schema',COALESCE(ns.nspname,''),'object_type',d.defaclobjtype::text,
 'state',CASE WHEN cardinality(d.defaclacl)=0 THEN 'empty' ELSE 'explicit' END)) v
 FROM pg_catalog.pg_default_acl d JOIN pg_catalog.pg_roles ro ON ro.oid=d.defaclrole
 LEFT JOIN pg_catalog.pg_namespace ns ON ns.oid=d.defaclnamespace
),
default_rows AS (
 SELECT jsonb_agg(jsonb_build_object('role',ro.rolname,'schema',COALESCE(ns.nspname,''),'object_type',d.defaclobjtype::text,
 'grantee',CASE WHEN x.grantee=0 THEN 'PUBLIC' ELSE gr.rolname END,'grantor',go.rolname,
 'privilege',x.privilege_type,'grantable',x.is_grantable)
 ORDER BY ro.rolname,COALESCE(ns.nspname,''),d.defaclobjtype,x.grantee,go.rolname,x.privilege_type,x.is_grantable) v
 FROM pg_catalog.pg_default_acl d JOIN pg_catalog.pg_roles ro ON ro.oid=d.defaclrole
 LEFT JOIN pg_catalog.pg_namespace ns ON ns.oid=d.defaclnamespace
 CROSS JOIN LATERAL pg_catalog.aclexplode(d.defaclacl) x
 LEFT JOIN pg_catalog.pg_roles gr ON gr.oid=x.grantee JOIN pg_catalog.pg_roles go ON go.oid=x.grantor
)
SELECT jsonb_pretty(jsonb_build_object(
 'database',current_database(),'server_version_num',current_setting('server_version_num')::integer,
 'acl_inventory',acl_inventory.v,'owner_inventory',owner_inventory.v,
 'owners',COALESCE(owners.v,'[]'::jsonb),
 'acls',COALESCE((SELECT jsonb_agg(x) FROM acl_rows,jsonb_array_elements(acl_rows.v) x WHERE x->>'source'='effective'),'[]'::jsonb),
 'initial_privileges',COALESCE((SELECT jsonb_agg(x) FROM acl_rows,jsonb_array_elements(acl_rows.v) x WHERE x->>'source' LIKE 'initial:%'),'[]'::jsonb),
 'acl_states',COALESCE(acl_states.v,'[]'::jsonb),
 'default_acl_objects',COALESCE(default_objects.v,'[]'::jsonb),
 'default_acls',COALESCE(default_rows.v,'[]'::jsonb)))
FROM acl_inventory CROSS JOIN owner_inventory CROSS JOIN acl_rows CROSS JOIN owners CROSS JOIN default_rows CROSS JOIN acl_states CROSS JOIN default_objects;
ROLLBACK;
