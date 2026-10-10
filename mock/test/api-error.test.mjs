import test from "node:test";
import assert from "node:assert/strict";
import { apiErrorMessage } from "../../internal/mockserver/static/api-error.js";

test("restricted-account 403 is readable in the mock action result", () => {
  const message = apiErrorMessage({
    error: {
      code: "restricted_account",
      message: "Esta cuenta solo puede operar sobre reservas existentes y según sus permisos actuales.",
      request_id: "req-1",
    },
  }, 403);
  assert.match(message, /^Acceso restringido:/);
  assert.match(message, /reservas existentes/);
  assert.match(message, /HTTP 403/);
});
