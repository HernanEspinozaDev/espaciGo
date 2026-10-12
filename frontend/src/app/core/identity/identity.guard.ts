import { inject } from '@angular/core';
import { CanActivateFn, Router, UrlTree } from '@angular/router';
import { IdentitySession } from './application/identity-session';

export const identityGuard: CanActivateFn = () => {
  const identity = inject(IdentitySession);
  const router = inject(Router);
  return identity.session() ? true : router.createUrlTree(['/acceso']);
};

export const roleGuard: CanActivateFn = (route): boolean | UrlTree => {
  const identity = inject(IdentitySession);
  const router = inject(Router);
  const session = identity.session();
  if (!session) return router.createUrlTree(['/acceso']);
  if (session.restricted_mode) return router.createUrlTree(['/mi-cuenta']);
  const role = route.data['requiredRole'] as string | undefined;
  return role && session.roles.includes(role as (typeof session.roles)[number])
    ? true
    : router.createUrlTree(['/mi-cuenta']);
};
