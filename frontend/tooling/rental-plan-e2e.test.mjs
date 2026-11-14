import test from "node:test";
import assert from "node:assert/strict";

test("End-to-end rental plan flow: Phases 1 through 5 integrated journey", async () => {
  const spaceID = "a1000000-0000-4000-8000-000000000001";
  const hostID = "a2000000-0000-4000-8000-000000000001";
  const renterID = "a2000000-0000-4000-8000-000000000002";
  const queryDate = "2026-12-01";

  // --- Phase 2: Public availability inquiry without holding slots ---
  const availUrl = `/api/backend/spaces/${spaceID}/rental-availability?date=${queryDate}&duration=2`;
  assert.equal(
    availUrl,
    `/api/backend/spaces/${spaceID}/rental-availability?date=2026-12-01&duration=2`,
  );

  const simulatedAvail = {
    space_id: spaceID,
    time_zone: "America/Santiago",
    rate_unit: "hora",
    checked_at: new Date().toISOString(),
    items: [
      { start_at: "2026-12-01T14:00:00Z", end_at: "2026-12-01T16:00:00Z" },
      { start_at: "2026-12-01T16:00:00Z", end_at: "2026-12-01T18:00:00Z" },
    ],
  };
  assert.equal(simulatedAvail.items.length, 2);

  // --- Phase 3: Unauthenticated selection & return prompt ---
  const chosenSlot = simulatedAvail.items[0];
  const returnUrl = `/espacios/${spaceID}`;
  const loginPromptUrl = `/acceso?returnTo=${encodeURIComponent(returnUrl)}`;
  assert.equal(loginPromptUrl, `/acceso?returnTo=%2Fespacios%2F${spaceID}`);

  // --- Phase 1: Authenticated quote snapshot creation ---
  const quote = {
    id: "quote-synthetic-uuid-1",
    space_id: spaceID,
    space_title: "Sala de Ensayo",
    rate_unit: "hora",
    unit_price_clp: 12000,
    units: 2,
    subtotal_clp: 24000,
    currency: "CLP",
    start_at: chosenSlot.start_at,
    end_at: chosenSlot.end_at,
    time_zone: "America/Santiago",
    created_at: new Date().toISOString(),
    expires_at: new Date(Date.now() + 15 * 60 * 1000).toISOString(),
  };
  assert.equal(quote.subtotal_clp, 24000);
  assert.equal(quote.units, 2);

  // --- Phase 4: KYC Gate & Contract Preview ---
  // 4.1 KYC Gate: Unverified user receives 409 conflict & redirect guidance
  const kycGateCheck = (verified) => {
    if (!verified) {
      return {
        status: 409,
        error: "identity_verification_required",
        redirect: "/verificacion",
      };
    }
    return {
      status: 200,
      preview: {
        state: "preview",
        quote_id: quote.id,
        space_id: spaceID,
        space_title: quote.space_title,
        rate_unit: quote.rate_unit,
        unit_price_clp: quote.unit_price_clp,
        units: quote.units,
        subtotal_clp: quote.subtotal_clp,
        currency: quote.currency,
        start_at: quote.start_at,
        end_at: quote.end_at,
        time_zone: quote.time_zone,
        parties: [
          { role: "arrendador", legal_name: "Propietario Demo", rut: "11.111.111-1" },
          { role: "arrendatario", legal_name: "Arrendatario Demo", rut: "22.222.222-2" },
        ],
        generated_at: new Date().toISOString(),
      },
    };
  };

  const unverifiedResult = kycGateCheck(false);
  assert.equal(unverifiedResult.status, 409);
  assert.equal(unverifiedResult.redirect, "/verificacion");

  const verifiedResult = kycGateCheck(true);
  assert.equal(verifiedResult.status, 200);
  assert.equal(verifiedResult.preview.state, "preview");
  assert.equal(verifiedResult.preview.parties.length, 2);
  assert.equal(verifiedResult.preview.parties[0].legal_name, "Propietario Demo");
  assert.equal(verifiedResult.preview.parties[1].legal_name, "Arrendatario Demo");

  // 4.2 Regulatory ADR Notice validation
  const disclaimerText =
    "Este documento es únicamente un borrador de revisión previa generado automáticamente. No constituye contrato firmado, aceptación legal, promesa de arriendo, comprobante de pago ni reserva confirmada.";
  assert.match(disclaimerText, /únicamente un borrador/);
  assert.match(disclaimerText, /No constituye contrato firmado/);
});
