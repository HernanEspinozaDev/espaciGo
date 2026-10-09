import test from "node:test";
import assert from "node:assert/strict";
import { canSendConversation } from "../../internal/mockserver/static/conversation-state.js";

const conversation = state => ({host_id:"host",renter_id:"renter",state});

test("participants may send while signatures are partial or complete", () => {
  for (const state of ["firma_parcial","lista_para_checkin"]) {
    assert.equal(canSendConversation("host",conversation(state)),true,state);
    assert.equal(canSendConversation("renter",conversation(state)),true,state);
  }
});

test("third parties and terminal or cancelled reservations remain read-only", () => {
  for (const state of ["firma_parcial","lista_para_checkin"]) {
    assert.equal(canSendConversation("other",conversation(state)),false,state);
  }
  for (const state of ["cancelada_arrendatario","cancelada_por_firma","rechazada_arrendador","vencida_host"]) {
    assert.equal(canSendConversation("host",conversation(state)),false,state);
    assert.equal(canSendConversation("renter",conversation(state)),false,state);
  }
});
