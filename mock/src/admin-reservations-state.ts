export interface AdminReservationContext { revision:number; accountID:string; token:string; generation:number; criteria:string; }
export class AdminReservationsState {
  revision=0;
  cursor="";
  nextCursor="";
  loading=false;
  criteria="";
  updateCriteria(criteria:string):boolean {
    if(criteria===this.criteria)return false;
    this.criteria=criteria;this.revision++;this.cursor="";this.nextCursor="";this.loading=false;
    return true;
  }
  begin(accountID:string,token:string,generation:number):AdminReservationContext {
    this.loading=true;
    return {revision:++this.revision,accountID,token,generation,criteria:this.criteria};
  }
  current(context:AdminReservationContext,accountID:string,token:string,generation:number,criteria=this.criteria):boolean {
    return context.revision===this.revision&&context.accountID===accountID&&context.token===token&&context.generation===generation&&context.criteria===criteria&&Boolean(token);
  }
  finish(context:AdminReservationContext,accountID:string,token:string,generation:number,criteria=this.criteria):boolean {
    if(!this.current(context,accountID,token,generation,criteria))return false;
    this.loading=false;
    return true;
  }
  clear():void { this.revision++;this.cursor="";this.nextCursor="";this.loading=false;this.criteria=""; }
}
