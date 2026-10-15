const fs = require('node:fs');
const { test, expect } = require('@playwright/test');

const accounts = JSON.parse(fs.readFileSync(process.env.FE14_ACCOUNT_FILE || '/run/fe14/accounts.json', 'utf8'));

test('GADM-04 checks retirement obligations through the API (consumes only a disposable fixture)', async ({ request }) => {
  const api = process.env.FE14_API_BASE_URL || 'http://backend:8080/api/v1';
  const login = await request.post(`${api}/auth/login`, { data: { email: accounts.globalAdmin.email, password: accounts.globalAdmin.password } });
  expect(login.status()).toBe(200);
  const headers = { Authorization: `Bearer ${(await login.json()).access_token}` };
  const findPublication = async (title) => {
    const query = new URLSearchParams({ search: title, state: 'activa', page_size: '25' });
    const response = await request.get(`${api}/admin/global/publications?${query}`, { headers });
    expect(response.status()).toBe(200);
    const publication = (await response.json()).data.items.find((item) => item.title === title);
    expect(publication, `active publication ${title}`).toBeTruthy();
    return publication;
  };
  const retire = (publication) => request.post(`${api}/admin/global/publications/${publication.publication_id}/retire`, {
    headers,
    data: { owner_id: publication.owner_id, expected_updated_at: publication.updated_at, reason: 'revision_administrativa' },
  });

  const protectedPublication = await findPublication('Retiro con obligación FE-14');
  expect((await retire(protectedPublication)).status()).toBe(409);
  expect((await findPublication('Retiro con obligación FE-14')).state).toBe('activa');

  const allowedPublication = await findPublication('Retiro permitido FE-14');
  expect((await retire(allowedPublication)).status()).toBe(200);
});
