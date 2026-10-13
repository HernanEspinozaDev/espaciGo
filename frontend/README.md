# Frontend Angular

Angular 22 es la interfaz local predeterminada de EspaciGo. El servicio `angular-frontend` de `compose.yaml` sirve la aplicación SSR y CSR en `http://127.0.0.1:4200`; Go permanece en sus rutas `/api/v1` y valida readiness, sesión, roles y permisos. La configuración del navegador vive en `/config.json` y contiene únicamente la URL pública del API. Angular no recibe secretos ni acceso a la red de datos.

## Desarrollo

Desde `frontend/`, usar Node 24.15+ compatible con Angular 22 y el lockfile: `npm ci`, `npm start`, `npm test -- --watch=false`, `npm run api:check` y `npm run build`. Para agregar Angular al stack: `scripts/dev-env.sh up`; para verificar readiness/CORS y endpoints del shell Angular: `scripts/dev-env.sh verify-http`.

El stack aislado del runner se controla con `scripts/local-docker-stack.sh up-frontend`, `verify-frontend`, `run-e2e`, `down-frontend`. La aplicación Angular es el servicio por defecto en Compose; el runner de verificación/E2E es opcional y se añade mediante overlays dedicados. `scripts/test-frontend-docker.sh` hace una instalación limpia con proyecto, volumen, puertos y archivos de estado desechables, y comprueba arranque, caída/restauración de Go, la sesión y el aislamiento de secretos. `FE15_PORT_BASE` elige el primer puerto libre y `FE15_KEEP_STACK=1` conserva el stack aislado para inspección.

## Validación

Instalación limpia y regresión: `npm ci`, `npm test -- --watch=false`, `npm run api:check` (sin `ng serve` activo) y `npm run build`. QA local integral: `scripts/test-local-qa.sh` ejecuta QA Angular y luego Go. `scripts/test-local-qa-backend.sh` permite correr Go unitario, vet e integraciones desechables sin instalar ni invocar Node/npm; el backend no compila ni ejecuta un frontend. Los tests de UI que cubrían sesión, filtros, publicaciones, pagos, archivos, conversaciones y administración fueron sustituidos por pruebas Angular de servicios/componentes/fachadas con el SDK generado y, para recorridos completos, Playwright contra Go.
