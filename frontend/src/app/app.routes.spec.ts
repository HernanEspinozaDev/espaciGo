import { routes } from './app.routes';
import { identityGuard, roleGuard } from './core/identity/identity.guard';

const routeAccess = {
  'terminos-y-condiciones': 'public',
  'politica-de-privacidad': 'public',
  '': 'public',
  categorias: 'public',
  'como-funciona': 'public',
  explorar: 'public',
  'espacios/:id': 'public',
  'panel-anfitrion': 'redirect',
  'panel/espacios': 'role',
  'panel/espacios/:spaceId/editar': 'role',
  'panel/espacios/:spaceId/tarifas': 'role',
  'panel/espacios/:spaceId/calendario': 'role',
  'panel/espacios/:spaceId/galeria': 'role',
  administracion: 'role',
  'administracion/global': 'capability',
  acceso: 'public',
  registro: 'public',
  'verificar-correo': 'public',
  'recuperar-acceso': 'public',
  'mi-cuenta/seguridad': 'identity',
  'mi-cuenta': 'identity',
  'mi-cuenta/privacidad': 'identity',
  'mi-cuenta/verificaciones': 'identity',
  reservas: 'identity',
  'reservas/:id': 'identity',
  'reservas/:id/disputas': 'identity',
  'administracion/disputas': 'role',
  '**': 'redirect',
} as const;

describe('application routes', () => {
  it('keeps a titled lazy route or explicit redirect and the expected access guard for every path', () => {
    const paths = routes.map((route) => route.path ?? '');
    expect([...paths].sort()).toEqual(Object.keys(routeAccess).sort());
    expect(new Set(paths).size).toBe(paths.length);

    for (const route of routes) {
      const path = route.path ?? '';
      const access = routeAccess[path as keyof typeof routeAccess];
      if (access === 'redirect') {
        expect(route.redirectTo, path).toBe(path === 'panel-anfitrion' ? 'panel/espacios' : '');
        continue;
      }

      expect(route.loadComponent, path).toBeTypeOf('function');
      expect(route.title, path).toBeTruthy();
      if (access === 'public') expect(route.canActivate, path).toBeUndefined();
      if (access === 'identity') expect(route.canActivate, path).toContain(identityGuard);
      if (access === 'role') expect(route.canActivate, path).toContain(roleGuard);
      if (access === 'capability') {
        expect(route.canActivate, path).toContain(roleGuard);
        expect(route.data?.['requiredCapability'], path).toBe('accounts.list');
      }
    }
  });

  it('keeps the host panel alias redirect valid and protects its destination', () => {
    const alias = routes.find((route) => route.path === 'panel-anfitrion');
    const destination = routes.find((route) => route.path === 'panel/espacios');

    expect(alias?.redirectTo).toBe('panel/espacios');
    expect(alias?.canActivate).toBeUndefined();
    expect(destination?.canActivate).toBeDefined();
    expect(destination?.data?.['requiredRole']).toBe('arrendador');
    expect(destination?.data?.['allowFirstBusinessAction']).toBe(true);
  });

  it('serves the private profile and photo controls from the authenticated account route', () => {
    const account = routes.find((route) => route.path === 'mi-cuenta/privacidad');

    expect(account?.loadComponent).toBeDefined();
    expect(account?.canActivate).toBeDefined();
    expect(account?.data?.['requiredRole']).toBeUndefined();
    expect(account?.title).toBe('Privacidad de la cuenta | EspaciGo');
  });
});
