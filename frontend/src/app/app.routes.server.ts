import { RenderMode, ServerRoute } from '@angular/ssr';

export const serverRoutes: ServerRoute[] = [
  {
    path: '',
    renderMode: RenderMode.Prerender,
  },
  {
    path: 'categorias',
    renderMode: RenderMode.Prerender,
  },
  {
    path: 'como-funciona',
    renderMode: RenderMode.Prerender,
  },
  {
    path: 'explorar',
    renderMode: RenderMode.Server,
  },
  {
    path: 'espacios/:id',
    renderMode: RenderMode.Server,
  },
  {
    path: '**',
    renderMode: RenderMode.Client,
  },
];
