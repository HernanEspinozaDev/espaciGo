# GCP-01 — portabilidad local y límites de integración

Fecha de ejecución: 2026-10-15. Rama temporal: `codex/infra/feature/gcp-01-local-portability`. Base integrada GCP-00: #324, merge `7d511d415c2bfe3cc45bd9935d50cff0e3d5726b`.

## Implementado en el corte local

- El API puede escuchar en `PORT` (Cloud Run) o `HTTP_ADDR`; local conserva `:8080`. `Dockerfile` instala y copia el bundle de CA raíz al runtime `scratch` para HTTPS hacia Google APIs.
- `DATABASE_CONNECTION_MODE=cloudsql` usa Cloud SQL Go Connector con ADC, nombre de instancia, elección de IP privada y `pgxpool`; TLS queda en el conector. La contraseña continúa viniendo de un archivo runtime (preparado para un volumen de Secret Manager). Pool por proceso: 10 por defecto, mínimo configurable y máximo 50.
- `BLOB_STORE_BACKEND=gcs` selecciona un adaptador de objetos privados con IDs UUID y precondición `DoesNotExist`, sin ACL pública ni URL de objeto. `filesystem` sigue siendo la opción local predeterminada.
- `APP_PROFILE=cloud-test` selecciona los servicios sintéticos de ensayo y rechaza mezclarse con `LOCAL_AUTH_PROTOTYPE`; no activa proveedores reales ni altera la autorización Go.
- SSR puede publicar API same-origin `/api/v1`. El navegador usa el gateway frontend; Cloud Run exige API interno HTTPS, audiencia igual al origen del servicio API y separa el Bearer de EspaciGo en `Authorization` del ID token server-side en `X-Serverless-Authorization`. El código elimina cabeceras IAM/IAP y cookies aportadas por clientes antes de llamar al API.
- Las lecturas públicas de Angular SSR pasan por un interceptor que solo autentica el origen y el path `/api/v1` internos configurados. Usa el mismo proveedor de ID token server-side que el gateway Express, establece `transferCache: false` y el filtro de TransferCache rechaza cualquier solicitud con credencial IAM.
- La entrada privada exige `IAP_JWT_AUDIENCE` y valida `x-goog-iap-jwt-assertion` con claves públicas oficiales rotables, ES256, `kid`, firma, `iss`, audiencia exacta y vigencia `iat`/`exp` con tolerancia de 30 segundos; configuración ausente, error de claves o claims inválidos rechazan la solicitud. Solo `/healthz` y `/health/ready` quedan exentos. La audiencia IAP (`/projects/.../locations/.../services/...`) es distinta de `API_SERVICE_AUDIENCE` (origen HTTPS de invocación API).
- La navegación Angular SPA a login/registro/rutas privadas usa navegación de documento hacia `PRIVATE_DEMO_ORIGIN`; `returnUrl` solo se conserva si `parseIdentityReturn` lo reconoce. Se cubre el salto desde catálogo y detalle y la ruta de retorno contextual seleccionada.
- La ruta pública admite solo GET anónimos de catálogo, detalle, imágenes, categorías y ubicaciones; excluye rutas/métodos codificados o privados. Las pruebas fijan URL pública de 4 KiB, JSON público de 2 MiB, timeout público de 10 s, imágenes de 1 MiB y sitemap de hasta 49.996 resultados. Solo se permite caché si el backend de una ruta pública indica explícitamente `public, max-age` entre 1 y 300 s; toda otra respuesta queda `private, no-store`.
- Mailpit se define como VM dedicada privada en la VPC, con SMTP 1025 solo desde API Cloud Run, disco persistente para bandeja, sin relay externo y UI/API accesibles solo a evaluadores por IAP. El ensayo local mantiene Mailpit dedicado de Compose; GCP-01 no crea VM ni modifica redes.

## Verificación local de este PR

- `go test ./...`: pasó localmente. `go vet ./cmd/api ./internal/adapters/cloudstorage`: pasó. Incluye tests unitarios de configuración/Cloud SQL, store GCS simulado, filesystem, privacy, identidad y outbox/mail.
- `npm test -- --watch=false`: 42 archivos y 314 pruebas pasaron.
- `npm run test:server-gateway` y `npm run test:iap-assertion`: verifican allowlist, separación de identidades, límites del gateway, bypasses de health explícitos y JWT IAP firmado con rechazos de firma, issuer, audience, alg/kid, expiración, `iat`, audiencia múltiple y claves no disponibles.
- Suites dirigidas Angular cubren SSR render con upstream IAM falso, no serialización del token, `transferCache: false`, aislamiento por origen/plataforma, filtro de TransferCache y clicks RouterLink desde catálogo y detalle hacia entrada privada con retorno permitido.
- `npm run architecture:check`: 16 pruebas y chequeo estático de 89 archivos pasaron. `npm run lint`: ESLint, reglas SCSS, Stylelint y arquitectura pasaron.
- `npm run build`: Angular browser+SSR terminó y prerenderizó 3 rutas. El bundle navegador se inspeccionó para confirmar que no contiene hostname de metadata ni URL de claves IAP.
- `docker build --target api -t espacigo-gcp01-check:local .`: imagen `scratch` compilada; la imagen temporal fue retirada después.
- `npm audit --omit=dev`: 0 vulnerabilidades de producción. `npm ci` informó 5 high en el conjunto completo con devDependencies; no se actualizaron dependencias fuera del alcance.
- `git diff --check` y `gofmt -d` de los Go modificados pasaron. El fake HTTP de GCS verifica create-only/Get/Delete; no es Cloud Storage real.

## Límites que bloquean despliegue

- `LOCAL_PRIVACY_REPLAY_FILE` todavía hace reemplazo atómico en el sistema de archivos local. La selección GCS actual cubre blobs, no ese registro. Hace falta un puerto persistente compartido y pruebas de concurrencia, idempotencia y recuperación antes de ejecutar Cloud Run.
- Los workers se inician dentro de cada proceso API. No hay lease PostgreSQL ni exclusión entre revisiones de Cloud Run; GCP-04 debe separar/coordenar trabajadores y probar cierre, reinicio y duplicados. No declarar seguridad multi-instancia.
- El límite agregado por IP se aplicará después en Cloud Armor sobre el balanceador. Criterio inicial documentado: 60 solicitudes/minuto/IP al catálogo y 120/minuto/IP al origen público, observando 429 y recuperación de ventana. No se prueba en este corte local.
- El perfil de ensayo no incorpora envío SMTP real: los datos de email son sintéticos y ninguna infraestructura cloud se crea. Se debe probar que el outbox retiene y reintenta mensajes con Mailpit privado en una fase posterior.
- No se probó conectividad, autenticación, permisos, Cloud SQL, Cloud Storage, Cloud Run/IAP, Cloud Armor, DNS, certificado, migración o restore reales en GCP. No se ejecutó Terraform, no se habilitaron APIs, no se cambiaron IAM/DNS y no se subieron datos.
- Cloud SQL PG18/PostGIS 3.6.0 frente al origen local PG18/PostGIS 3.6.4 permanece como gate de restore GCP-03. V1–V61 no se modifican en este corte.
