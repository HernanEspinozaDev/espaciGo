export interface DamageClaimRequestContext {
  requestRevision:number;
  selectionRevision:number;
  selectionID:string;
  accountID:string;
  token:string;
  generation:number;
}

export function damageClaimRequestIsCurrent(context:DamageClaimRequestContext,current:DamageClaimRequestContext):boolean{
  return Boolean(context.token)&&context.requestRevision===current.requestRevision&&context.selectionRevision===current.selectionRevision&&context.selectionID===current.selectionID&&context.accountID===current.accountID&&context.token===current.token&&context.generation===current.generation;
}

export async function finishDamageClaimAfterReload(context:DamageClaimRequestContext,reload:()=>Promise<unknown>,current:()=>DamageClaimRequestContext,onSuccess:()=>void):Promise<boolean>{
  await reload();
  if(!damageClaimRequestIsCurrent(context,current()))return false;
  onSuccess();
  return true;
}
