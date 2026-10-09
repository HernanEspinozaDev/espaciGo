export interface ContractUIState {
  accountID: string;
  reservationState: string;
  reservationStart: number;
  contractState?: string;
  signatures: Array<{ signer_id: string; state: string }>;
  now: number;
}
export interface ContractRequestContext { revision:number; reservationID:string; accountID:string; token:string; generation:number }
export function contractResponseIsCurrent(start:ContractRequestContext,current:ContractRequestContext):boolean {
  return start.revision===current.revision&&start.reservationID===current.reservationID&&start.accountID===current.accountID&&start.token===current.token&&start.generation===current.generation;
}
export function contractActions(state: ContractUIState): { canOpen: boolean; canSign: boolean; canReject: boolean; canDownload: boolean } {
  const own = state.signatures.find(signature => signature.signer_id === state.accountID);
  const beforeStart = state.now < state.reservationStart;
  const actionable = beforeStart && Boolean(own && own.state === "pendiente") && !["firmado", "anulado"].includes(state.contractState ?? "");
  return {
    canOpen: ["aprobada_host", "firma_parcial", "lista_para_checkin"].includes(state.reservationState),
    canSign: actionable,
    canReject: actionable,
    canDownload: state.contractState === "firmado",
  };
}
