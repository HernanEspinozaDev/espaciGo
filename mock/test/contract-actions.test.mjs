import test from "node:test";
import assert from "node:assert/strict";
import { contractActions, contractInviteResponseIsCurrent, contractResponseIsCurrent } from "../../internal/mockserver/static/contract-actions.js";

test("solo el firmante pendiente puede firmar/rechazar antes del inicio", () => {
  const common = { accountID: "host", reservationState: "firma_parcial", reservationStart: 100, now: 99,
    contractState: "firma_parcial", signatures: [{ signer_id: "host", state: "pendiente" }, { signer_id: "renter", state: "firmada" }] };
  assert.deepEqual(contractActions(common), { canOpen: true, canSign: true, canReject: true, canDownload: false });
  assert.equal(contractActions({ ...common, signatures: [{ signer_id: "host", state: "rechazada" }] }).canSign, false);
  assert.equal(contractActions({ ...common, now: 100 }).canSign, false);
});

test("la reserva completamente firmada conserva descarga aunque llegue start_at", () => {
  const state = { accountID: "host", reservationState: "lista_para_checkin", reservationStart: 100, now: 100,
    contractState: "firmado", signatures: [{ signer_id: "host", state: "firmada" }] };
  assert.deepEqual(contractActions(state), { canOpen: true, canSign: false, canReject: false, canDownload: true });
});

test("una reserva cancelada no permite acciones de contrato pendientes",()=>{
  for (const reservationState of ["cancelada_arrendatario","cancelada_por_firma"]) {
    const state={accountID:"host",reservationState,reservationStart:100,now:99,contractState:"anulado",signatures:[{signer_id:"host",state:"pendiente"}]};
    assert.equal(contractActions(state).canSign,false);
    assert.equal(contractActions(state).canReject,false);
  }
});

test("descarta respuesta tardía al cambiar selección, cuenta o sesión",()=>{
  const request={revision:2,reservationID:"reservation-a",accountID:"account-a",token:"session-a1",generation:4};
  assert.equal(contractResponseIsCurrent(request,{...request}),true);
  assert.equal(contractResponseIsCurrent(request,{...request,revision:3,reservationID:"reservation-b"}),false);
  assert.equal(contractResponseIsCurrent(request,{...request,token:"session-a2",generation:5}),false);
  assert.equal(contractResponseIsCurrent(request,{...request,accountID:"account-b",token:"session-b1",generation:5}),false);
});

test("una invitación tardía no reemplaza otra reserva seleccionada",()=>{
  const request={contractID:"contract-a",accountID:"account-a",token:"session-a1",generation:4,selectionRevision:7};
  assert.equal(contractInviteResponseIsCurrent(request,{...request}),true);
  assert.equal(contractInviteResponseIsCurrent(request,{...request,selectionRevision:8}),false);
});

test("una invitación tardía no cambia ni restaura una sesión perdida o renovada",()=>{
  const request={contractID:"contract-a",accountID:"account-a",token:"session-a1",generation:4,selectionRevision:7};
  assert.equal(contractInviteResponseIsCurrent(request,{...request,accountID:"",token:"",generation:5}),false);
  assert.equal(contractInviteResponseIsCurrent(request,{...request,token:"session-a2",generation:5}),false);
  assert.equal(contractInviteResponseIsCurrent(request,{...request,accountID:"account-b",token:"session-b1",generation:5}),false);
});
