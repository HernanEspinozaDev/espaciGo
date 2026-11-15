# Revisión local del plan de identidad propio — 2026-11-10

## Alcance y resultado

La primera parte de esta evidencia registra la revisión documental, no implementación ni aceptación de producto. El seguimiento del 10 de noviembre continúa únicamente VI-01 en la UI de cuenta. El encargo es verificación propia; no se hicieron llamadas al proveedor, escrituras SQL, cambios de servicios/secretos o recursos cloud en este subcorte frontend.

Plan vigente: [Verificación de identidad EspaciGo](plan_verificacion_identidad_espacigo.md). Está propuesto para ejecución en cortes VI-00–VI-05; revisión humana es la recomendación mínima, no un sistema ya entregado ni una prueba biométrica certificada.

## Inventario y contraste parental

La inspección delegada identificó reutilización adicional, contrastada con fuente por el padre:

- `internal/documentverification/service.go`: `PurposeIdentity`, `review_identity`, estados y separación de espacios.
- `internal/documentverification/reviewer_access.go`: asignación/acceso de identidad y prohibición de auto-revisión.
- `internal/adapters/documentcodec/codec.go`: AES-GCM con AAD de identidad; conservarlo al añadir adapter GCS.
- `cmd/api/runtime_storage.go`: el almacén documental actual es filesystem cifrado; el GCS genérico no aplica el códec. No declarar existente el cableado documental cifrado en cloud.
- `internal/identityverification/service.go`: contrato mínimo manual con expiración/revocación; no relabelar resultados sintéticos o externos.

Se ajustó el plan para reutilizar casos y rutas existentes antes de proponer nuevas tablas/endpoints, mantener enums reales y añadir únicamente el vínculo de declaración/evidencias/resultado mínimo que falte.

## Observaciones de revisión independiente y adjudicación

1. **Puntero supuestamente ausente:** el revisor buscó el nombre descriptivo `legacyplanpointer/README`, que no es un archivo asignado. Las rutas reales son `planning/kyc_didit/README.md` y `planning/kyc_didit/plan_kyc_didit.md`. El padre leyó el README y verificó sus enlaces: ambos retiran la autoridad operativa anterior y dirigen al plan nuevo. No se inventó una carpeta para satisfacer ese localizador equivocado.
2. **Retiro frente a conservación:** §10 ahora separa expresamente el MVP sin imports/llamadas/fallback externos de la custodia residual de datos/colas/migraciones/recibos. La conservación de historial o capacidad local no convierte al proveedor en dependencia del MVP. VI-01 exige prueba de cero tráfico retirado, sin afirmar que el código se eliminó ya.
3. **Significado de borrado:** §7 distingue revocación de acceso, ausencia de generaciones activas y fin de la recuperabilidad de copias. No se afirma eliminación total por un ACK/404. Los plazos y política efectiva del bucket deben ratificarse/verificarse antes de capturar datos reales; ese es un gate posterior de operación, no una configuración aplicada por el plan.

La revisión independiente no aprobó su primera lectura. Las observaciones se resolvieron/adjudicaron localmente por el padre; no se atribuye al revisor una re-aprobación posterior ni al usuario aceptación del producto.

## Comprobaciones y conservación

- Plan activo sin marca/requisitos del proveedor retirado; referencias a fuente existentes y enlaces relativos comprobados mediante Python.
- Plan anterior íntegro en `planning/kyc_didit/historico_plan_proveedor_hasta_20261110.md`: SHA256 `42adc4469e92f02421e3d0dac8b7b84b5c3df2ba4b5740fcc55100b8d571ba87`, igual antes y después de archivarlo.
- Índice de planning y puntero anterior conducen al plan nuevo. Preparación de release anterior marcada histórica.
- En seguimiento VI-01: mensaje de la tarjeta de cuenta reemplazado por estado neutral y comprobado con Playwright local en `tests/didit-account-status.spec.ts`; 4/4 pruebas pasan (tarjeta, retorno legado/query strip, pantalla de declaración y rechazo BFF). El test usa BFF/API sintética; no prueba integración con Go/DB.
- `eslint` de los dos archivos editados, `tsc --noEmit`, Node `tooling/*.test.mjs` (490 pasan, 1 omitida), Prettier de archivos editados y `next build`: PASS. El lint completo no pasa por error preexistente en `tooling/account-canonical-navigation.test.mjs:19` (`no-assign-module-variable`); avisos existentes adicionales no corregidos. `pnpm` no está disponible en PATH; los binarios locales `node_modules/.bin` se usaron directamente. El test E2E corrió sobre el servidor standalone local en puerto de ensayo 4203, sin perfil de usuario ni DB.
- Traza de siguiente brecha, sin editar código: `POST /document-verification/cases/{id}/files` define solo multipart `file`; `OwnerCasesHandler.uploadFile`, `UploadService.Upload`, `Inspector.Inspect`, `documentverification.File` y las lecturas/escrituras de `internal/adapters/postgres/documentverification/repository.go` no llevan rol/tipo de evidencia. `ValidateQuota(PurposeIdentity)` limita a tres archivos como máximo; `Submit` exige archivos limpios no vacíos, no frente+reverso+selfie distintos. Por tanto, la UI/API actual no puede asegurar el conjunto de identidad que el plan recomienda.
- Verificación backend sin conectarse a DB: con variables opt-in de integración y `PGPASSFILE` explícitamente retiradas del entorno, `go test -count=1 ./cmd/api ./internal/documentverification/... ./internal/adapters/postgres/documentverification/... ./internal/verification/transport/http/...`: PASS. Los casos PostgreSQL opt-in no se ejecutaron. `go build -o /dev/null ./cmd/api`, `go vet` sobre los mismos paquetes y `git diff --check`: PASS.
- No se ejecutaron migraciones, suites Go opt-in con PostgreSQL, operaciones contra la DB persistente ni cloud en este subcorte.
- Rama conservada `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`, con cambios preexistentes preservados. Sin commit/push/main ni despliegue.

## Pendiente deliberado

Ratificar el modo de revisión humana propuesto, custodios/revisores, retención y privacidad antes de captura real. VI-01 sigue abierto hasta cerrar sus gates de persistencia, ownership, concurrencia, vínculo inmutable declaración→caso y resultado mínimo; luego se requieren VI-02–VI-05. Este cambio pequeño de copy y sus pruebas no habilitan captura propia ni cierran el flujo. La traza confirma que roles frente/reverso/selfie necesitan un delta append-only posterior a V94; por orden del plan pertenecen a VI-02, no se improvisan en UI ni se presenta el máximo de tres como validación semántica. Ningún SQL fue escrito ni aplicado en este seguimiento. Configuración KMS/bucket, permisos, borrado real e integración BFF→Go/DB todavía necesitan implementación y evidencia propias.

## Integración documental: cédula física anterior y nueva

A petición del usuario se integró al flujo la distinción entre formatos físicos sin activar OCR/MRZ/QR ni reconocimiento automático. Se consultaron el 2026-11-10 fuentes oficiales del Consulado de Chile, BCN/Ley Chile y Diario Oficial; referencias S6–S9 y alcance están registrados en el plan activo.

- El flujo pide formato declarado anterior al 16-12-2024, nuevo desde esa fecha o «No estoy seguro»; guía ambas caras y mantiene selfie como evidencia separada.
- El revisor confirma formato observado y compara fecha de emisión/vencimiento; discrepancia o incertidumbre deriva a revisión/corrección, no a aprobación automática.
- Se registra como requisito que las cédulas anteriores continúan siendo admisibles hasta su vencimiento. Los marcadores QR, MRZ y chip no sirven por sí solos para separar los diseños: ya aparecían en la resolución del formato anterior. La nueva tiene señales visuales descritas oficialmente (retrato principal láser B/N, retrato secundario SLI con año de nacimiento y contactos visibles del chip); son ayudas de orientación, no prueba de autenticidad.
- Nacionalidad/variante de extranjero queda separada de la antigüedad. La cédula digital se deja fuera de la captura de tarjetas físicas; como regla propuesta del MVP, una captura estática no sustituye un flujo de verificación digital.
- Solo se actualizaron `plan_verificacion_identidad_espacigo.md` y esta evidencia. Es diseño/documentación, no captura implementada, nuevo contrato OpenAPI, esquema aplicado ni aceptación legal.

## VI-02 — cierre del contrato y apertura del corte (2026-11-10)

Por instrucción expresa del usuario, se cerró el contrato mínimo en §8 del plan y se abrió VI-02 para desarrollo local sintético. `planning/openapi.yaml` ahora expresa los campos de formato declarado/observado, los roles de evidencia, el reemplazo de slots con CAS, la exigencia de los tres roles para enviar y el DTO específico de revisor. El contrato queda congelado; su presencia en OpenAPI no significa que ya esté implementado en Go, persistencia o UI.

- Datos: `document_format_declared` = `legacy|new|unsure`; `document_format_observed` = `legacy|new|undetermined`; `evidence_role` = `identity_front|identity_back|identity_selfie`.
- El servidor debe ligar el caso a la revisión legal vigente del titular; no acepta su ID desde cliente. La purga no puede borrar una revisión que sostenga un caso activo.
- Crear identidad no solicita vencimiento al usuario; el revisor envía `document_expires_at_observed`, que se almacena como `document_expires_at` y solo sale en el DTO del revisor. `undetermined` no puede aprobar.
- La carga reemplaza el slot del mismo rol mediante `If-Match`; el anterior queda inaccesible y en retención de borrado. Enviar exige exactamente los tres archivos limpios; solo JPEG/PNG son válidos para identidad. Los archivos históricos no se etiquetan retroactivamente.
- Evidencia al cierre de esta continuación: `planning/openapi.yaml` parseó con el paquete `yaml` del proyecto y `node tooling/api-check.mjs` confirmó contratos/inventario actuales (199 operaciones). El dominio, upload tipado, DTO/rutas HTTP de titular y revisor, y derivación transaccional de la revisión legal del titular están implementados parcialmente en Go. Se corrigió la compatibilidad multipart de `PurposeSpace` cuando solo envía `file`; el test focal y las suites del paquete pasan.
- Verificación local, con variables de DB opt-in y `PGPASSFILE` retiradas: `go test -count=1 ./internal/documentverification/... ./internal/adapters/postgres/documentverification/... ./internal/verification/transport/http/...` PASS; la misma suite con `-race` PASS; `go build -o /dev/null ./cmd/api` PASS; `go vet` de los paquetes afectados PASS; `git diff --check` PASS. Estos resultados no incluyen una base real: la integración PostgreSQL opt-in se omitió al no estar configurada. No hubo DDL, conexión a la DB persistente, cloud ni navegador.
- Archivos propios del subcorte VI-02 en este árbol: `internal/documentverification/{service.go,service_test.go,upload.go,upload_test.go,reviewer_access.go,reviewer_access_test.go,authorization_parent_test.go}`, `internal/documentverification/transport/http/{owner_cases.go,owner_cases_test.go,owner_upload_test.go,reviewer_cases.go,reviewer_cases_test.go}`, `internal/adapters/postgres/documentverification/{repository.go,repository_test.go,upload_integration_test.go}`. El árbol contiene además cambios heredados de otros cortes; no se atribuyen aquí.
- **VI-02 sigue abierto.** Falta V95 append-only y su registro/harness; persistir/cargar en PostgreSQL los formatos, roles y observación del revisor; hacer durable la sustitución de archivos con ocultación inmediata, retención y borrado encolado; proteger la purga de revisiones enlazadas a casos activos y liberar el vínculo al cerrar; integrar codec AES-GCM con GCS y completar la experiencia visible. El código actual deriva el ID de declaración dentro de `CreateCase`, pero no lo persiste aún; roles y metadata de revisión tampoco tienen persistencia demostrada. No se aplicó SQL ni se afirmó recorrido Go→DB funcional.
- Rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD observado `274f37e483a078890e92382a159fa49fd23c2fe8`; worktree compartido y dirty preservado. Sin commit/push, integración a `main` ni despliegue. La aceptación del producto y captura de datos reales permanecen pendientes.
