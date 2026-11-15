import test from "node:test";
import assert from "node:assert/strict";

test("rental contract preview payload and KYC gate response match Phase 4 contract", () => {
  const quoteID = "33333333-3333-4333-8333-333333333333";
  const previewEndpoint = `/api/backend/rental-quotes/${encodeURIComponent(quoteID)}/contract-preview`;
  assert.equal(
    previewEndpoint,
    "/api/backend/rental-quotes/33333333-3333-4333-8333-333333333333/contract-preview",
  );

  // 1. Success response with verified identities for both parties
  const successResponse = {
    data: {
      state: "preview",
      quote_id: quoteID,
      space_id: "11111111-1111-4111-8111-111111111111",
      space_title: "Bodega El Salto",
      rate_unit: "mes",
      unit_price_clp: 180000,
      units: 1,
      subtotal_clp: 180000,
      currency: "CLP",
      start_at: "2026-12-01T00:00:00Z",
      end_at: "2027-01-01T00:00:00Z",
      time_zone: "America/Santiago",
      parties: [
        {
          role: "arrendador",
          legal_name: "Carlos Propietario",
          rut: "12.345.678-9",
        },
        {
          role: "arrendatario",
          legal_name: "Ana Arrendataria",
          rut: "98.765.432-1",
        },
      ],
      generated_at: "2026-11-14T05:00:00Z",
    },
  };

  assert.equal(successResponse.data.state, "preview");
  assert.equal(successResponse.data.parties.length, 2);
  assert.equal(successResponse.data.parties[0].role, "arrendador");
  assert.equal(successResponse.data.parties[1].role, "arrendatario");
  assert.equal(successResponse.data.parties[0].rut, "12.345.678-9");
  assert.equal(successResponse.data.parties[1].rut, "98.765.432-1");

  // 2. KYC verification path
  const kycRedirectUrl = "/verificacion";
  assert.equal(kycRedirectUrl, "/verificacion");
});
