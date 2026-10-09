export interface RentalReservationState {
  host_id:string;renter_id:string;state:string;
}
export interface RentalOperationState {kind:"checkin"|"checkout"|"recepcion"}
export interface DamageClaimState {state:string;defense?:unknown}
export interface RentalOperationControls {
  canCheckIn:boolean;canCheckOut:boolean;canReceive:boolean;canOpenClaim:boolean;canDefend:boolean;
}
export interface RentalOperationContext {revision:number;reservationID:string;accountID:string;token:string;generation:number}
export interface RentalOperationCurrent extends RentalOperationContext {}

export function rentalOperationControls(accountID:string,reservation:RentalReservationState|null,operations:RentalOperationState[],hasClaim:boolean,claim:DamageClaimState|null,hasSession:boolean):RentalOperationControls{
  const kinds=new Set(operations.map(item=>item.kind));
  return {
    canCheckIn:Boolean(hasSession&&reservation&&reservation.renter_id===accountID&&reservation.state==="lista_para_checkin"&&!kinds.has("checkin")),
    canCheckOut:Boolean(hasSession&&reservation&&reservation.renter_id===accountID&&reservation.state==="en_curso"&&!kinds.has("checkout")),
    canReceive:Boolean(hasSession&&reservation&&reservation.host_id===accountID&&reservation.state==="finalizada"&&!kinds.has("recepcion")),
    canOpenClaim:Boolean(hasSession&&reservation&&reservation.host_id===accountID&&reservation.state==="finalizada"&&operations.some(item=>item.kind==="checkout")&&!hasClaim),
    canDefend:Boolean(hasSession&&reservation&&reservation.renter_id===accountID&&reservation.state==="en_disputa"&&hasClaim&&claim?.state==="abierto"&&!claim.defense),
  };
}
export function captureRentalOperationContext(value:RentalOperationCurrent):RentalOperationContext{return {...value};}
export function rentalOperationResponseIsCurrent(context:RentalOperationContext,current:RentalOperationCurrent):boolean{return context.revision===current.revision&&context.reservationID===current.reservationID&&context.accountID===current.accountID&&context.token===current.token&&context.generation===current.generation&&Boolean(context.token);}
export class RentalOperationIdempotencyKeys {
  private values=new Map<string,{key:string;payload:string}>();
  get(reservationID:string,action:string,payload:string,generate:()=>string):string{const id=`${reservationID}:${action}`,prior=this.values.get(id);if(prior?.payload===payload)return prior.key;const next={key:generate(),payload};this.values.set(id,next);return next.key;}
  clear(reservationID:string,action:string):void{this.values.delete(`${reservationID}:${action}`);}
  clearAll():void{this.values.clear();}
}
