# Acta de cierre local — estado de verificación

Fecha: 2026-10-14 (America/Santiago). Estado: **no aceptada; LOCAL-CLOSE-02 sigue abierto por evidencias faltantes**.

## Alcance ensayado

Release base `701dabbf7d381c69d240f8cbed823c69de5e4ca1` (merge #309 por `HernanEspinozaDev`). Se verificaron Go, Angular/SSR y un stack aislado Angular→Go→PostgreSQL sobre base vacía con datos sintéticos. La evidencia detallada de comandos, resultados, entorno, migraciones, capturas y límites está en [`evidencia/local-close-02/README.md`](evidencia/local-close-02/README.md). No se alteraron migraciones V1–V57 ni se usó `espacigo_pgdata`.

El contrato `planning/openapi.yaml` contabiliza 152 operaciones (`operationId`) en el HEAD base. `api:check` pasó; el corte no cambió endpoints ni migraciones.

## Resultado por grupo

- **Go:** QA completo PASS: tests unitarios, `go vet`, PostgreSQL/PostGIS + Mailpit, 91 integraciones requeridas sin skips y cinco repeticiones exitosas de check-in.
- **Angular:** lockfile, lint/arquitectura/tokens, SDK OpenAPI, 281 pruebas, build SSR/prerender y auditoría de dependencias de producción PASS.
- **Navegador servido:** 18/18 pruebas Playwright PASS para recorrido público, reserva, oferta/foto, roles, exportaciones, responsive y caída/restauración de Go.
- **Liquidación:** integración PostgreSQL Go cubre frontera checkout+24 h, éxito, CLP y comisión, `cerrada`, historial, recibo, no duplicación ante replay concurrente, timeout y recuperación de conciliación; el handler prueba reglas admin/anfitrión/tercero. No fue un recorrido de navegador de liquidación ni ocurrió en el mismo stack de la suite Playwright.
- **Instalación repetible/restauración:** la suite E2E partió de base vacía y retiró su stack. No se realizó un restore de DB/archivos en un segundo stack.

## Aislamiento y recursos

El Compose temporal `espacigo_localclose02_4a7c2d9e610b`, volumen, red, contenedores y estado privado se retiraron al acabar. Config resuelta validó volumen nuevo, API/Angular/Mailpit en loopback, base sin puerto host y ausencia de referencias a `espacigo_pgdata`. El inventario mostró en ejecución y sin cambios los stacks retenidos MPV2-13 (`a2bf4ff1f4ef`, `316ee26df160`), MPV2-09 y la base de desarrollo. No se tocaron ni se reutilizaron. No quedó una demo LOCAL-CLOSE-02 propia retenida.

## Pendientes que impiden la aceptación

1. Recorrido servido en el mismo release que complete firma, check-in/out, recepción, disputa/descargo/resolución, liberación de garantía, rechazo pre-24 h, liquidación posterior, estado `cerrada`, comprobante visible, reintento idempotente y autorización/no acceso de tercero.
2. Reloj lógico montado desde una ruta única de este corte, con evidencia de hora IANA y registro del límite. El host, datos históricos y sesiones no deben modificarse.
3. Completar el ciclo funcional dentro de la demo retenida, empezando por la cobertura actual acotada a catálogo/roles.
4. Restore real de la base/archivos del ensayo y registro de ciclo de vida de la demo.

## Exclusiones y decisión

No se usaron proveedores reales, fondos/personas reales ni GCP. No se cerraron Issue #267 ni #269 ni se afirma aceptación de DOCKER-02. La aceptación MPV2-13 sigue limitada al producto/UX que cubrió #307. El cierre integral local requiere que el usuario revise la evidencia completa y decida explícitamente; este acta no anticipa esa decisión.
