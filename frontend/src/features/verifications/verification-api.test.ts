import assert from "node:assert/strict";
import { afterEach, describe, it } from "node:test";
import {
  getIdentityDeclaration,
  saveIdentityDeclaration,
  type IdentityDeclarationPayload,
} from "./verification-api";

const UUID = "b3b6b741-52a9-4e10-8d33-7f96ee3d5142";
const originalFetch = globalThis.fetch;

const jsonResponse = (body: unknown, status = 200): Response =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("verification API contract", () => {
  it("reads absent declarations as null and saves the OpenAPI payload", async () => {
    globalThis.fetch = (async () =>
      new Response(null, { status: 404 })) as typeof fetch;
    assert.equal(await getIdentityDeclaration(), null);

    const payload: IdentityDeclarationPayload = {
      first_name: "Ana",
      last_name: "Pérez",
      date_of_birth: "1990-01-01",
      rut: "12345670-K",
      id_country: "CHL",
      document_type: "ID",
      consent_given: true,
      consent_version: "id-consent-20261108",
    };
    const response = {
      id: UUID,
      revision: 1,
      ...payload,
      consent_given_at: "2026-11-08T10:00:00Z",
    };
    const savedCalls: Array<{ input: RequestInfo | URL; init?: RequestInit }> =
      [];
    globalThis.fetch = (async (
      input: RequestInfo | URL,
      init?: RequestInit,
    ) => {
      savedCalls.push({ input, init });
      return jsonResponse(response);
    }) as typeof fetch;

    assert.deepEqual(await saveIdentityDeclaration(payload), response);
    assert.equal(
      String(savedCalls[0]?.input),
      "/api/backend/verifications/identity-declaration",
    );
    assert.equal(savedCalls[0]?.init?.method, "POST");
    assert.equal(savedCalls[0]?.init?.body, JSON.stringify(payload));
  });
});
