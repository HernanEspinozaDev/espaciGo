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
});
