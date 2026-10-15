# Evidencia GADM-04

La tabla inicial registra el baseline de evidencia GADM-04 en `27c656aeaf0c8436143f99013960d6cbebcef08f`. Sus sidecars `.json` repiten ese SHA. Las nuevas pruebas de revisión, su SHA final, dimensiones y checksums se agregan después del cierre del recorrido actualizado.

## Revisión GADM-04 actualizada — HEAD 94aba71dff8e6abc7d03bdef0daf01e441381cf0

Ejecutada el 2026-10-15 en el worktree de `codex/marketplace/chore/gadm-04-regression-master-delivery`, con Go, FE-14 y PostgreSQL/PostGIS desechables. Esta sección reemplaza para el SHA indicado el inventario preliminar que sigue más abajo.

| Capa | Entorno / alcance | Resultado |
| --- | --- | --- |
| Backend local | `bash scripts/test-local-qa-backend.sh` | 146 eventos de tests Go pasaron; 92 integraciones requeridas, cero skips; `go vet` pasó; el lifecycle de reserva/cierre pasó cinco repeticiones. Runner verificó la retirada de sus contenedores/credenciales. |
| Activación | FE-14 desechable `espacigo_fe14_4db11cb116d5`, PostgreSQL 18/PostGIS; DB `espacigo_local` | La CLI `activate-local` activó solo el usuario sintético `globalAdmin`. URL/servidor/cliente loopback, DB explícita, cuenta ya verificada, membresía única y evento CLI auditado. Sin migraciones nuevas ni consulta de `espacigo_pgdata`. |
| Migración GADM-03 | Mismo Compose desechable | Base vacía, upgrade V56 y V58 pasaron; historia V1–V59 no cambió. La prueba de integración `TestGlobalBootstrapDirectoryAndLastMasterConcurrency` pasó sin omisiones. |
| Actores/rutas | Mismo Compose; cuatro pruebas Playwright servidas Angular→Go→PostGIS | 4/4 pasaron: GADM-02 (2), GADM-03 (1), GADM-04 (1). El seed confirmó `globalAdmin` con membership global y sin rol heredado, `admin` con rol heredado y sin membership global, y tercero ordinario. Los dos primeros leyeron publicación, reservas, reportes de reservas, finanzas y disputas (200); tercero fue denegado (403). Global-only bloqueó una cuenta y legacy-only la desbloqueó por rutas existentes. Aserciones SQL confirmaron persistencia/auditoría. |
| Secretos y limpieza | Mismo ensayo | Cinco cuentas sintéticas; archivos privados mode 0600; log backend sin correos/contraseñas; proyecto, volumen y estado temporal retirados por el runner. |
| Demo retenida | Compose `espacigo_gadm04_demo_e83a3e47dcb3`, volume `..._pgdata`, ports 20940–20943; `.local/testing/gadm-04-demo/e83a3e47dcb3` | Wrapping GADM-04 pasó 1/1; frontend y `/healthz` devolvieron 200. `accounts.json`, runtime y registro 0600. Stop + reanudación mediante `start-gadm-04-demo.sh` comprobaron readiness 200. Owner HernanMEC; motivo revisión solicitada. La demo permanece arriba para la revisión; retirada segura usa `remove-gadm-04-demo.sh e83a3e47dcb3`. |
| Capturas / consultas | Capturas y sidecars vinculados al SHA en esta carpeta | 390×844: móvil documento 390×3474 (GADM-02) y 390×5341 (GADM-03). 1365×900: escritorio documento 1365×2122 y 1365×2951, respectivamente. Sin desborde horizontal. `gadm03-account-query-plan.txt` contiene el plan real del fixture de 1.000 cuentas. `SHA256SUMS` registra hashes de artefactos/registro. |

La QA Angular (API check, lint, pruebas de componentes y build) había pasado en el baseline GADM-04 anterior; este cambio solo agrega una especificación Playwright y no altera fuentes Angular. `frontend-quality.yml` aplica al PR por `frontend/**`, pero no ejecuta Playwright ni las integraciones Go; el workflow remoto se reportará solo tras consultar su estado en GitHub. `LOCAL-CLOSE-02` es evidencia histórica de una ejecución anterior, no parte de este recorrido final. No se declara FE-14 completo.

La migración V1–V59 permanece intacta. `espacigo_pgdata`, las demos retenidas anteriores, soporte/GCP y pagos reales quedan fuera. No se ejecutó activación real ni aceptación de producto.

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
| `fe14_gadm_03_admin_mobile.jpg` | 390×844 | 390×5341 |
| `fe14_gadm_03_admin_desktop.jpg` | 1365×900 | 1365×2951 |
| `local-close-02/fe14_local_close_02_closed_history_renter_mobile.jpg` | 390×844 | 390×5139 |
| `local-close-02/fe14_local_close_02_closed_finance_desktop.jpg` | 1365×900 | 1365×2907 |

## Alcance y límites

Las pruebas prueban regresión técnica con datos sintéticos. No equivalen a aceptación de producto, no activan cuenta real, soporte ni GCP y no prueban cobros reales. La cuenta personal del propietario y el entorno local destino no están identificados. La CLI ofrece `activate-local` explícito para la DB ordinaria local `espacigo_local`, además del modo aislado `local-test`. GADM-04 la ejecuta solo sobre un destino Compose descartable y una cuenta sintética; la activación real permanece pendiente de UUID verificado y entorno local elegidos por el propietario. `operations.*` y `finance.*` siguen como capacidades propuestas, pendientes de decisión de alcance.
