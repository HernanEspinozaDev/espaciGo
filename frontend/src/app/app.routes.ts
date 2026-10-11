import { Routes } from '@angular/router';

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
      import('./features/coming-soon/coming-soon').then((module) => module.ComingSoon),
    title: 'Explorar espacios | EspaciGo',
    data: {
      title: 'Explorar espacios',
      message: 'El catálogo privado se habilitará con sesión en el corte FE-03.',
    },
  },
  {
    path: 'acceso',
    loadComponent: () =>
      import('./features/coming-soon/coming-soon').then((module) => module.ComingSoon),
    title: 'Acceso | EspaciGo',
    data: {
      title: 'Acceso a tu cuenta',
      message: 'El acceso y la sesión se implementarán en el corte FE-02.',
    },
  },
  { path: '**', redirectTo: '' },
];
