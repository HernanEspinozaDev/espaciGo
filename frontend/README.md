# EspaciGo Angular

Frontend independiente del backend Go y del mock de transición. Cubre FE-00/FE-01 y el corte FE-02 de identidad: contrato generado, shell responsivo, páginas informativas, comprobación real de readiness, registro y sesión conectados a Go. Catálogo, reservas y demás capacidades quedan para sus cortes; los destinos visibles no implican que estén implementados.

## Herramientas fijadas

- Node `24.21.0` (`.nvmrc`; Angular 22 requiere Node `^22.22.3 || ^24.15.0 || >=26.0.0`).
- Angular framework, CLI, build y compiler-cli `22.2.2`.
- TypeScript `6.0.3` (rango Angular `>=6.0.0 <6.1.0`).
- npm `12.1.0`, RxJS `7.8.2`, `@hey-api/openapi-ts` `0.99.0`.

## Contrato y cliente

El contrato canónico es `../planning/openapi.yaml` (OpenAPI 3.1.0). El cliente Angular se regenera sin editar archivos generados:

```sh
npm ci
npm run api:generate
npm run api:check
```

Se comprobó el generador `typescript-angular` de OpenAPI Generator, pero el ejecutor estándar requiere Java, no instalado en el entorno de desarrollo. El generador fijado Hey API 0.99.0 reconoce OpenAPI 3.1. La primera ejecución detectó una referencia inexistente `#/components/parameters/IdempotencyKey`; se añadió el parámetro reutilizable a la especificación y la generación produjo el SDK Angular completo. `api:check` regenera el cliente y compila el TypeScript de la aplicación. El output incluye clientes tipados para login, perfil, catálogo autenticado y archivo ZIP privado.

El SDK conserva `int64` JSON como `number`; por ello, no se deben presentar o usar como valores exactos importes fuera del rango seguro de JavaScript. FE-00 deja esta limitación explícita para resolver el decodificador de números JSON sin pérdida antes de los flujos financieros. El corte actual no consume ni transforma importes.

FE-02 reutiliza el SDK para identidad y mantiene Bearer solo en memoria. El adaptador pasa explícitamente Authorization a las operaciones protegidas; una prueba de transporte cubre la configuración del SDK generado. Los archivos binarios están tipados como `Blob | File`; su descarga autenticada, manejo de URL temporal y aislamiento entre sesiones quedan para el corte de privacidad que consume esas operaciones.

## Servidor local

`GET /healthz` comprueba el proceso Angular. La aplicación consulta el endpoint real `GET /health/ready` del backend Go mediante `API_PUBLIC_BASE_URL`; nunca muestra una disponibilidad sintética. En ejecución Docker esta URL es configuración pública del navegador y no contiene credenciales. El origen Angular local se agrega a la lista CORS de Compose, sin cambiar rutas, autenticación ni autorización del backend.

Para añadir Angular al stack local y conservar el mock:

```sh
docker compose -f compose.yaml -f compose.frontend.yaml up --build angular-frontend
```

El frontend se publica en `http://localhost:4200` (override `LOCAL_ANGULAR_PORT`). Puede ejecutarse independientemente con `npm start`; la configuración de desarrollo apunta a `http://localhost:8080/api/v1`.

## Stack FE-13 y runner integrado

La validación desde cero se ejecuta con `scripts/test-frontend-docker.sh`. Usa un proyecto Compose, volumen PostgreSQL, puertos y directorios de secretos efímeros propios; al terminar elimina únicamente ese proyecto/volumen. El stack incluye Go, migrador, Mailpit, mock opcional y Angular; Angular y el runner solo están en la red `api`, sin secretos/DB. Override `FE13_PORT_BASE` para escoger la primera de cuatro puertos consecutivos libres; `FE13_KEEP_STACK=1` conserva el stack aislado para inspección.

El runner `angular-runner` verifica el HTML de Angular, su configuración pública no cacheable, el readiness real de Go y CORS/preflight desde el origen del navegador en el host. Usa `http://backend:8080` solo dentro de la red Compose; `/config.json` conserva la URL `127.0.0.1` que el navegador del host puede abrir. Comandos de bajo nivel: `scripts/local-docker-stack.sh up-frontend`, `verify-frontend`, `down-frontend`.

Instalación limpia y regresión desde `frontend/`: `npm ci`, `npm test -- --watch=false`, `npm run api:check` (sin `ng serve` activo) y `npm run build`. El mock permanece disponible pero el runner y la app Angular no lo consultan.

## Alcance visible

`/`, `/categorias` y `/como-funciona` contienen solo información del producto y son prerenderizadas. Las páginas de identidad y cuenta son CSR/noindex. `/explorar` y las áreas de anfitrión/administración requieren la sesión, roles y modo restringido que confirma Go; sus funciones posteriores siguen pendientes. El registro distingue la preferencia inicial de los roles autorizados. No se muestran fichas, disponibilidad, precios ni acciones no respaldadas por una sesión o por el contrato actual.

## Identidad (FE-02)

Las pantallas `/acceso`, `/registro`, `/verificar-correo`, `/recuperar-acceso`, `/mi-cuenta` y `/mi-cuenta/seguridad` consumen operaciones `/auth/*` de Go. Tokens de verificación/recuperación se envían solo en el cuerpo del formulario; Bearer nunca se persiste en localStorage/sessionStorage ni se coloca en URLs o logs. Los requisitos locales de términos son metadatos versionados/hash, no texto legal.

## Fronteras de arquitectura

Las vistas standalone delegan la coordinación a fachadas por feature. Las fachadas dependen de puertos propios y proyecciones puras; los adaptadores HTTP/SDK están en `infrastructure/`. `frontend.providers.ts` es la composición DI. Administración/finanzas usan operaciones semánticas, cuya traducción REST está exclusivamente en infraestructura. Sesión, reintentos inciertos y snapshots conservan sus contratos previos; el registro de reservas es TypeScript puro.

`npm run lint` aplica Angular ESLint, Stylelint y el chequeo de capas; `npm run api:check` valida el contrato y la compatibilidad de proyecciones. TypeScript y templates son estrictos. UI común y formularios tipados se documentan en `planning/frontend/catalogo_ui.md`; auditoría y decisiones en `planning/frontend/revision_arquitectura.md`. La demo/aceptación FE-14 se repite después de aprobar esta adecuación; FE-15 continúa bloqueado.
