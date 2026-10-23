const { test, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');
const docs=require('./fixtures/consent-documents.cjs');
const tokenId='11111111-1111-4111-8111-111111111111';
const token='synthetic-link-token';
async function setup(page,baseURL,registerStatus='email_pending') {
 const writes=[];
 await page.route('**/health/ready',r=>r.fulfill({json:{status:'ready'}}));
 await page.route('**/config.json',r=>r.fulfill({json:{apiBaseUrl:baseURL+'/api/v1',siteOrigin:baseURL}}));
 await page.route('**/api/v1/**',async r=>{
  const url=new URL(r.request().url());
  if(r.request().method()==='POST') writes.push({path:url.pathname,body:r.request().postDataJSON()});
  if(url.pathname.endsWith('/auth/terms'))return r.fulfill({json:{items:docs}});
  if(url.pathname.endsWith('/auth/register'))return r.fulfill({status:201,json:{account_id:tokenId,status:registerStatus}});
  if(url.pathname.endsWith('/auth/verification'))return r.fulfill({status:204,body:''});
  if(url.pathname.endsWith('/auth/verification/reissue'))return r.fulfill({status:202,body:''});
  if(url.pathname.endsWith('/auth/password/recovery/consume'))return r.fulfill({status:204,body:''});
  if(url.pathname.endsWith('/auth/password/recovery'))return r.fulfill({status:202,json:{status:'accepted',message:'Si la cuenta es elegible, recibirás instrucciones.'}});
  return r.fulfill({status:404,body:''});
 });
 return writes;
}
async function capture(page,name) {
 await page.addScriptTag({path:require.resolve('axe-core/axe.min.js')});
 const violations=await page.evaluate(async()=> (await window.axe.run(document)).violations.map(v=>({id:v.id,targets:v.nodes.map(n=>n.target)})));
 expect(violations).toEqual([]);
 const dir=process.env.AUTH_UX_ARTIFACT_DIR;
 if(dir){fs.mkdirSync(dir,{recursive:true});await page.screenshot({path:path.join(dir,name+'.png'),fullPage:true});}
 const {width,viewport}=await page.evaluate(()=>({width:document.documentElement.scrollWidth,viewport:innerWidth}));
 expect(width).toBeLessThanOrEqual(viewport);
}
for(const viewport of [{name:'desktop',width:1440,height:1000},{name:'mobile',width:390,height:844},{name:'compact',width:320,height:800}]){
 test(`registration consent and review-email states ${viewport.name}`,async({page,baseURL})=>{
  await page.setViewportSize(viewport);const writes=await setup(page,baseURL);
  await page.goto('/registro');
  await expect(page.getByRole('heading',{name:/Haz espacio/})).toBeVisible();
  await expect(page.getByRole('radio')).toHaveCount(0);
  await expect(page.getByRole('checkbox')).toHaveCount(2);
  await expect(page.getByRole('checkbox').first()).not.toBeChecked();
  await expect(page.locator('main.identity-page')).not.toContainText(/SHA-256|Go entrega|Versiones sintéticas|Mailpit/);
  await capture(page,'register-'+viewport.name);
  await page.getByRole('button',{name:'Crear cuenta',exact:true}).click();
  expect(writes).toEqual([]);
  await page.locator('#register-email').fill('fixture@example.invalid');
  await page.locator('#register-password').fill('Synthetic#123');
  const toggle=page.getByRole('button',{name:'Mostrar contraseña',exact:true}).first();
  await toggle.click();await expect(page.locator('#register-password')).toHaveAttribute('type','text');
  await page.getByRole('button',{name:'Ocultar contraseña',exact:true}).first().click();await expect(page.locator('#register-password')).toHaveAttribute('type','password');
  await page.locator('#register-confirm').fill('Synthetic#123');
  for(const c of await page.getByRole('checkbox').all())await c.check();
  await page.getByRole('button',{name:'Crear cuenta',exact:true}).click();
  await expect(page.getByRole('heading',{name:'Revisa tu correo',exact:true})).toBeVisible();
  expect(writes[0].body.terms_version_ids).toEqual(docs.map(d=>d.id));
  expect(writes[0].body).not.toHaveProperty('use_preference');
  await expect(page.locator('#register-password')).toHaveCount(0);
  await capture(page,'check-email-'+viewport.name);
 });
}
test('provider failure, resend and normal verification link without manual tokens',async({page,baseURL})=>{
 const writes=await setup(page,baseURL,'verification_pending');
 await page.goto('/registro');
 await page.locator('#register-email').fill('fixture@example.invalid');await page.locator('#register-password').fill('Synthetic#123');await page.locator('#register-confirm').fill('Synthetic#123');
 for(const c of await page.getByRole('checkbox').all())await c.check();
 await page.getByRole('button',{name:'Crear cuenta',exact:true}).click();
 await expect(page.getByText(/no pudimos confirmar el envío/)).toBeVisible();
 await page.getByRole('button',{name:'Reenviar verificación',exact:true}).click();
 expect(writes.at(-1).path).toBe('/api/v1/auth/verification/reissue');
 await page.goto('/verificar-correo#'+new URLSearchParams({token_id:tokenId,token}));
 await expect(page).toHaveURL(baseURL+'/verificar-correo');
 await expect(page.locator('#verify-token')).toHaveCount(0);
 await page.getByRole('button',{name:'Verificar correo',exact:true}).click();
 expect(writes.at(-1).body).toEqual({token_id:tokenId,token});
 await expect(page).toHaveURL(baseURL+'/acceso?verificado=1');
 await expect(page.getByRole('status')).toContainText('Correo verificado');
});
test('recovery link consumes fragment, drops untrusted return and resets through API',async({page,baseURL})=>{
 const writes=await setup(page,baseURL);
 await page.goto('/recuperar-acceso?returnUrl=https%3A%2F%2Funtrusted.invalid#'+new URLSearchParams({token_id:tokenId,token}));
 await expect(page).toHaveURL(baseURL+'/recuperar-acceso');
 await expect(page.getByRole('heading',{name:/Una nueva/})).toBeVisible();
 await expect(page.locator('#recovery-id')).toHaveCount(0);
 await page.locator('#recovery-password').fill('Recovered#234');await page.locator('#recovery-confirm').fill('Recovered#234');
 await capture(page,'recovery-link-desktop');
 await page.getByRole('button',{name:'Guardar nueva contraseña'}).click();
 expect(writes.at(-1).body).toMatchObject({token_id:tokenId,token,new_password:'Recovered#234'});
 await expect(page.getByRole('heading',{name:/Tu contraseña/})).toBeVisible();
 await expect(page.getByRole('link',{name:/Ir a iniciar sesión/})).toBeVisible();
});
test('public documents are readable exact server versions and recovery stays neutral',async({page,baseURL})=>{
 await setup(page,baseURL);
 await page.goto('/terminos-y-condiciones?version='+docs[0].id);
 await expect(page.getByRole('heading',{name:'Términos y condiciones',exact:true})).toBeVisible();
 await expect(page.getByText('Entorno de evaluación de EspaciGo',{exact:true})).toBeVisible();
 await capture(page,'terms-desktop');
 await page.goto('/recuperar-acceso');await page.locator('#recovery-email').fill('unknown@example.invalid');
 await page.getByRole('button',{name:'Enviar enlace de recuperación'}).click();
 await expect(page.getByRole('status')).toContainText('No podemos confirmar la entrega');
});

test('account screen visual and accessibility review desktop and mobile', async({page,baseURL})=>{
 await setup(page,baseURL);
 for(const [device,width,height] of [['desktop',1440,1000],['mobile',390,844]]){
  await page.setViewportSize({width,height});
  for(const [name,url] of [
   ['login','/acceso'],['verify','/verificar-correo#'+new URLSearchParams({token_id:tokenId,token})],
   ['recovery-request','/recuperar-acceso'],['recovery-link','/recuperar-acceso#'+new URLSearchParams({token_id:tokenId,token})],
   ['privacy','/politica-de-privacidad?version='+docs[1].id]
  ]){await page.goto(url);await page.locator('h1').waitFor();await capture(page,name+'-'+device);}
 }
});

test('document outage prevents consent and can be retried without preselecting it', async({page,baseURL})=>{
 const writes=await setup(page,baseURL);let attempts=0;
 await page.route('**/api/v1/auth/terms',r=>{attempts++;return attempts===1?r.fulfill({status:503,json:{error:{code:'terms_unavailable'}}}):r.fulfill({json:{items:docs}})});
 await page.goto('/registro');await expect(page.getByRole('alert')).toContainText('No pudimos cargar los documentos');
 await expect(page.getByRole('button',{name:'Crear cuenta',exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'Volver a intentar',exact:true}).click();
 await expect(page.getByRole('checkbox')).toHaveCount(2);await expect(page.getByRole('checkbox').first()).not.toBeChecked();expect(writes).toEqual([]);
});
test('used or expired verification link offers reissue without revealing a token field',async({page,baseURL})=>{
 await setup(page,baseURL);await page.route('**/api/v1/auth/verification',r=>r.fulfill({status:422,json:{error:{code:'invalid_token'}}}));
 await page.goto('/verificar-correo#'+new URLSearchParams({token_id:tokenId,token}));await page.getByRole('button',{name:'Verificar correo',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('ha vencido o ya se utilizó');
 await expect(page.getByRole('button',{name:'Reenviar verificación',exact:true})).toBeVisible();
 await expect(page.locator('#verify-token')).toHaveCount(0);
});

test('a pending registration disables repeat submission',async({page,baseURL})=>{
 const writes=await setup(page,baseURL);let release;
 const pending=new Promise(resolve=>{release=resolve});let requests=0;
 await page.route('**/api/v1/auth/register',async r=>{requests++;await pending;await r.fulfill({status:201,json:{account_id:tokenId,status:'email_pending'}})});
 await page.goto('/registro');await page.locator('#register-email').fill('fixture@example.invalid');await page.locator('#register-password').fill('Synthetic#123');await page.locator('#register-confirm').fill('Synthetic#123');
 for(const c of await page.getByRole('checkbox').all())await c.check();
 const button=page.getByRole('button',{name:'Crear cuenta',exact:true});await button.click();
 await expect(page.getByRole('button',{name:'Creando tu cuenta…',exact:true})).toBeDisabled();
 await page.locator('button[type=submit]').evaluate(e=>e.click());expect(requests).toBe(1);
 release();await expect(page.getByRole('heading',{name:'Revisa tu correo',exact:true})).toBeVisible();
});

test('safe registration field errors preserve values and focus the failing field', async({page,baseURL})=>{
 await setup(page,baseURL);
 await page.route('**/api/v1/auth/register',r=>r.fulfill({status:422,json:{error:{code:'invalid_value',message:'internal detail must not render',request_id:tokenId,details:[{field:'email',code:'invalid_email',message:'external detail must not render'}]}}}));
 await page.goto('/registro');
 await page.locator('#register-email').fill('fixture@example.invalid');
 await page.locator('#register-password').fill('Synthetic#123');
 await page.locator('#register-confirm').fill('Synthetic#123');
 await page.getByRole('checkbox').nth(0).check(); await page.getByRole('checkbox').nth(1).check();
 await page.getByRole('button',{name:'Crear cuenta',exact:true}).click();
 await expect(page.locator('#register-email-error')).toHaveText('Escribe un correo válido.');
 await expect(page.locator('#register-email')).toHaveValue('fixture@example.invalid');
 await expect(page.locator('#register-email')).toBeFocused();
 await expect(page.locator('main.identity-page')).not.toContainText(/internal detail|external detail/);
 await page.locator('#register-email').fill('corrected@example.invalid');
 await expect(page.locator('#register-email-error')).toHaveCount(0);
});
