import { Routes } from '@angular/router';
import { identityGuard, roleGuard } from './core/identity/identity.guard';

const routeDefinitions: Routes = [
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
    title: 'Explorar espacios | EspaciGo',
  },
  {
    path: 'espacios/:id',
    loadComponent: () =>
      import('./features/explore/space-detail-page').then((module) => module.SpaceDetailPage),
    title: 'Detalle de espacio | EspaciGo',
  },
  {
    path: 'panel-anfitrion',
    redirectTo: 'panel/espacios',
    pathMatch: 'full',
  },
  {
    path: 'panel/espacios',
    loadComponent: () =>
      import('./features/hosting/hosting-spaces-page').then((module) => module.HostingSpacesPage),
    canActivate: [roleGuard],
    title: 'Mis espacios | EspaciGo',
    data: { requiredRole: 'arrendador' },
  },
  {
    path: 'panel/espacios/:spaceId/editar',
    loadComponent: () =>
      import('./features/hosting/hosting-spaces-page').then((module) => module.HostingSpacesPage),
    canActivate: [roleGuard],
    title: 'Editar espacio | EspaciGo',
    data: { requiredRole: 'arrendador', section: 'datos' },
  },
  {
    path: 'panel/espacios/:spaceId/tarifas',
    loadComponent: () =>
      import('./features/hosting/hosting-spaces-page').then((module) => module.HostingSpacesPage),
    canActivate: [roleGuard],
    title: 'Tarifas | EspaciGo',
    data: { requiredRole: 'arrendador', section: 'tarifas' },
  },
  {
    path: 'panel/espacios/:spaceId/calendario',
    loadComponent: () =>
      import('./features/hosting/hosting-spaces-page').then((module) => module.HostingSpacesPage),
    canActivate: [roleGuard],
    title: 'Calendario | EspaciGo',
    data: { requiredRole: 'arrendador', section: 'calendario' },
  },
  {
    path: 'panel/espacios/:spaceId/galeria',
    loadComponent: () =>
      import('./features/hosting/hosting-spaces-page').then((module) => module.HostingSpacesPage),
    canActivate: [roleGuard],
    title: 'Galería | EspaciGo',
    data: { requiredRole: 'arrendador', section: 'galeria' },
  },
  {
    path: 'administracion',
    loadComponent: () => import('./features/admin/admin-page').then((module) => module.AdminPage),
    canActivate: [roleGuard],
    title: 'Administración | EspaciGo',
    data: { requiredRole: 'administrador', noindex: true },
  },
  {
    path: 'administracion/global',
    loadComponent: () => import('./features/global-admin/global-admin-page').then((module) => module.GlobalAdminPage),
    canActivate: [roleGuard],
    title: 'Administración global | EspaciGo',
    data: { requiredCapability: 'accounts.list', noindex: true },
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
      import('./features/account/account-privacy-page').then((module) => module.AccountPrivacyPage),
    canActivate: [identityGuard],
    title: 'Mi cuenta y privacidad | EspaciGo',
  },
  {
    path: 'mi-cuenta/privacidad',
    loadComponent: () =>
      import('./features/account/account-privacy-page').then((module) => module.AccountPrivacyPage),
    canActivate: [identityGuard],
    title: 'Privacidad de la cuenta | EspaciGo',
  },
  {
    path: 'mi-cuenta/verificaciones',
    loadComponent: () =>
      import('./features/verification/verification-page').then((module) => module.VerificationPage),
    canActivate: [identityGuard],
    title: 'Verificaciones | EspaciGo',
  },
  {
    path: 'reservas',
    loadComponent: () =>
      import('./features/bookings/bookings-page').then((module) => module.BookingsPage),
    canActivate: [identityGuard],
    title: 'Reservas | EspaciGo',
  },
  {
    path: 'reservas/:id',
    loadComponent: () =>
      import('./features/bookings/booking-detail-page').then((module) => module.BookingDetailPage),
    canActivate: [identityGuard],
    title: 'Detalle de reserva | EspaciGo',
  },
  {
    path: 'reservas/:id/disputas',
    loadComponent: () =>
      import('./features/finance/finance-disputes-page').then(
        (module) => module.FinanceDisputesPage,
      ),
    canActivate: [identityGuard],
    title: 'Disputas y finanzas | EspaciGo',
  },
  {
    path: 'administracion/disputas',
    loadComponent: () =>
      import('./features/finance/finance-disputes-page').then(
        (module) => module.FinanceDisputesPage,
      ),
    canActivate: [roleGuard],
    title: 'Disputas y finanzas | EspaciGo',
    data: { requiredRole: 'administrador', mode: 'admin' },
  },
  { path: '**', redirectTo: '' },
];

export const routes: Routes = routeDefinitions;
