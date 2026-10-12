import { Routes } from '@angular/router';
import { identityGuard, roleGuard } from './core/identity/identity.guard';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () => import('./features/home/home').then((module) => module.Home),
    title: 'EspaciGo | Espacios flexibles para tus ideas',
  },
  {
    path: 'categorias',
    loadComponent: () =>
      import('./features/categories/categories').then((module) => module.Categories),
    title: 'Categorías | EspaciGo',
  },
  {
    path: 'como-funciona',
    loadComponent: () =>
      import('./features/how-it-works/how-it-works').then((module) => module.HowItWorks),
    title: 'Cómo funciona | EspaciGo',
  },
  {
    path: 'explorar',
    loadComponent: () =>
      import('./features/explore/explore-page').then((module) => module.ExplorePage),
    canActivate: [roleGuard],
    title: 'Explorar espacios | EspaciGo',
    data: { requiredRole: 'arrendatario' },
  },
  {
    path: 'espacios/:id',
    loadComponent: () =>
      import('./features/explore/space-detail-page').then((module) => module.SpaceDetailPage),
    canActivate: [roleGuard],
    title: 'Detalle de espacio | EspaciGo',
    data: { requiredRole: 'arrendatario' },
  },
  {
    path: 'panel-anfitrion',
    loadComponent: () =>
      import('./features/coming-soon/coming-soon').then((module) => module.ComingSoon),
    canActivate: [roleGuard],
    title: 'Panel de anfitrión | EspaciGo',
    data: {
      requiredRole: 'arrendador',
      title: 'Panel de anfitrión',
      message: 'La gestión de espacios llegará en FE-06.',
    },
  },
  {
    path: 'administracion',
    loadComponent: () =>
      import('./features/coming-soon/coming-soon').then((module) => module.ComingSoon),
    canActivate: [roleGuard],
    title: 'Administración | EspaciGo',
    data: {
      requiredRole: 'administrador',
      title: 'Administración',
      message: 'Las funciones administrativas llegarán en FE-11.',
    },
  },
  {
    path: 'acceso',
    loadComponent: () =>
      import('./features/identity/identity-page').then((module) => module.IdentityPage),
    title: 'Acceso | EspaciGo',
    data: { identityMode: 'login' },
  },
  {
    path: 'registro',
    loadComponent: () =>
      import('./features/identity/identity-page').then((module) => module.IdentityPage),
    title: 'Crear cuenta | EspaciGo',
    data: { identityMode: 'register' },
  },
  {
    path: 'verificar-correo',
    loadComponent: () =>
      import('./features/identity/identity-page').then((module) => module.IdentityPage),
    title: 'Verificar correo | EspaciGo',
    data: { identityMode: 'verify' },
  },
  {
    path: 'recuperar-acceso',
    loadComponent: () =>
      import('./features/identity/identity-page').then((module) => module.IdentityPage),
    title: 'Recuperar acceso | EspaciGo',
    data: { identityMode: 'recovery' },
  },
  {
    path: 'mi-cuenta/seguridad',
    loadComponent: () =>
      import('./features/identity/identity-page').then((module) => module.IdentityPage),
    canActivate: [identityGuard],
    title: 'Seguridad de la cuenta | EspaciGo',
    data: { identityMode: 'change-password' },
  },
  {
    path: 'mi-cuenta',
    loadComponent: () =>
      import('./features/identity/identity-page').then((module) => module.IdentityPage),
    canActivate: [identityGuard],
    title: 'Mi cuenta | EspaciGo',
    data: { identityMode: 'account' },
  },
  { path: '**', redirectTo: '' },
];
