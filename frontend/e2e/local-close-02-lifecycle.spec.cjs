const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { test, expect } = require('@playwright/test');

const accounts = JSON.parse(fs.readFileSync(process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json', 'utf8'));
const artifactDir = process.env.FE14_ARTIFACT_DIR || '/run/fe14/artifacts';
fs.mkdirSync(artifactDir, { recursive: true });
const backend = 'http://backend:8080/api/v1';

async function login(page, account) {
  await page.goto('/acceso');
  await page.getByLabel('Correo electrónico').fill(account.email);
  await page.getByLabel('Contraseña').fill(account.password);
  await page.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(page).toHaveURL(/\/mi-cuenta$/);
  await expect(page.getByRole('heading', { name: 'Tu información y tus derechos' })).toBeVisible();
  await expect(page.locator('.role-list')).toContainText(account.roleLabel);
}

async function navigateSpa(page, target) {
  await page.evaluate((url) => {
    history.pushState(null, '', url);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }, target);
}

async function tokenFor(request, account) {
  const response = await request.post(`${backend}/auth/login`, {
    data: { email: account.email, password: account.password },
  });
  expect(response.status()).toBe(200);
  return (await response.json()).access_token;
}

async function bodyData(response) {
  const value = await response.json();
  return value.data ?? value;
}

async function setBusinessClock(phase, now) {
  const requestPath = path.join(artifactDir, `clock-request-${phase}.json`);
  const temporaryPath = `${requestPath}.tmp`;
  fs.writeFileSync(temporaryPath, JSON.stringify({ phase, now }), { mode: 0o600 });
  fs.renameSync(temporaryPath, requestPath);
  const marker = path.join(artifactDir, `clock-done-${phase}`);
  const deadline = Date.now() + 90_000;
  while (!fs.existsSync(marker) && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  expect(fs.existsSync(marker)).toBeTruthy();
  const result = JSON.parse(fs.readFileSync(path.join(artifactDir, `clock-result-${phase}.json`), 'utf8'));
  expect(new Date(result.now).toISOString()).toBe(new Date(now).toISOString());
  return result.now;
}

async function screenshot(page, name) {
  const measurement = await page.evaluate(() => ({
    viewport: { width: innerWidth, height: innerHeight },
    document: { width: document.documentElement.scrollWidth, height: document.documentElement.scrollHeight },
  }));
  measurement.head = process.env.FE14_HEAD || 'unknown';
  expect(measurement.document.width).toBeLessThanOrEqual(measurement.viewport.width);
  fs.writeFileSync(path.join(artifactDir, `${name}.json`), JSON.stringify(measurement, null, 2));
  await page.screenshot({ path: path.join(artifactDir, `${name}.jpg`), type: 'jpeg', quality: 88, fullPage: true });
}

test('LOCAL-CLOSE-02 serves a signed reservation through its financial close', async ({ browser, request }) => {
  test.setTimeout(360_000);
  fs.writeFileSync(path.join(artifactDir, 'local-close-02-clock-started'), 'ready');
  const head = process.env.FE14_HEAD || 'unknown';
  const hostToken = await tokenFor(request, accounts.host);
  const renterToken = await tokenFor(request, accounts.renter);
  const thirdPartyToken = await tokenFor(request, accounts.thirdParty);
  const adminToken = await tokenFor(request, accounts.admin);
  const hostAuth = { Authorization: `Bearer ${hostToken}` };
  const renterAuth = { Authorization: `Bearer ${renterToken}` };
  const thirdPartyAuth = { Authorization: `Bearer ${thirdPartyToken}` };
  const adminAuth = { Authorization: `Bearer ${adminToken}` };

  const host = await browser.newPage({ viewport: { width: 1365, height: 900 }, deviceScaleFactor: 1 });
  await login(host, accounts.host);
  await navigateSpa(host, '/panel/espacios');
  await expect(host.getByRole('heading', { name: 'Mis espacios' })).toBeVisible();
  const space = { id: process.env.FE14_CLOCK_FIXTURE_SPACE_ID };
  expect(space.id).toMatch(/^[0-9a-f-]{36}$/i);
  const fixtureChoice = host.getByRole('button', { name: /Espacio sintético público FE-14/ });
  await expect(fixtureChoice).toBeVisible();
  await fixtureChoice.click();
  await expect(host.getByRole('button', { name: 'Calendario', exact: true })).toBeVisible();

  let renter = await browser.newPage({ viewport: { width: 1365, height: 900 }, deviceScaleFactor: 1 });
  await renter.goto('/explorar');
  await renter.getByRole('link', { name: 'Espacio sintético público FE-14' }).first().click();
  await expect(renter).toHaveURL(new RegExp(`/espacios/${space.id}\\?returnTo=`));
  await expect(renter.getByRole('heading', { name: 'Espacio sintético público FE-14' })).toBeVisible();
  await renter.locator('.booking a.button').click();
  await expect(renter).toHaveURL(/\/acceso\?returnUrl=/);
  await renter.getByLabel('Correo electrónico').fill(accounts.renter.email);
  await renter.getByLabel('Contraseña').fill(accounts.renter.password);
  await renter.getByRole('button', { name: 'Iniciar sesión' }).click();
  await expect(renter).toHaveURL(new RegExp(`/espacios/${space.id}\\?context=returned&returnTo=`));
  await expect(renter.getByText(/Volviste a esta ficha/)).toBeVisible();
  await expect(renter.getByLabel('Horario disponible')).toBeVisible();
  const bookingDate = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000);
  const dateValue = new Intl.DateTimeFormat('en-CA', { timeZone: 'America/Santiago' }).format(bookingDate);
  await renter.getByLabel('Fecha local').fill(dateValue);
  await renter.getByLabel('Fecha local').dispatchEvent('change');
  await expect.poll(() => renter.getByLabel('Horario disponible').locator('option').count()).toBeGreaterThan(1);
  await renter.getByLabel('Horario disponible').selectOption('0');
  const quoteResponsePromise = renter.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith('/api/v1/local/booking-trial/quotes'));
  await renter.getByRole('button', { name: 'Cotizar horario' }).click();
  const quoteResponse = await quoteResponsePromise;
  expect(quoteResponse.status()).toBe(200);
  const quote = await bodyData(quoteResponse);
  expect(quote.time_zone).toBe('America/Santiago');
  const createReservationPromise = renter.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith('/api/v1/local/booking-trial/reservations'));
  await renter.getByRole('button', { name: 'Solicitar reserva' }).click();
  const reservationResponse = await createReservationPromise;
  expect(reservationResponse.status()).toBe(201);
  const reservation = await bodyData(reservationResponse);
  const reservationId = reservation.id;
  expect(reservationId).toMatch(/^[0-9a-f-]{36}$/i);
  await expect(renter).toHaveURL(new RegExp(`/reservas/${reservationId}$`));
  const paymentResponsePromise = renter.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith(`/api/v1/local/booking-trial/reservations/${reservationId}/payment`));
  await renter.getByRole('button', { name: 'Simular pago' }).click();
  expect((await paymentResponsePromise).status()).toBe(200);
  await expect(renter.getByRole('heading', { name: 'Pago fake confirmado; espera decisión' })).toBeVisible();
  const guaranteeAuthorization = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/guarantee`, {
    headers: { ...renterAuth, 'Idempotency-Key': `local-close-02-guarantee-${reservationId}` },
    data: { kind: 'autorizacion', amount_clp: Number(reservation.guarantee_expected_clp ?? 50000), outcome: 'exito' },
  });
  expect(guaranteeAuthorization.status(), await guaranteeAuthorization.text()).toBe(202);

  await navigateSpa(host, `/reservas/${reservationId}`);
  await expect(host.getByRole('heading', { name: 'Pago fake confirmado; espera decisión' })).toBeVisible();
  const approvalResponsePromise = host.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith(`/api/v1/local/booking-trial/reservations/${reservationId}/decision`));
  await host.getByRole('button', { name: 'Aprobar', exact: true }).click();
  const approvalResponse = await approvalResponsePromise;
  expect(approvalResponse.status(), await approvalResponse.text()).toBe(200);
  await expect(host.getByRole('heading', { name: 'Aprobada por anfitrión' })).toBeVisible();
  const contractResponsePromise = host.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith(`/api/v1/local/booking-trial/reservations/${reservationId}/contract`));
  await host.getByRole('button', { name: 'Generar o consultar contrato' }).click();
  const contractResponse = await contractResponsePromise;
  expect(contractResponse.status()).toBe(200);
  const contract = await bodyData(contractResponse);
  const contractId = contract.id;
  expect(contractId).toMatch(/^[0-9a-f-]{36}$/i);
  const hostSignPromise = host.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith(`/api/v1/local/booking-trial/contracts/${contractId}/sign`));
  await host.getByRole('button', { name: 'Firmar versión' }).click();
  expect((await hostSignPromise).status()).toBe(200);

  const renterSignResponse = await request.post(`${backend}/local/booking-trial/contracts/${contractId}/sign`, { headers: renterAuth });
  expect(renterSignResponse.status()).toBe(200);
  expect((await bodyData(renterSignResponse)).state).toBe('firmado');
  await renter.close();
  renter = await browser.newPage({ viewport: { width: 1365, height: 900 }, deviceScaleFactor: 1 });
  await login(renter, accounts.renter);
  const renterDetailResponse = await request.get(`${backend}/local/booking-trial/reservations/${reservationId}`, { headers: renterAuth });
  expect(renterDetailResponse.status()).toBe(200);
  expect((await bodyData(renterDetailResponse)).state).toBe('lista_para_checkin');
  await navigateSpa(renter, `/reservas/${reservationId}`);
  await expect(renter.locator('main h1')).toHaveText('Lista para llegada', { timeout: 60_000 });
  const confirmedContractPromise = renter.waitForResponse((response) => response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith(`/api/v1/local/booking-trial/reservations/${reservationId}/contract`));
  await renter.getByRole('button', { name: 'Generar o consultar contrato' }).click();
  const confirmedContractResponse = await confirmedContractPromise;
  expect(confirmedContractResponse.status()).toBe(200);
  expect((await bodyData(confirmedContractResponse)).state).toBe('firmado');
  const pdfResponsePromise = renter.waitForResponse((response) => response.request().method() === 'GET' && new URL(response.url()).pathname.endsWith(`/api/v1/local/booking-trial/contracts/${contractId}/document`));
  await renter.getByRole('button', { name: 'Descargar PDF privado' }).click();
  const pdfResponse = await pdfResponsePromise;
  expect(pdfResponse.status()).toBe(200);
  expect(pdfResponse.headers()['content-type']).toContain('application/pdf');
  expect((await pdfResponse.body()).subarray(0, 4).toString()).toBe('%PDF');

  await setBusinessClock('first-start', quote.start_at);
  const checkInKey = crypto.randomUUID();
  const checkInPath = `${backend}/local/booking-trial/reservations/${reservationId}/check-in`;
  const checkInBody = { comments: 'Llegada sintética verificada en el cierre local.' };
  const checkInResponse = await request.post(checkInPath, { headers: { ...renterAuth, 'Idempotency-Key': checkInKey }, data: checkInBody });
  expect(checkInResponse.status()).toBe(201);
  const checkInOperation = await bodyData(checkInResponse);
  const checkInReplay = await request.post(checkInPath, { headers: { ...renterAuth, 'Idempotency-Key': checkInKey }, data: checkInBody });
  expect(checkInReplay.status()).toBe(200);
  expect((await bodyData(checkInReplay)).id).toBe(checkInOperation.id);
  const checkInRead = await request.get(`${backend}/local/booking-trial/reservations/${reservationId}`, { headers: renterAuth });
  expect((await bodyData(checkInRead)).state).toBe('en_curso');
  await navigateSpa(renter, '/reservas');
  await renter.getByRole('link', { name: 'Ver solicitud' }).first().click();
  await expect(renter.locator('main h1')).toHaveText('En curso');

  await setBusinessClock('first-end', quote.end_at);
  const checkOutResponse = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/check-out`, { headers: { ...renterAuth, 'Idempotency-Key': crypto.randomUUID() }, data: { comments: 'Salida sintética verificada en el cierre local.' } });
  expect(checkOutResponse.status()).toBe(201);
  const checkOutRead = await request.get(`${backend}/local/booking-trial/reservations/${reservationId}`, { headers: renterAuth });
  expect((await bodyData(checkOutRead)).state).toBe('finalizada');
  await navigateSpa(renter, '/reservas');
  await renter.getByRole('link', { name: 'Ver solicitud' }).first().click();
  await expect(renter.locator('main h1')).toHaveText('Finalizada');
  const receptionResponse = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/reception`, { headers: { ...hostAuth, 'Idempotency-Key': crypto.randomUUID() }, data: { observations: 'Recepción sintética verificada por anfitrión.' } });
  expect(receptionResponse.status()).toBe(201);
  const receptionRead = await request.get(`${backend}/local/booking-trial/reservations/${reservationId}`, { headers: hostAuth });
  expect((await bodyData(receptionRead)).state).toBe('finalizada');

  expect((await request.get(`${backend}/local/booking-trial/reservations/${reservationId}`, { headers: thirdPartyAuth })).status()).toBe(404);
  expect((await request.get(`${backend}/local/booking-trial/reservations/${reservationId}/receipt`, { headers: thirdPartyAuth })).status()).toBe(404);
  expect((await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/settlement`, { headers: { ...renterAuth, 'Idempotency-Key': `local-close-02-renter-${reservationId}` }, data: { outcome: 'exito' } })).status()).toBe(404);
  expect((await request.get(`${backend}/local/booking-trial/reservations/${reservationId}/settlement`, { headers: thirdPartyAuth })).status()).toBe(404);

  const settlePath = `${backend}/local/booking-trial/reservations/${reservationId}/settlement`;
  const settleRequestKeys = [`local-close-02-host-${reservationId}`];
  const before24h = await request.post(settlePath, { headers: { ...hostAuth, 'Idempotency-Key': settleRequestKeys[0] }, data: { outcome: 'exito' } });
  expect(before24h.status()).toBe(409);
  const incidentResponse = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/disputes`, { headers: { ...hostAuth, 'Idempotency-Key': crypto.randomUUID() }, data: { reason_code: 'ensayo_privacidad' } });
  expect([200, 201]).toContain(incidentResponse.status());
  const incident = await bodyData(incidentResponse);
  const claimResponse = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/damage-claim`, { headers: { ...hostAuth, 'Idempotency-Key': crypto.randomUUID() }, data: { description: 'Reclamo sintético por diferencia de entrega, sin bienes ni personas reales.' } });
  expect([200, 201]).toContain(claimResponse.status());
  const claim = await bodyData(claimResponse);
  const claimId = claim.id ?? claim.claim_id;
  expect(claimId).toMatch(/^[0-9a-f-]{36}$/i);

  const defenseBody = { description: 'Descargo sintético: recepción conforme a la reserva.' };
  const defenseResponse = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/damage-claim/defense`, { headers: { ...renterAuth, 'Idempotency-Key': crypto.randomUUID() }, data: defenseBody });
  expect([200, 201]).toContain(defenseResponse.status());
  const defenseEvidenceResponse = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/damage-claim/defense/evidence`, { headers: { ...renterAuth, 'Idempotency-Key': crypto.randomUUID() }, data: {} });
  expect([200, 201]).toContain(defenseEvidenceResponse.status());

  const admin = await browser.newPage({ viewport: { width: 1365, height: 900 }, deviceScaleFactor: 1 });
  await login(admin, accounts.admin);
  await navigateSpa(admin, '/administracion/disputas');
  await admin.getByLabel('Identificador de reserva').fill(reservationId);
  const adminClaimsResponse = await request.get(`${backend}/admin/local/damage-claims`, { headers: adminAuth });
  expect(adminClaimsResponse.status()).toBe(200);
  const resolutionResponse = await request.post(`${backend}/admin/local/damage-claims/${claimId}/resolution`, { headers: { ...adminAuth, 'Idempotency-Key': crypto.randomUUID() }, data: { outcome: 'rechazado', reason_code: 'evidencia_insuficiente' } });
  expect([200, 201]).toContain(resolutionResponse.status());
  const financialDecisionResponse = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/financial-decision`, { headers: { ...adminAuth, 'Idempotency-Key': crypto.randomUUID() }, data: { claim_id: claimId, outcome: 'rechazado', deduction_clp: 0, reason_code: 'sin_deduccion' } });
  expect(financialDecisionResponse.status()).toBe(200);
  const closeIncidentResponse = await request.post(`${backend}/admin/disputes/${incident.id}/close`, { headers: adminAuth, data: { reason_code: 'ensayo_finalizado' } });
  expect(closeIncidentResponse.status()).toBe(200);
  await screenshot(admin, 'fe14_local_close_02_dispute_resolution_desktop');

  const clockAfterDeadline = new Date(new Date(quote.end_at).getTime() + 24 * 60 * 60 * 1000 + 1000).toISOString();
  await setBusinessClock('first-after24', clockAfterDeadline);
  const guaranteeRelease = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/guarantee`, {
    headers: { ...adminAuth, 'Idempotency-Key': `local-close-02-release-${reservationId}` },
    data: { kind: 'liberacion', amount_clp: Number(reservation.guarantee_expected_clp ?? 50000), outcome: 'exito' },
  });
  expect(guaranteeRelease.status()).toBe(202);
  const releaseReconciliation = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/guarantee-reconcile`, {
    headers: adminAuth,
    data: { kind: 'liberacion', outcome: 'exito' },
  });
  expect(releaseReconciliation.status()).toBe(200);
  expect((await bodyData(releaseReconciliation)).state).toBe('confirmada');

  const settlementKey = `local-close-02-settlement-${reservationId}`;
  const settlementResponse = await request.post(settlePath, { headers: { ...hostAuth, 'Idempotency-Key': settlementKey }, data: { outcome: 'sin_respuesta' } });
  expect(settlementResponse.status()).toBe(202);
  const pendingSettlement = await bodyData(settlementResponse);
  expect(pendingSettlement.state).toBe('por_conciliar');

  const reconciliation = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/settlement-reconcile`, { headers: adminAuth, data: { outcome: 'exito' } });
  expect(reconciliation.status()).toBe(200);
  const settled = await bodyData(reconciliation);
  expect(settled.state).toBe('liquidada');
  await admin.getByRole('button', { name: 'Consultar garantía, liquidación y comprobante' }).click();
  await expect(admin.locator('section.card').filter({ has: admin.getByRole('heading', { name: 'Liquidación' }) })).toContainText('liquidada');
  await screenshot(admin, 'fe14_local_close_02_admin_receipt_desktop');

  const replay = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/settlement`, {
    headers: { ...hostAuth, 'Idempotency-Key': settlementKey },
    data: { outcome: 'sin_respuesta' },
  });
  expect(replay.status()).toBe(202);
  const replayed = await bodyData(replay);
  expect(replayed.id).toBe(settled.id);
  expect(replayed.state).toBe('liquidada');
  const repeatedReconcile = await request.post(`${backend}/local/booking-trial/reservations/${reservationId}/settlement-reconcile`, {
    headers: adminAuth,
    data: { outcome: 'exito' },
  });
  expect(repeatedReconcile.status()).toBe(200);
  expect((await bodyData(repeatedReconcile)).id).toBe(settled.id);

  const receiptResponse = await request.get(`${backend}/local/booking-trial/reservations/${reservationId}/receipt`, { headers: renterAuth });
  expect(receiptResponse.status()).toBe(200);
  const receipt = await bodyData(receiptResponse);
  expect(receipt.kind).toBe('comprobante_ensayo');
  expect(receipt.transfer_clp).toBe(settled.transfer_clp);
  expect((await request.get(`${backend}/local/booking-trial/reservations/${reservationId}/receipt`, { headers: adminAuth })).status()).toBe(404);
  expect((await request.get(`${backend}/local/booking-trial/reservations/${reservationId}/receipt`, { headers: thirdPartyAuth })).status()).toBe(404);

  const closedReservationResponse = await request.get(`${backend}/local/booking-trial/reservations/${reservationId}`, { headers: renterAuth });
  expect(closedReservationResponse.status()).toBe(200);
  expect((await bodyData(closedReservationResponse)).state).toBe('cerrada');
  await navigateSpa(renter, '/reservas');
  await renter.getByRole('link', { name: 'Ver solicitud' }).first().click();
  await expect(renter.locator('main')).toContainText('Cerrada');
  await expect(renter.locator('.history')).toContainText('Cerrada');
  await navigateSpa(host, `/reservas/${reservationId}/disputas`);
  await host.getByRole('button', { name: 'Actualizar desde Go' }).click();
  await expect(host.locator('section.card').filter({ has: host.getByRole('heading', { name: 'Liquidación' }) })).toContainText('liquidada');
  await expect(host.locator('section.card').filter({ has: host.getByRole('heading', { name: 'Comprobante de ensayo' }) })).toContainText('comprobante_ensayo');
  await screenshot(host, 'fe14_local_close_02_closed_finance_desktop');

  const evidence = {
    head,
    reservation_id: reservationId,
    space_id: space.id,
    contract_id: contractId,
    claim_id: claimId,
    quote: { start_at: quote.start_at, end_at: quote.end_at, time_zone: quote.time_zone, subtotal_clp: quote.subtotal_clp },
    checkout_at: quote.end_at,
    settlement_id: settled.id,
    settlement_state: settled.state,
    gross_clp: settled.gross_rent_clp,
    refunds_clp: settled.refunds_clp,
    commission_clp: settled.commission_clp,
    transfer_clp: settled.transfer_clp,
    receipt_id: receipt.receipt_id,
    clock_after_24h: clockAfterDeadline,
    settlement_idempotency_key_sha256: crypto.createHash('sha256').update(settleRequestKeys[0]).digest('hex'),
  };
  fs.writeFileSync(path.join(artifactDir, 'local-close-02-settlement-result.json'), JSON.stringify(evidence, null, 2));
  await host.close();
  await renter.close();
  await admin.close();
});
