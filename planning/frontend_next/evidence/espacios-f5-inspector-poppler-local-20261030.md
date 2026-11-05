# F5 — Inspección PDF local estructural con pdfcpu y Poppler

**Fecha:** 2026-10-30

**Checkout:** `/home/nandev/Documentos/projects/espaciGo`

**Rama / HEAD:** `codex/backend/feature/local-close-02-espacios-galeria-documentos` / `3723e51e0b3d7658915c753163b43c6f705e9e17`
**Estado:** pruebas locales pasan tras correcciones de revisión; la inspección completa F5/F6 sigue abierta.

## Implementación

- `internal/documentverification/inspection/pdf_structure.go` usa `pdfcpu` v0.16.2 (`api.ReadAndValidate`) con configuración stateless, validación estricta/offline y límites de recursos. Recorre la tabla xref y resuelve objetos comprimidos para detectar acciones PDF, formularios y adjuntos; rechaza cifrado, fallos de parseo y discrepancias de páginas. El test de `Launch` comprimido verifica que el parser clasifique la estructura activa, no que el caso falle incidentalmente.
- `internal/documentverification/inspection/inspector.go` conserva rutas absolutas explícitas para Poppler: `/usr/bin/pdfinfo` y `/usr/bin/pdfdetach`; se rechaza la resolución implícita por `PATH`. `pdfinfo` debe reportar `Pages`, `Encrypted`, `JavaScript` y `Form`; además, `pdfinfo -js` y `pdfinfo -url` deben devolver resultados válidos sin contenido prohibido. El listado URL admite columnas adicionales en su encabezado, pero rechaza cualquier fila de datos. `pdfdetach -list` acepta únicamente el conteo tokenizado exacto `0 embedded files` (no una búsqueda por subcadena). Cada ejecución Poppler tiene deadline de 5 s y límites de 1 MiB por stdout y stderr; salidas incompletas, stderr, timeout, exceso de salida, PDF cifrado o parser fallido se rechazan.
- `pdfcpu` recibe bytes en memoria (`bytes.Reader`); `pdfdetach` recibe un descriptor anónimo `/proc/self/fd/3` creado con `memfd`. No se crean PDFs temporales en texto claro. Los PDFs de prueba se generan sintéticamente en memoria; los scripts de parser de prueba sí usan archivos temporales.
- La normalización y cuotas de imágenes no cambiaron: PNG/JPEG se decodifican y recodifican; el SHA-256 corresponde a los bytes normalizados.

## Pruebas ejecutadas

- `go test -mod=readonly ./internal/documentverification/inspection ./internal/adapters/documentfs -count=1` — PASS. Cubre PDFs sintéticos válidos limpios, cifrados, con JavaScript/OpenAction, AcroForm, URI, Launch (incluido objeto comprimido) y adjunto; parser estructural malformado/fallido; conteo de adjuntos de varios dígitos; salida Poppler excesiva y timeout; rutas absolutas; imagen normalizada, propósito/ID, tamaño y píxeles.
- `go vet -mod=readonly ./internal/documentverification/inspection ./internal/adapters/documentfs` — PASS.
- `go mod verify` — PASS (`all modules verified`); `git diff --check` — PASS; `gofmt -l` de los tres archivos de inspección — sin salida.
- Verificación: `go list -m github.com/pdfcpu/pdfcpu` → `v0.16.2`; `/usr/bin/pdfinfo` → Poppler 26.01.0; `/usr/bin/pdfdetach` existe. No se confirmó versión independiente de `pdfdetach`. No se instalaron paquetes del sistema.

## Límites vigentes

Esta evidencia cubre el inspector estructural local con pdfcpu/Poppler y bytes sintéticos; no demuestra carga vía API, persistencia cifrada, escaneo de ClamAV, revisión manual, descarga autorizada, expiración ni retención. qpdf/mutool no están disponibles; la inspección AST se realiza mediante la biblioteca pdfcpu v0.16.2, no mediante un ejecutable qpdf. Los tests que requieren Poppler se omiten en entornos donde no estén disponibles; en este entorno las rutas `/usr/bin/pdfinfo` y `/usr/bin/pdfdetach` estuvieron disponibles durante la suite. El recorrido runtime→API→BFF/UI→navegador sigue abierto.

## Hallazgos de revisión independiente y corrección

- La revisión `deleg_e881156e` señaló falta de presupuesto/deadline durante el recorrido del AST, valores estructurales malformados y cobertura Poppler condicional. La fuente revisada ya rechazaba variantes de objeto no soportadas por el `default` de `walk`, y `nameEntry` ya rechazaba valores no-nombre; se confirmó el hueco concreto: `walk` no comprobaba la expiración del contexto entre nodos y trataba como ausente un valor `nil` presente en `/Type`, `/Subtype` o `/S`.
- Se añadió comprobación del contexto compartido en cada nodo y durante el accounting de object streams, un máximo de 500.000 nodos por recorrido y rechazo de `nil` presente en los campos de nombre sensibles y en las entradas lazy. El contexto/deadline se crea y comprueba antes de iniciar el decode lazy, y se verifica otra vez después. pdfcpu `GetData()` no acepta contexto para cancelar un decode ya iniciado; su salida sigue limitada a 128 MiB por stream y 256 MiB acumulados. Este límite de cancelación interna queda explícito y se mantiene bajo revisión; no se amplía la lista de estructuras aceptadas.
- RED observado: `TestPDFStructureScannerStopsWhenContextIsCancelled`, `TestPDFStructureScannerEnforcesNodeBudget` y `TestPDFStructureScannerRejectsNilSecurityNameValues` fallaron antes de sus respectivas correcciones por aceptar el recorrido cancelado, una matriz de 600.000 nodos y valores nil sensibles. `TestPDFStructureScannerRejectsNilLazyObjectInObjectStream` reprodujo un panic por puntero nil; `TestPDFStructureScannerStopsObjectStreamAccountingWhenContextIsCancelled` reprodujo accounting posterior a la cancelación; `TestPDFStructureScannerDoesNotDecodeLazyObjectAfterContextCancellation` reprodujo un decode iniciado con contexto cancelado. Tras las guardas, los seis casos pasan.
- Verificación posterior: `go test -mod=readonly -timeout=120s ./internal/documentverification/inspection -count=1`, `go vet -mod=readonly ./internal/documentverification/inspection`, `go mod verify`, `git diff --check` y `gofmt -l internal/documentverification/inspection` — PASS.
- La recomendación de que CI fije Poppler sigue como límite de configuración de entorno; no se alteró infraestructura CI ni se instalaron paquetes. La suite local sí ejecutó en este entorno con las rutas Poppler configuradas.

## Límite de decode pdfcpu previo al scanner — 2026-10-30

- La revisión independiente `deleg_2eb1b40c` verificó que `api.ReadAndValidate` puede alcanzar decodificación lazy de object streams antes de `pdfStructureScanner`; el scanner sí limita sus propios object streams por stream y a 256 MiB agregados, pero ese contador inicia después de `ReadAndValidate` (`pdf_structure.go:90-100`). `GetData` puede volver a consultar el stream; con `Content` ya materializado no necesariamente lo descomprime de nuevo.
- `validatePDFStructure` ya pasaba `conf.Limits.MaxDecodeBytes=128 MiB` a `ReadAndValidate` (`pdf_structure.go:64-90`), pero el pdfcpu v0.16.2 usado anteriormente no respetaba el límite para contenido cacheado/catalog metadata. Se agregó `TestPDFReadAndValidateRejectsCatalogMetadataAboveDecodeLimit`, con metadata XML sintética de 128 KiB comprimida y cap de 4 KiB: RED observado en v0.16.2 (`ReadAndValidate error=<nil>`); tras actualizar a pdfcpu v0.16.3, la misma prueba pasó.
- Se actualizó pdfcpu a v0.16.3, upstream security release para bypasses de límites de bytes decodificados (GHSA-jp42-qm45-2x6g), y sus dependencias requeridas. `go list -m` confirmó `github.com/pdfcpu/pdfcpu v0.16.3`; `go mod verify` pasó. La release documenta aplicación del límite configurado en decodificación de streams y contenido combinado de página; esto no demuestra un presupuesto global para todos los object streams distintos procesados durante `ReadAndValidate`. No se declara resuelto ese límite agregado previo al scanner.
- Verificación: suite Go relevante con ambos DSN unset pasó; `go vet`, `gofmt -d`, `git diff --check` y `go mod verify` pasaron. No hubo conexión DB ni DDL. Revisión independiente del upgrade y cobertura `deleg_79a03ea6` pendiente; el inspector F5 continúa parcial mientras no se demuestre un límite agregado para la etapa pre-scanner.

## Reconciliación posterior — 2026-10-31

- El cambio posterior reserva presupuesto antes de `api.ReadContext` y la suite de `internal/documentverification/inspection` pasó. La revisión de pdfcpu v0.16.3 confirma que la lectura procesa xref/object streams antes de devolver el contexto; el contador agregado propio no contiene esos decodes. Las pruebas de metadata verifican límites individuales/combinados de pdfcpu, no una cuota global para todos los streams del documento.
- Poppler conserva ejecución por descriptor fijado y los tests del paquete pasan. Esto no cierra el riesgo de CPU/memoria durante la lectura inicial síncrona ni acredita límites de proceso.
- F5 sigue parcial. Antes de aceptar el inspector se necesita una cuota compartida antes de cada decode (hook/fork), otro parser controlable o aislamiento de proceso con límites efectivos, y fixtures multistream que cubran exceso agregado, límite exacto y rechazo antes del scanner. No hubo DDL ni prueba autenticada por este cambio.
