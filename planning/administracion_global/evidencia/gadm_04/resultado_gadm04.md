# Evidencia GADM-04

Código bajo prueba: `27c656aeaf0c8436143f99013960d6cbebcef08f` (`codex/marketplace/chore/gadm-04-regression-master-delivery`). Los sidecars `.json` de cada captura repiten este SHA y registran viewport y ancho/alto de documento. `SHA256SUMS` cubre este registro, sidecars, capturas y plan SQL.

## Pruebas

| Capa | Ejecución / entorno | Resultado |
| --- | --- | --- |
| Go unitario y vet | `bash scripts/test-local-qa-backend.sh`; sin servicios compartidos para unit tests | Suite Go completa y `go vet` pasaron. |
| Integración obligatoria | Mismo runner; Postgres/PostGIS + Mailpit efímeros por paquete | 144 eventos Go pasaron; 92 integraciones requeridas pasaron, 0 skips. Incluye `internal/adminglobal::TestGlobalBootstrapDirectoryAndLastMasterConcurrency`, con destino verificado, bootstrap único/replay, recovery, permisos y concurrencia del último global. |
| Estabilidad | Runner backend, booking Postgres aislado | `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry` pasó cinco repeticiones. |
| Migraciones | Integración GADM-03 en Compose FE-14 del ensayo | Base vacía y upgrade desde V56 y V58 pasaron; checksums de migraciones aplicadas sin cambio. GADM-04 no altera `db/migrations`; V1–V59 iguales a `27c656a`. |
| Contrato Angular | `npm run api:check`, `npm run lint`, `npm test -- --watch=false`, `npm run build` | OpenAPI/SDK y TypeScript pasaron; lint, estilos y arquitectura pasaron; 39 archivos/306 pruebas Angular; build SSR/browser con 3 rutas prerenderizadas. Las pruebas de componente cubren errores y reintento KPI/auditoría/exportación. |
| Recorrido conectado | Proyecto Compose desechable `espacigo_fe14_bbb5106b5ccc`, puertos 19780–19783; Angular→Go→PostGIS | 5/5 Playwright: catálogo anónimo; roles, navegación, JSON/ZIP y foco de teclado; dos recorridos GADM-02 de mutación real; GADM-03 KPI/auditoría/export. Persistencia y eventos de auditoría comprobados por SQL; descarga JSON/ZIP inspeccionada en disco. |
| GADM-03, volumen/perfil | Misma PostGIS desechable, 1.000 cuentas sintéticas sin login | Seed comprobado en exactamente 1.000 filas. Consulta real de prefijo/página 25 guardada en `gadm03-account-query-plan.txt`: 1,179 ms, 25 filas, 1.043 bloques cacheados, 0 bloques leídos. Medida de ensayo, no SLO. API prueba paginación keyset/snapshot y actor distinto de cuenta objetivo. Fuente revisada: consultas por página/categoría son de cantidad fija (cuenta usa conteo+página; KPI agregados; auditoría una página), sin consultar una fila por resultado; no se hizo benchmark independiente de producción. |
| LOCAL-CLOSE-02 | Proyecto efímero `espacigo_localclose02_f14c10c2026d`, puertos 19790–19793, reloj coordinado | 1/1 Playwright de reserva firmada y cierre financiero; SSR/sitemap 503 durante caída de Go y recuperación después. Logs sin correo/contraseña sintéticos; estado, credenciales y volumen retirados al terminar. |
| Higiene | `git diff --check`; preflight Compose | Sin cambios de migración. Proyectos y puertos exclusivos; se detectó 18780 ocupado por demo MPV2-09 y el runner terminó sin tocarla. El preflight añadido rechaza sockets y puertos Docker ocupados antes de iniciar. `espacigo_pgdata` y demos retenidas permanecieron intactos. |

## Capturas

Los JPEG son capturas reales del navegador con `deviceScaleFactor=1`. Los tamaños de viewport son 390×844 y 1365×900; las dimensiones completas de documento están en los sidecars. Los cuatro JPEG de administración no contienen correos ni credenciales; las filas auditadas muestran UUID sintéticos. El foco visible se comprobó con Tab en el recorrido de roles. Error→reintento se comprobó en la suite de componentes Angular, no se presenta como captura de un error real de servidor.

| Archivo | Viewport | Documento |
| --- | ---: | ---: |
| `fe14_gadm_02_admin_mobile.jpg` | 390×844 | 390×3474 |
| `fe14_gadm_02_admin_desktop.jpg` | 1365×900 | 1365×2122 |
| `fe14_gadm_03_admin_mobile.jpg` | 390×844 | 390×5245 |
| `fe14_gadm_03_admin_desktop.jpg` | 1365×900 | 1365×2927 |
| `local-close-02/fe14_local_close_02_closed_history_renter_mobile.jpg` | 390×844 | 390×5139 |
| `local-close-02/fe14_local_close_02_closed_finance_desktop.jpg` | 1365×900 | 1365×2907 |

## Alcance y límites

Las pruebas prueban regresión técnica con datos sintéticos. No equivalen a aceptación de producto, no activan cuenta real, soporte ni GCP y no prueban cobros reales. La cuenta personal del propietario y el entorno local destino no están identificados. El CLI vigente solo admite DB de ensayo loopback; la activación real permanece pendiente de UUID/correo verificado y nombre de entorno/base elegidos por el propietario. `operations.*` y `finance.*` siguen como capacidades propuestas, pendientes de decisión de alcance.
