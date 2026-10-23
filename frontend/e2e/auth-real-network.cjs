// Invoked by TestAuthRealBrowserLogin against real Go HTTP and disposable PG.
// Fixture mail is captured in Go; bcrypt and registration/verification are real.
const { chromium, expect } = require('@playwright/test');
(async () => {
 const baseURL=process.env.AUTH_BROWSER_BASE_URL;
 if(!baseURL || new URL(baseURL).hostname!=='127.0.0.1') throw Error('loopback required');
 const browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_EXECUTABLE_PATH?{executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH}:{})});
 try {
  for(const autofill of [false,true]) {
   const context=await browser.newContext({baseURL}); const page=await context.newPage();
   let loginWrites=0, sessions=0;
   page.on('request',r=>{const p=new URL(r.url()).pathname;if(p.endsWith('/auth/login'))loginWrites++;if(p.endsWith('/auth/session'))sessions++;});
   await page.goto('/acceso');
   await page.getByRole('button',{name:'Iniciar sesión',exact:true}).click();
   await expect(page.locator('#login-password-error')).toBeVisible(); expect(loginWrites).toBe(0);
   // One failed attempt only, using a fresh disposable account; no real account.
   await page.locator('#login-email').fill('browser-fixture@example.invalid');
   await page.locator('#login-password').fill('IntentionallyWrong#123');
   const failed=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith('/auth/login'));
   await page.getByRole('button',{name:'Iniciar sesión',exact:true}).click();
   expect((await failed).status()).toBe(401);
   await expect(page.getByRole('alert')).toHaveText('Correo o contraseña incorrectos');
   await expect(page.locator('#login-password-error')).toHaveCount(0);
   if(autofill) {
    await page.evaluate(()=>{
     document.querySelector('#login-email').value='browser-fixture@example.invalid';
     document.querySelector('#login-password').value='Synthetic#123';
    });
   } else {
    await page.locator('#login-email').fill('browser-fixture@example.invalid');
    await page.locator('#login-password').fill('Synthetic#123');
    await page.locator('#login-password').blur();
   }
   await page.route('**/api/v1/auth/login', async route => { await new Promise(resolve=>setTimeout(resolve,200)); await route.continue(); });
   const success=page.waitForResponse(r=>new URL(r.url()).pathname.endsWith('/auth/login'));
   await page.getByRole('button',{name:'Iniciar sesión',exact:true}).click();
   await expect(page.locator('.busy-spinner')).toBeVisible();
   await expect(page.getByRole('button',{name:'Iniciando sesión…',exact:true})).toBeDisabled();
   await page.locator('form').first().dispatchEvent('submit');
   expect((await success).status()).toBe(200);
   await expect(page).toHaveURL(/\/mi-cuenta$/); expect(loginWrites).toBe(2); expect(sessions).toBe(1);
   console.log('PASS fresh browser '+(autofill?'native values without input events':'manual input and blur')+' → real 401 → bcrypt login → session → navigation');
   await context.close();
  }
 } finally { await browser.close(); }
})().catch(()=>{console.error('FAIL real browser login assertions (no payload logged)');process.exitCode=1;});
