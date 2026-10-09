export interface AdminReservationContext { revision:number; accountID:string; token:string; generation:number; }
export class AdminReservationsState {
  revision=0;
  cursor="";
  nextCursor="";
  loading=false;
  begin(accountID:string,token:string,generation:number):AdminReservationContext {
    this.loading=true;
    return {revision:++this.revision,accountID,token,generation};
  }
  current(context:AdminReservationContext,accountID:string,token:string,generation:number):boolean {
    return context.revision===this.revision&&context.accountID===accountID&&context.token===token&&context.generation===generation&&Boolean(token);
  }
  finish(context:AdminReservationContext,accountID:string,token:string,generation:number):boolean {
    if(!this.current(context,accountID,token,generation))return false;
    this.loading=false;
    return true;
  }
  clear():void { this.revision++;this.cursor="";this.nextCursor="";this.loading=false; }
}
