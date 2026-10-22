import { routes } from './app.routes';

describe('application routes', () => {
  it('keeps the host panel alias redirect valid and protects its destination', () => {
    const alias = routes.find((route) => route.path === 'panel-anfitrion');
    const destination = routes.find((route) => route.path === 'panel/espacios');

    expect(alias?.redirectTo).toBe('panel/espacios');
    expect(alias?.canActivate).toBeUndefined();
    expect(destination?.canActivate).toBeDefined();
    expect(destination?.data?.['requiredRole']).toBe('arrendador');
  });

  it('serves the private profile and photo controls from the authenticated account route', () => {
    const account = routes.find((route) => route.path === 'mi-cuenta/privacidad');

    expect(account?.loadComponent).toBeDefined();
    expect(account?.canActivate).toBeDefined();
    expect(account?.data?.['requiredRole']).toBeUndefined();
    expect(account?.title).toBe('Privacidad de la cuenta | EspaciGo');
  });
});
