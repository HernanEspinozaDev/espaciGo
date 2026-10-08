# Evidencia LOCAL-PRIV-01A4 — 2026-10-08

Issue #196, subentrega de #185 y relacionada con #40. Se reutilizan las implementaciones fusionadas enumeradas en [la decisión y alcance](../LOCAL-PRIV-01A4-exportacion-completa-local.md). No se cierran #185 ni #40.

## Comprobaciones

- `GO_TEST_RUN=TestLocalCompleteExportZIPIsOwnerScopedReadOnlyAndAvailableWithBlockedSuppression bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity` — integración PostgreSQL en contenedor/base efímeros; ejecuta el endpoint autenticado con dos titulares, el rol `espacigo_runtime`, consulta de secciones y archivos, y evaluación de supresión bloqueada.
- `go test ./...` — todos los paquetes Go.
- `npm --prefix mock run build` y `node --test mock/test/privacy-export-state.test.mjs` — compilación del mock y protección de Blob ZIP ante logout/cambio de sesión (3 pruebas).
- Parseo de `planning/openapi.yaml` con PyYAML; se verificó `application/zip` binario en `GET /privacy/export/archive`.
- `git diff --check`.

## Resultado comprobado del archivo HTTP

La prueba recibe los bytes de `GET /api/v1/privacy/export/archive`, abre el ZIP con `archive/zip` y comprueba `manifest.json`, `data.json` versión 1 y el PNG sintético propio. Los manifiestos de anfitrión y arrendatario contienen distintos archivos de evidencia. Se inspeccionan perfil, preferencia, aceptaciones, decisiones de derechos, caso y evidencia, borrador, atributos, tarifa, simulación, cotización, reserva e historial, pago, devolución e intento, disputa e historial, mensaje escrito por el titular y cursor.

El anfitrión tiene una disputa abierta que bloquea la evaluación de supresión de la cuenta; el resultado `bloqueada` no impide recibir su ZIP. El contenido de ambos ZIP se contrasta para excluir email/UUID personal de contraparte y mensajes ajenos. El archivo no exporta roles operativos. Se comparan conteos de derechos, verificación/evidencia, espacios/atributos, tarifa/simulación, cotizaciones/reservas/pagos/devoluciones/disputas/mensajes/cursores, auditoría y outbox antes y después; coinciden.

La comprobación del archivo fue sobre los bytes reales de respuesta HTTP del handler, no sobre un enlace presentado. El mock compila y su prueba de estado confirma que los bytes `application/zip` solo se entregan a la generación de sesión que los solicitó; no se ejecutó automatización visual de navegador en esta entrega.

## Límites

Esta evidencia acepta únicamente la exportación de lo actualmente implementado en el prototipo local. No completa la exportación general ni la supresión integral; #185/#40 permanecen abiertas. Las lecturas de módulos se componen sin mutaciones, pero no comparten una única transacción serializable entre dominios. Sin GCP y sin uso de la base persistente, secretos o datos locales para fixtures.
