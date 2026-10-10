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
  current:(context:AdminAuditContext)=>boolean,
  onUnauthorized:()=>void,
  onForbidden:()=>void|Promise<void>,
):Promise<AdminAuditExportOutcome>{
  if(!response.ok){
    let code="unknown";
    try{const body=await response.json() as {error?:{code?:string}};code=body.error?.code??code;}catch{/* response body may be unavailable */}
    if(!current(context))return {kind:"stale"};
    if(response.status===401){onUnauthorized();return {kind:"unauthorized",code};}
    if(response.status===403){await onForbidden();return {kind:"forbidden",code};}
    return {kind:"failed",status:response.status,code};
  }
  const blob=await response.blob();
  if(!current(context))return {kind:"stale"};
  return {kind:"download",blob};
}
