import { inject } from '@angular/core';
import { CanActivateFn, Router, UrlTree } from '@angular/router';
import { IdentitySession } from './application/identity-session';

export const identityGuard: CanActivateFn = (_route, state) => {
  const identity = inject(IdentitySession);
  const router = inject(Router);
  return identity.session()
    ? true
    : router.createUrlTree(['/acceso'], { queryParams: { returnUrl: state.url } });
};

export const roleGuard: CanActivateFn = (route): boolean | UrlTree => {
  const identity = inject(IdentitySession);
  const router = inject(Router);
  const session = identity.session();
  if (!session) return router.createUrlTree(['/acceso']);
  if (session.restricted_mode) return router.createUrlTree(['/mi-cuenta']);
  const role = route.data['requiredRole'] as string | undefined;
  const capability = route.data['requiredCapability'] as string | undefined;
  const allowFirstBusinessAction = route.data['allowFirstBusinessAction'] === true;
  const allowed = capability
    ? session.capabilities?.includes(capability as (typeof session.capabilities)[number]) === true
    : role === 'administrador'
      ? session.capabilities?.includes('legacy.administration') === true || session.roles.includes('administrador')
      : role !== undefined && (
          session.roles.includes(role as (typeof session.roles)[number]) ||
          (allowFirstBusinessAction && role === 'arrendador' &&
            (session.roles.length === 0 || session.roles.includes('arrendatario')))
        );
  return allowed
    ? true
    : router.createUrlTree(['/mi-cuenta']);
};
