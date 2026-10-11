export interface AdminLocalContext { account:string; token:string; generation:number; revision:number; }
export interface AdminLocalSessionContext { account:string; token:string; generation:number; }
export interface AdminLocalSessionSnapshot { roles:string[]; restrictedMode:boolean; }
export function captureAdminLocalContext(account:string,token:string,generation:number,revision:number):AdminLocalContext{return{account,token,generation,revision};}
export function captureAdminLocalSession(context:AdminLocalContext):AdminLocalSessionContext{return{account:context.account,token:context.token,generation:context.generation};}
export function adminLocalSessionMatches(context:AdminLocalSessionContext,current:{account:string;token:string;generation:number}):boolean{return Boolean(context.token&&context.account===current.account&&context.token===current.token&&context.generation===current.generation);}
export function adminLocalContextMatches(context:AdminLocalContext,current:{account:string;token:string;generation:number;revision:number;isAdmin:boolean}):boolean{return Boolean(context.token&&current.isAdmin&&context.account===current.account&&context.token===current.token&&context.generation===current.generation&&context.revision===current.revision);}
export function isAdminLocalForbiddenError(error:unknown):boolean{return error instanceof Error&&/\(HTTP 403(?:,|\))/.test(error.message);}

export async function handleAdminLocalForbidden(
  context:AdminLocalSessionContext,
  current:()=>{account:string;token:string;generation:number;roles:string[];restrictedMode:boolean},
  clearView:(message:string)=>void,
  refreshSession:(token:string)=>Promise<AdminLocalSessionSnapshot>,
  applySession:(snapshot:AdminLocalSessionSnapshot)=>void,
):Promise<"stale"|"refreshed">{
  if(!adminLocalSessionMatches(context,current()))return "stale";
  clearView("Acceso administrativo denegado; se están actualizando los permisos de esta sesión.");
  const currentSession=current();
  applySession({roles:currentSession.roles.filter(role=>role!=="administrador"),restrictedMode:currentSession.restrictedMode});
  if(!adminLocalSessionMatches(context,current()))return "stale";
  let snapshot:AdminLocalSessionSnapshot;
  try{snapshot=await refreshSession(context.token);}catch{
    if(!adminLocalSessionMatches(context,current()))return "stale";
    clearView("No se pudieron actualizar los permisos; la sesión continúa disponible para operaciones permitidas.");
    return "refreshed";
  }
  if(!adminLocalSessionMatches(context,current()))return "stale";
  applySession(snapshot);
  if(!snapshot.roles.includes("administrador")||snapshot.restrictedMode){
    clearView("El acceso administrativo ya no está disponible para esta sesión.");
  }
  return "refreshed";
}
