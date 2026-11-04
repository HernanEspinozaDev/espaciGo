# F6 — recorrido nativo local completado, 2026-11-04

Rama exclusiva: `codex/frontend/fix/local-close-02-f6-native-e2e`; base `a50f62ce00fd8558abf5e1d321692d59e34ad863`. No se sobrescribió `frontend-next-hermes`, no se integró main ni se desplegó. El commit que contiene este documento identifica la fuente frontend probada; el recibo final del padre registra los refs remotos exactos de ambas ramas.

## Correcciones verificadas

- Catálogo y perfiles protegidos: Bearer explícito hook→fachada→puerto→adaptador; sin credenciales globales ni cookie fallback. Revisión independiente `deleg_04dfe5bc` PASS.
- `POST /spaces` no declara idempotencia en Go/OpenAPI: creación deja de enviar el header no admitido, sin abrir el BFF. Galería mantiene su clave real.
- El wizard envía únicamente atributos dinámicos y versión del perfil cargado; falta de perfil bloquea creación. Cancelación sigue como texto de normas, no atributo inventado. Depósito no soportado queda deshabilitado con explicación accesible. No se amplía backend/schema para hacerlo pasar.
- Spec nativo: inventario privado configurable solo por ruta canónica dentro del directorio privado, códigos de región/comuna válidos, selector Wi-Fi por etiqueta del perfil real y sufijo sintético para repetir con espacio nuevo.
- Envío HTTP200 persiste `enviada`, no inicia por sí solo revisión humana. Readback independiente exige ese estado exacto después de enviar.
- Descarga valida el stream del archivo realmente descargado, sin cancelarlo antes; `%PDF-` y `pdfinfo` verifican PDF A4 de una página. Se comprueban loopback y BFF same-origin por separado.

## Ejecución y límites

Frontend standalone reconstruido después del último cambio productivo. Next local `127.0.0.1:4203`, BFF→Go `127.0.0.1:18082/api/v1`, DB única `127.0.0.1:15434/espacigo_f5_gate_20261101` (`172.28.0.2`, PG18.6, V1–V83), filesystem cifrado y ClamAV real. No browser del usuario ni destinos online. Se reutilizaron exactamente las dos cuentas autorizadas, sin imprimir identidades o credenciales.

| Gate | Resultado real | Fuerza y límites |
|---|---|---|
| API generada/inventario | PASS, 185 operaciones vigentes | Contrato fuente, no integración |
| ESLint global, tsc, formato global, build | PASS | Artefacto standalone real |
| `node --test tooling/*.test.mjs` | 80 PASS, 0 FAIL/SKIP | Unidades, fakes y guardas fuente diferenciadas |
| Native `@fixture\|@local-http-worker-free` | 2 PASS, 0 FAIL/SKIP | Perfil API realmente worker-free; espacio nuevo sufijo `20261104-final1`, POST201, readback/reanudación y logout |
| Native `@f6` | 4 PASS, 0 FAIL/SKIP | Perfil scan-only sobre el MISMO espacio nuevo; fixture PDF, preflight, UI real crear caso→upload202→inspección→scanner→submit200→readback→descarga PDF→otro owner404→logout204 |
| Playwright global sin opt-ins nativos | 69 PASS, 48 SKIP, 0 FAIL | Incluye mocks/Axe y checks no gated; las 48 omisiones no son integración aprobada. Los opt-ins de este corte están acreditados en las dos filas anteriores |
| Regresión final del wizard/mock y captura | 1 PASS | Header ausente, atributos exactos del perfil, depósito disabled y descripción accesible; screenshot sintético recortado |

El primer intento global encontró un test mock antiguo que exigía Idempotency-Key. Se corrigió el test contra Go/OpenAPI (no la allowlist), se retiró el atributo no perteneciente al perfil del fixture y se repitió la suite global verde. Tras agregar únicamente aserciones/captura del depósito, se repitió ese test focalizado con lint/types/formato PASS.

## Matriz de aceptación local

- Borrador autenticado y persistido desde UI: aprobado.
- Catálogo/perfil reales y esquema exacto: aprobado.
- Carga, inspección y scanner real con outcome persistido: aprobado.
- Envío y rehidratación del caso: aprobado, estado enviada.
- Descarga visible de bytes PDF válidos: aprobado.
- Otro propietario denegado y logout: aprobado.
- Revisión/revocación, retención, permisos y concurrencia: prueba PostgreSQL committed/backend en la evidencia del padre `f5-f6-recovered-live-20261104.md`, no se atribuyen a clicks de reviewer en esta UI.

No implica aceptación humana/visual del producto, KYC real, acceso binario de reviewer, depósitos operativos, idempotencia de creación, pruebas cloud ni cierre F7. La etiqueta actual de comuna admite nombre aunque Go exige código: límite de UX separado, no permiso para inventar un catálogo o relajar la validación. F3/F4 conservan su evidencia histórica; se repitió la regresión compartida afectada, no migraciones ni operaciones cloud.

La captura `f6-deposit-unavailable-20261104.png` es solo del control sintético; inspección visual del padre confirmó ausencia de emails, UUIDs y credenciales. No representa revisión humana de todo el producto ni screenshot online.

## Revisión y cierre operativo

Revisión independiente final `deleg_0df92f8a`: `passed=true`, sin hallazgos de seguridad o lógica. Inspeccionó wizard, test nuevo y spec nativo contra Go/OpenAPI; fue source-only, no una repetición independiente del runtime. Su sugerencia no bloqueante es hacer más robusta/syntax-aware la extracción del objeto en el test de payload; no se modifica el producto por esa sugerencia.

El padre retiró API propia y forward temporal; readback de sockets confirmó ausencia de listeners en15434/18082/4203. La red temporal de egress del updater quedó eliminada, sin contenedores adjuntos. PostgreSQL y scanner se conservaron healthy para no perder datos/firmas tmpfs; Mailpit existente se preservó con sus datos, no se limpió. Responsable de conservación: este corte local hasta revisión humana de evidencia; no se autoriza detener o borrar esos recursos por inferencia. Backup y estado final están registrados en la evidencia backend.
