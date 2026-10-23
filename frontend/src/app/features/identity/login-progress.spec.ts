import { HttpErrorResponse, provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter, Router } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { IdentityPage } from './identity-page';
function mount(api: object) {
  TestBed.configureTestingModule({
    imports: [IdentityPage],
    providers: [
      ...provideFrontendPorts(),
      provideHttpClient(),
      provideRouter([]),
      { provide: ActivatedRoute, useValue: { snapshot: { data: { identityMode: 'login' } } } },
      { provide: IdentityApi, useValue: api },
      PrivateSessionState,
    ],
  });
  const fixture = TestBed.createComponent(IdentityPage);
  fixture.detectChanges();
  fixture.componentInstance.loginModel.set({
    email: 'fixture@example.invalid',
    password: 'Synthetic#123',
  });
  return fixture;
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}
describe('Login request ownership and progress', () => {
  afterEach(() => TestBed.resetTestingModule());
  it.each([401, 500])('keeps failure %s visible after clearing private state', async (status) => {
    const api = {
      login: vi.fn().mockRejectedValue(new HttpErrorResponse({ status })),
      session: vi.fn(),
    };
    const fixture = mount(api);
    const page = fixture.componentInstance;
    await page.onLogin(new Event('submit') as SubmitEvent);
    fixture.detectChanges();
    expect(page.busy()).toBe(false);
    expect(fixture.nativeElement.querySelector('[role="alert"]')?.textContent).toContain(
      status === 401 ? 'Correo o contraseña incorrectos' : 'El servicio no pudo completar',
    );
    expect(fixture.nativeElement.querySelector('#login-password-error')).toBeNull();
    expect(api.login).toHaveBeenCalledOnce();
    fixture.destroy();
  });
  it('shows a session confirmation failure after login succeeds', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({ account_id: 'fixture', access_token: 'fixture-bearer' }),
      session: vi.fn().mockRejectedValue(new HttpErrorResponse({ status: 500 })),
    };
    const fixture = mount(api);
    const page = fixture.componentInstance;
    const navigate = vi.spyOn(TestBed.inject(Router), 'navigateByUrl');
    await page.onLogin(new Event('submit') as SubmitEvent);
    fixture.detectChanges();
    expect(page.busy()).toBe(false);
    expect(navigate).not.toHaveBeenCalled();
    expect(fixture.nativeElement.querySelector('[role="alert"]')?.textContent).toContain(
      'El servicio no pudo completar',
    );
    expect(fixture.nativeElement.querySelector('#login-password-error')).toBeNull();
    fixture.destroy();
  });

  it('blocks duplicate submission and keeps spinner through login, session and navigation', async () => {
    const login = deferred<unknown>(),
      session = deferred<unknown>(),
      navigation = deferred<boolean>();
    const api = { login: vi.fn(() => login.promise), session: vi.fn(() => session.promise) };
    const fixture = mount(api);
    const page = fixture.componentInstance;
    vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockReturnValue(navigation.promise);
    const pending = page.onLogin(new Event('submit') as SubmitEvent);
    await vi.waitFor(() => expect(api.login).toHaveBeenCalledOnce());
    fixture.detectChanges();
    expect(page.busy()).toBe(true);
    expect(fixture.nativeElement.querySelector('.busy-spinner')).not.toBeNull();
    expect(fixture.nativeElement.querySelector('button[type="submit"]').disabled).toBe(true);
    await page.onLogin(new Event('submit') as SubmitEvent);
    expect(api.login).toHaveBeenCalledOnce();
    login.resolve({ account_id: 'fixture', access_token: 'fixture-bearer' });
    await vi.waitFor(() => expect(api.session).toHaveBeenCalledOnce());
    expect(page.busy()).toBe(true);
    session.resolve({ account_id: 'fixture', roles: [], capabilities: [], restricted_mode: false });
    await vi.waitFor(() => expect(page.loginConfirmed()).toBe(true));
    expect(page.busy()).toBe(true);
    navigation.resolve(true);
    await pending;
    expect(page.busy()).toBe(false);
    fixture.destroy();
  });
  it('uses native password-manager values at submit when input events were omitted', async () => {
    const api = {
      login: vi.fn().mockRejectedValue(new HttpErrorResponse({ status: 401 })),
      session: vi.fn(),
    };
    const fixture = mount(api);
    const page = fixture.componentInstance;
    page.loginModel.set({ email: '', password: '' });
    fixture.detectChanges();
    const form = fixture.nativeElement.querySelector('form') as HTMLFormElement;
    (form.querySelector('#login-email') as HTMLInputElement).value = 'autofill@example.invalid';
    (form.querySelector('#login-password') as HTMLInputElement).value = 'Autofill#123';
    const event = new Event('submit', { cancelable: true });
    Object.defineProperty(event, 'target', { value: form });
    await page.onLogin(event as SubmitEvent);
    expect(api.login).toHaveBeenCalledWith('autofill@example.invalid', 'Autofill#123');
    fixture.destroy();
  });
});
