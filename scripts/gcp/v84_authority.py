"""V84 dependency ACL contract; deliberately not a cluster-wide permission audit."""

ROLE_FIELDS={'can_login','inherit','superuser','create_role','create_db','replication','bypass_rls'}
EDGES=[{'role':'espacigo_schema_owner','member':'espacigo_migrator_login','admin':False,'inherit':False,'set':True},
       {'role':'espacigo_document_verification_fn','member':'espacigo_schema_owner','admin':False,'inherit':False,'set':True}]
ACL_FIELDS={'object','column','owner','grantee','grantor','privilege','grantable'}


def allowed_acl(a):
    if not isinstance(a,dict) or set(a)!=ACL_FIELDS or type(a['grantable']) is not bool:
        return False
    if any(not isinstance(a[k],str) or not a[k] for k in ['object','owner','grantee','grantor','privilege']) or not isinstance(a['column'],str):
        return False
    obj,col,who,priv=a['object'],a['column'],a['grantee'],a['privilege']
    if who==a['owner']:return True
    if obj.startswith('default:'):return False
    if who=='PUBLIC':
        if a['grantable'] or col!='':return False
        if (obj,priv) in [('public','USAGE'),('database','CONNECT')]:return True
        return obj=='database' and priv=='TEMPORARY' and a['grantor']==a['owner']
    if who not in ['espacigo_migrator_login','espacigo_schema_owner','espacigo_document_verification_fn']:return True
    if a['grantable'] or a['grantor']!=a['owner']:return False
    if obj=='database':return col=='' and ((who=='espacigo_migrator_login' and priv=='CONNECT') or (who=='espacigo_schema_owner' and priv in ['CONNECT','TEMPORARY']))
    if obj=='public':return col=='' and ((who=='espacigo_schema_owner' and priv in ['USAGE','CREATE']) or (who=='espacigo_document_verification_fn' and priv=='USAGE'))
    if obj=='public.usuario':return (who=='espacigo_schema_owner' and priv=='REFERENCES' and col in ['','id']) or (who=='espacigo_document_verification_fn' and ((priv=='SELECT' and col in ['id','estado']) or (priv=='UPDATE' and col=='id')))
    if obj=='public.schema_migrations':return who=='espacigo_schema_owner' and ((priv=='SELECT' and col in ['','version','name','checksum']) or (priv=='INSERT' and col in ['','version','name','checksum','runner_checksum']))
    return False


def validate_catalog(authority):
    edges=authority.get('memberships')
    if not isinstance(edges,list) or len(edges)!=len(EDGES) or any(not isinstance(e,dict) or set(e)!=set(EDGES[0]) or any(type(e[k]) is not bool for k in ['admin','inherit','set']) for e in edges) or sorted(edges,key=lambda e:e['role'])!=sorted(EDGES,key=lambda e:e['role']):
        raise ValueError('V84 memberships differ from exact contract')
    c=authority.get('catalog')
    if not isinstance(c,dict) or set(c)!={'schemas','helper_role','database_owner','database_create','acl'} or c['schemas']!=['pg_catalog','public']:
        raise ValueError('V84 effective search path or catalog evidence differs')
    helper=c['helper_role']
    if not isinstance(helper,dict) or set(helper)!=ROLE_FIELDS or any(v is not False for v in helper.values()):
        raise ValueError('V84 reachable helper role is unsafe')
    if not isinstance(c['database_owner'],str) or not c['database_owner'] or c['database_owner'] in [authority['session_user'],authority['current_user']] or c['database_create'] is not False:
        raise ValueError('V84 database authority is unsafe')
    if authority['public_schema_owner']!='pg_database_owner' or authority['usuario_owner']!=authority['ledger_owner'] or authority['usuario_owner'] not in ['postgres','espacigo_schema_owner']:
        raise ValueError('V84 owners require a reviewed authority profile')
    if not isinstance(c['acl'],list) or not c['acl'] or not all(allowed_acl(a) for a in c['acl']):
        raise ValueError('V84 missing or unexpected ACL evidence')
