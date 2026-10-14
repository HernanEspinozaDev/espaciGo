# LOCAL-CLOSE-02 — resultado de validación al 2026-10-14

## Identificación

- Base: merge de PR #309, `701dabbf7d381c69d240f8cbed823c69de5e4ca1`.
- Rama del corte: `codex/marketplace/chore/local-close-02-final-validation`.
- Stack E2E desechable: proyecto `espacigo_localclose02_4a7c2d9e610b`, volumen `espacigo_localclose02_4a7c2d9e610b_pgdata`, puertos loopback API 20860, Mailpit 20861 y Angular 20862 (SSR 20863). Se retiraron contenedores, red, volumen, directorio privado y credenciales al terminar. La config Compose resuelta verificó volumen único, sin `espacigo_pgdata`, PostgreSQL sin puerto publicado y puertos de servicio limitados a loopback.
- La evidencia visual seleccionada incluye mediciones con SHA del código. Las imágenes y JSON de viewport se originaron en esta ejecución. No se incluyen cuentas, secretos, trazas de navegador ni registros de aplicación.

## Comprobaciones ejecutadas

| Capa | Resultado |
| --- | --- |
| Backend | `bash scripts/test-local-qa-backend.sh` PASS en HEAD base: autopruebas 4/4, `go test -p 1 ./...`, `go vet ./...`, PostgreSQL/PostGIS + Mailpit: 132 eventos `passed`, 91 integraciones requeridas, cero skips; integración de check-in repetida cinco veces, cinco PASS. El runner retiró sus recursos temporales. |
| Settlement PostgreSQL | `TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay` se ejecutó en la suite obligatoria. Comprueba límite estricto antes de checkout+24 h y éxito en el instante de frontera luego de esperar bloqueo de fila; CLP 10.000 transferidos, comisión CLP 0; reintentos concurrentes con una operación, una transferencia fake, un comprobante, un historial de confirmación, estado `cerrada`, una transición terminal y lectura del recibo. Un tercero recibe NotFound. El test también prueba timeout persistido y conciliación administrativa tras reinicio, con HTTP handler de reserva. Nivel: integración PostgreSQL + handler Go, no recorrido de navegador servido para liquidación. |
| Angular | `npm ci --no-audit --no-fund`, `npm run lint`, `npm run api:check`, `npm test -- --watch=false` (38 archivos, 281 pruebas), `npm run build` (SSR y tres rutas prerenderizadas), `npm audit --omit=dev` (cero vulnerabilidades): PASS. El `npm ci` informó cuatro paquetes con scripts de instalación bloqueados por la política de npm; el build y las pruebas terminaron PASS. |
| E2E navegador servido | `scripts/test-frontend-e2e.sh`, stack limpio Angular→Go→PostgreSQL; 18/18 Playwright PASS. Incluye datos sintéticos, catálogo real con más de 48 ofertas en ocho categorías, orden/filtros/paginación 12/24/48, registro/verificación/retorno, login y reserva con confirmación explícita, oferta/anfitrión, foto declarada/retirada, roles, exportaciones privadas JSON/ZIP y caída/restauración de Go conservando sesión. También ejecuta las dos pruebas LOCAL-CLOSE-01. |
| Auditoría visual | E2E en escritorio 1365×900 y móvil 390×844; capturas y mediciones seleccionadas incluidas. |
| Migraciones | 57 archivos V1–V57. SHA-256 por archivo en [`migrations.sha256`](migrations.sha256); SHA-256 del manifiesto: `68ebe80d8c9fdd2a799413ad309f48404a3a0b11b92ad0e76601156bf4aa9e3b`. No se modificó ninguna migración en LOCAL-CLOSE-02. El contrato OpenAPI tenía 152 `operationId` al HEAD probado; `api:check` pasó y el contrato no cambió en este corte. |

## Límites y decisión

El recorrido de navegador de esta corrida no avanzó una reserva hasta firma, check-in/out, disputa, liquidación ni comprobante; esos criterios no se atribuyen a las 18 pruebas E2E. La liquidación y su límite 24 h tienen cobertura actual en la integración Go/PostgreSQL citada, pero falta un recorrido servido en el mismo stack release que conecte esas pantallas, pruebe allí el reloj aislado, política/garantía y visibilidad del comprobante. Tampoco se mantuvo una demo final retenida para una sesión manual; solo queda documentación histórica de demos existentes, que no se reutilizaron.

Por eso DOCKER-02 y el cierre local integral siguen sin aceptación. No se cerró #267/#269, no se publicó GCP y no se infiere aceptación de producto. Esta evidencia distingue pruebas de integración y E2E real del recorrido aún pendiente; no sustituye la decisión explícita del usuario.

## Demo LOCAL-CLOSE-02 retenida

La demo de revisión corre en `http://127.0.0.1:20872/explorar`; SSR en `http://127.0.0.1:20873/explorar`; API en `http://127.0.0.1:20870`; Mailpit en `http://127.0.0.1:20871`. Project Compose: `espacigo_localclose02_demo_55369d21130f`. Volumen: `espacigo_localclose02_demo_55369d21130f_pgdata`. Responsable: `HernanMEC`. Motivo: conservar, a petición del usuario, una demo LOCAL-CLOSE-02 aislada para revisión manual mientras la decisión de producto queda pendiente. Cuentas verificadas sintéticas y secretos se encuentran solo en `.local/testing/local-close-02-demo/55369d21130f/`, permisos 0600 para `accounts.json` y `runtime.env`; el estado privado no se versiona.

Se repitieron seis pruebas E2E focalizadas PASS (catálogo, páginas 48, diseño responsivo, retiro de foto y navegación/exports de roles) y el smoke SSR outage/recovery: 503 en catálogo/sitemap con Go detenido, readiness recuperado y SSR 200 al reiniciar. No se usaron recursos de demos anteriores. Imágenes construidas para esta corrida: Angular `sha256:ae092bad9380bcab652e7dbbb3940dc438ee5ad0b7bc50cdb29313f0c9e7105d`, API `sha256:3a038ca815e4435cff30ef6096a69aec315fd757f8e531badf24117bd29ba9a2`, PostgreSQL `sha256:f01d4902011cc201f5f14758daa980110bdf4bb55ca727018eb7c43a9f1f171f`, Mailpit `sha256:b68349e3a014b90c5610bfb26b2ae36f3892d7b8cf25ee140c6c71c98d2fcf48`.

Capturas de la demo con mediciones de viewport: [catálogo escritorio](fe14_public_catalog_desktop_55369d21130f.jpg), [arrendatario escritorio](fe14_renter_desktop_55369d21130f.jpg), [anfitrión escritorio](fe14_host_desktop_55369d21130f.jpg), [administración escritorio](fe14_admin_desktop_55369d21130f.jpg), y [arrendatario móvil](fe14_renter_mobile_55369d21130f.jpg). El stack queda encendido para que se revise. Esta demo cubre datos públicos sintéticos y pantallas por rol; no contiene una reserva de extremo a extremo ni prueba visual de liquidación. No detener ni retirar esta demo como parte de la limpieza de la corrida desechable. Su ciclo de vida queda a cargo del responsable y pendiente de la decisión del usuario.
