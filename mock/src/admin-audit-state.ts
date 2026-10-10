export interface AdminAuditContext { revision:number; accountID:string; token:string; generation:number; criteria:string; }
export class AdminAuditState {
  revision=0; cursor=""; nextCursor=""; loading=false; criteria="";
  updateCriteria(value:string):boolean { if(value===this.criteria)return false;this.criteria=value;this.revision++;this.cursor="";this.nextCursor="";this.loading=false;return true; }
  begin(accountID:string,token:string,generation:number):AdminAuditContext { this.loading=true;return {revision:++this.revision,accountID,token,generation,criteria:this.criteria}; }
  current(c:AdminAuditContext,accountID:string,token:string,generation:number,criteria=this.criteria):boolean { return Boolean(token)&&c.revision===this.revision&&c.accountID===accountID&&c.token===token&&c.generation===generation&&c.criteria===criteria; }
  finish(c:AdminAuditContext,accountID:string,token:string,generation:number,criteria=this.criteria):boolean { if(!this.current(c,accountID,token,generation,criteria))return false;this.loading=false;return true; }
  clear():void { this.revision++;this.cursor="";this.nextCursor="";this.loading=false;this.criteria=""; }
}
