# Evidencia — VI-03 / VI-04, primitivas léxicas de inspección, descarga segura y transporte de revisor

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc3459c55b6ef2220d91244ea1d9da7a5fb`.
- **Entorno de ejecución:** Entorno local aislado con fixtures sintéticos en memoria. Sin acceso ni llamadas a Cloud Storage, Secret Manager, Cloud SQL, bases PostgreSQL persistentes ni DDL/SQL.
- **Archivos de pruebas añadidos / modificados:**
  - `internal/documentverification/inspection/inspector_primitives_test.go` (nuevo).
  - `internal/documentverification/download_test.go` (ampliado).
  - `internal/documentverification/transport/http/reviewer_cases_test.go` (ampliado).

## 1. Alcance cubierto

1. **Primitivas de validación de imágenes y Poppler (`inspection/inspector_primitives_test.go`)**:
   - `jpegOrientation`: entradas truncadas, secuencias sin marcador `0xff`, marcadores terminales EOI (`0xd9`) y SOS (`0xda`), tamaños inválidos (<2) y desbordamiento de búfer, rechazo explícito (retorno 0) ante metadatos Exif (`0xe1` con `Exif\x00\x00`), y aceptación (retorno 1) ante otros marcadores APP sin Exif.
   - `hasZeroEmbeddedPDFFiles`: aceptación estricta de `0 embedded files` con insensibilidad a mayúsculas/espacios; rechazo ante recuentos distintos de cero, cadenas truncadas, palabras adicionales o entrada vacía.
   - `hasEmptyPDFInfoURLList`: aceptación de cabeceras canónicas `Page Type URL` e insensibilidad a mayúsculas/espacios/saltos de línea; rechazo ante columnas faltantes, presencia de URLs en líneas subsecuentes, cadenas vacías o solo espacios en blanco.
   - `trustedPDFExecutable` y `trustedPDFDirectories`: rechazo de rutas relativas, rutas con redundancias no canónicas, nombres base discrepantes y ejecutables o directorios inexistentes.

2. **Primitivas del escáner estructural de PDF (`inspection/inspector_primitives_test.go`)**:
   - `positiveIntegerEntry`: rechazo fail-closed (`ErrRejected`) ante clave no encontrada, valor no entero, entero cero o entero negativo; retorno exacto del valor ante entero positivo válido.
   - `nameEntry`: distinción entre clave no encontrada (`"", false, nil`), valor no tipado como Name (`ErrRejected`) y nombre válido (`Name("Form")` -> `"Form", true, nil`).
   - `validateImagePixelLimit`: rechazo ante límite configurado nulo, ausencia de dimensión `Height` o desbordamiento de píxeles (`width > maxImagePixels/height`); aprobación ante dimensiones válidas dentro de presupuesto.
   - `validateFormInlineImagePixelLimit`: tolerancia ante subtipos distintos de Form; rechazo ante subtipo Form con tabla xref nula.
   - `dereference`: paso directo sin error para objetos que no son referencias indirectas.

3. **Descarga segura y discrepancia de tamaño (`download_test.go`)**:
   - Verificación de que discordancias entre el tamaño físico leído del almacén y el tamaño registrado en el snapshot (`len(content) != selected.Size`) se mapean a `ErrDocumentStorage`.

4. **Transporte HTTP de revisor y cancelación de contexto (`transport/http/reviewer_cases_test.go`)**:
   - Manejo fail-closed de cancelación de contexto en `review` retornando HTTP 503 (`StatusServiceUnavailable`).
   - Manejo fail-closed de cancelación de contexto en `revoke` retornando HTTP 503 (`StatusServiceUnavailable`), llevando la cobertura de declaraciones de `revoke` al **100,0%**.

## 2. Resultados de ejecución local registrada

```bash
$ go test -race -v -run 'Test(JPEGAndPDFInfoValidationPrimitives|PDFStructureScannerPrimitiveEntries)' ./internal/documentverification/inspection
=== RUN   TestJPEGAndPDFInfoValidationPrimitives
--- PASS: TestJPEGAndPDFInfoValidationPrimitives (0.00s)
=== RUN   TestPDFStructureScannerPrimitiveEntries
--- PASS: TestPDFStructureScannerPrimitiveEntries (0.00s)
PASS
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection	1.034s

$ go test -race -v -run '^TestSpaceDownloadStorageReadErrorAndCaseMismatch$' ./internal/documentverification
=== RUN   TestSpaceDownloadStorageReadErrorAndCaseMismatch
--- PASS: TestSpaceDownloadStorageReadErrorAndCaseMismatch (0.00s)
PASS
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification	1.017s

$ go test -race -v -run '^TestReviewerCasesHandlerSecurityBoundariesAndSpaceReview$' ./internal/documentverification/transport/http
=== RUN   TestReviewerCasesHandlerSecurityBoundariesAndSpaceReview
--- PASS: TestReviewerCasesHandlerSecurityBoundariesAndSpaceReview (0.00s)
PASS
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http	1.024s

$ go test -race -count=1 ./internal/documentverification/...
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification	1.049s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection	15.416s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http	1.410s

Total de statements de todo el subsistema documentverification: 93.8%
```

## 3. Conformidad con directivas y límites

- Cobertura global del subsistema `documentverification/...` incrementada a **93,8%**.
- Paquete `inspection` subió a **86,3%** (`hasZeroEmbeddedPDFFiles` al 100%, `hasEmptyPDFInfoURLList` al 100%, `jpegOrientation` al 93,8%).
- En `reviewer_cases.go`, la función `revoke` alcanzó el **100,0%**.
- Cero DDL/SQL ejecutado.
- Cero acceso a bases persistentes o Cloud Storage.
- `git diff --check`: limpio.
