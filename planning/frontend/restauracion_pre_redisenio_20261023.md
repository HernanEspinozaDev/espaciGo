# Restauración del frontend anterior al rediseño

Solicitud humana: el resultado UX-02–UX-09 fue rechazado por navegación y fidelidad visual; restaurar main antes de ese diseño y reconstruir localmente en una rama aparte. No representa aceptación del resultado rechazado.

## Base y alcance

- Main de partida: `275c0bfad3378622fa36c230a5903d53243c565f` (PR #549).
- Base funcional/visual restaurada: `0b8548e2dc4ef2ff5d368235087ca0b279296220` (PR #539), anterior a UX-02.
- Se restaura el árbol `frontend` y los dos scripts de ensayo que modificó el rediseño (`provision-fe14-accounts.py`, `test-frontend-e2e.sh`).
- Excepción deliberada sin cambio visual: conservar el lockfile actual y el override Zod 4.6.6, incluidos body-parser 2.4.0/qs 6.16.1, para no reintroducir vulnerabilidades ya corregidas.
- Se conserva la regla de pruebas solo locales en AGENTS, actualizando expresamente que la nueva reconstrucción no está autorizada para publicar. El catálogo UI vuelve a las capacidades de #539, con evidencia FE-14 rotulada histórica. Se conservan las evidencias históricas de planning. Las pruebas específicas retiradas junto a su implementación siguen conservadas en la historia Git; no se reescribe historia ni se hace force-push.
- Sin cambios de API, backend, OpenAPI, migraciones, datos, Cloud SQL, IAM, Terraform, trigger, cloudbuild o SMTP. Se conserva el arreglo #539 y las capacidades previas, no se retrocede a antes de identidad o validaciones #538.

## Verificación ejecutada por el autor

- Instalación limpia con Node 24.21.0/npm 12.1.0 mediante npm ci.
- `npm run api:check`: PASS; generación sin cambios de contrato.
- `CI=true npm test -- --watch=false`: 341/341, 42 archivos, PASS.
- `npm run lint`: PASS (incluye estilos y arquitectura).
- `npm run build`: PASS; warning histórico hosting SCSS 9.09 kB frente a 9 kB, sin aumentar presupuestos.
- `npm run test:server-gateway`: 11/11 PASS.
- `npm audit --omit=dev --audit-level=low`: 0 vulnerabilidades.
- Comparación de fuentes runtime/rutas/guards/estilos y arnés con #539: iguales a la base solicitada; dependencias excepto los parches de seguridad descritos.
- Integración Express gateway contra stub local: 1/1 PASS; assertion IAP: 8/8 PASS.
- `git diff --check`: PASS.

No se hicieron comprobaciones online, login ni uso de cuentas. Pasar estas pruebas no demuestra todavía todas las interacciones de pestañas reportadas por el usuario: la reconstrucción local deberá probar cada destino con navegación de navegador real y confrontarlo con la maqueta A, sin aceptar mocks/capturas de la propia implementación como prueba de fidelidad.

## Entrega y siguiente trabajo

Este PR debe ser revisado por HernanEspinozaDev y fusionado mediante SHA esperado; restaura main sin reset. Recuperación de la web exclusivamente mediante operador de release y flujo existente o revisiones frontend previas verificadas, sin modificar datos o infraestructura. No afirmar deploy restaurado hasta verificar sus metadatos.

Después del merge, crear `codex/frontend/fix/rediseno-de-frontend` desde main restaurado en worktree aislado. Conservar la maqueta aprobada de `ux-local-prototypes` y el historial rechazado; reconstruir el frontend solo localmente. Ninguna nueva publicación del rediseño hasta revisión expresa del usuario. El PR #547 de CI/CD permanece detenido.
