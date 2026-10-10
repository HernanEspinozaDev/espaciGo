import {AdminAuditState, type AdminAuditContext} from "./admin-audit-state.js";

export type AdminAuditExportOutcome =
  | {kind:"stale"}
  | {kind:"unauthorized";code:string}
  | {kind:"forbidden";code:string}
  | {kind:"failed";status:number;code:string}
  | {kind:"download";blob:Blob};

export async function consumeAdminAuditExportResponse(
  response:Response,
  state:AdminAuditState,
  context:AdminAuditContext,
  currentResult:(context:AdminAuditContext)=>boolean,
  currentSession:(context:AdminAuditContext)=>boolean,
  onUnauthorized:()=>void,
  onForbidden:()=>void|Promise<void>,
):Promise<AdminAuditExportOutcome>{
  if(!response.ok){
    let code="unknown";
    try{const body=await response.json() as {error?:{code?:string}};code=body.error?.code??code;}catch{/* response body may be unavailable */}
    if(response.status===401){if(!currentSession(context))return {kind:"stale"};onUnauthorized();return {kind:"unauthorized",code};}
    if(!currentResult(context))return {kind:"stale"};
    if(response.status===403){await onForbidden();return {kind:"forbidden",code};}
    return {kind:"failed",status:response.status,code};
  }
  const blob=await response.blob();
  if(!currentResult(context))return {kind:"stale"};
  return {kind:"download",blob};
}
