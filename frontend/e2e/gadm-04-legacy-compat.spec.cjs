const fs = require('node:fs');
const { test, expect } = require('@playwright/test');

const accounts = JSON.parse(fs.readFileSync(process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json', 'utf8'));

test('GADM-04 verifies global-only legacy access, legacy-only access, and outsider denial', async ({ request }) => {
  const api = process.env.FE14_API_BASE_URL || 'http://backend:8080/api/v1';
  const login = async (account) => {
    const response = await request.post(`${api}/auth/login`, { data: { email: account.email, password: account.password } });
    expect(response.status()).toBe(200);
    return { Authorization: `Bearer ${(await response.json()).access_token}` };
  };
  const global = await login(accounts.globalAdmin);
  const legacy = await login(accounts.admin);
  const outsider = await login(accounts.thirdParty);
  const now = new Date();
  const period = new URLSearchParams({
    from: new Date(now.getTime() - 86400000).toISOString(),
    until: new Date(now.getTime() + 60000).toISOString(),
    timezone: 'UTC',
  });
  const globalSelf = await request.get(`${api}/admin/global/me`, { headers: global });
  expect(globalSelf.status()).toBe(200);
  expect((await globalSelf.json()).data.account_id).toBeTruthy();
  expect((await request.get(`${api}/admin/global/me`, { headers: legacy })).status()).toBe(403);
  expect((await request.get(`${api}/admin/global/me`, { headers: outsider })).status()).toBe(403);

  const legacyReadPaths = [
    '/admin/local/published-spaces',
    '/admin/local/reservations',
    `/admin/local/reports/reservations?${period}`,
    `/admin/local/reports/finance?${period}`,
    `/admin/local/reports/disputes?${period}`,
  ];
  for (const path of legacyReadPaths) {
    expect((await request.get(`${api}${path}`, { headers: global })).status(), `global-only ${path}`).toBe(200);
    expect((await request.get(`${api}${path}`, { headers: legacy })).status(), `legacy-only ${path}`).toBe(200);
    expect((await request.get(`${api}${path}`, { headers: outsider })).status(), `outsider ${path}`).toBe(403);
  }
  const target = await request.get(`${api}/admin/global/accounts?email_prefix=${encodeURIComponent(accounts.renter.email)}&page_size=1`, { headers: global });
  expect(target.status()).toBe(200);
  const accountId = (await target.json()).data.items[0].account_id;
  const deniedBlock = await request.post(`${api}/admin/local/accounts/${accountId}/block`, { headers: outsider, data: { reason_code: 'revision_administrativa' } });
  expect(deniedBlock.status()).toBe(403);
  const blocked = await request.post(`${api}/admin/local/accounts/${accountId}/block`, { headers: global, data: { reason_code: 'revision_administrativa' } });
  expect(blocked.status()).toBe(200);
  const unblocked = await request.post(`${api}/admin/local/accounts/${accountId}/unblock`, { headers: legacy, data: { reason_code: 'revision_concluida' } });
  expect(unblocked.status()).toBe(200);
});
