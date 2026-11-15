# VI-02 — formatos de evidencia de identidad

- Fecha de verificación local: 2026-11-12.
- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- Rama/HEAD comprobados: `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Estado: cambios locales sin commit; se conservaron los cambios preexistentes del worktree compartido.

## Cambio y regresión

En `internal/documentverification/upload.go`, para casos de identidad tipada (con `document_format_declared`), el servicio ahora rechaza el MIME inspeccionado salvo `image/jpeg` o `image/png`, antes de llamar al almacenamiento privado. Esto evita el recorrido previo donde un PDF inspeccionado alcanzaba `PutIdentityDocument` y después era retirado por la validación transaccional de `Service.AddFile`.

Se añadió `TestTypedIdentityUploadRejectsPDFBeforeStorage` en `internal/documentverification/upload_test.go`. La prueba se ejecutó primero contra el código anterior y falló por el límite correcto: el fake registró `putCalls:1` y `deleteCalls:1`; después de la guarda, pasó y afirma cero escrituras/borrados. Usa solo metadatos y contenido sintéticos.

## Límites aprobados revisados en fuente

- `MaxUploadBytes` es 10 MiB; el upload y el inspector rechazan valores mayores. El handler multipart limita el archivo individual y permite margen acotado adicional para el envelope de la solicitud.
- El inspector limita el tamaño a 10 MiB, revisa la cabecera con `image.DecodeConfig`, limita píxeles mediante `MaxPixels`, decodifica la imagen y normaliza JPEG/PNG. El valor predeterminado actual de `MaxPixels` es 40.000.000; este valor de implementación no se presenta como una decisión de producto adicional.
- Los roles tipados son `identity_front`, `identity_back` e `identity_selfie`; el servicio exige uno de ellos y rechaza duplicados al validar el snapshot. La aprobación exige exactamente un archivo limpio por rol.
- Las pruebas existentes del inspector cubren formato no admitido, exceso de bytes y pixel bomb. No se cambió el inspector, el límite, la política de roles ni el flujo de espacio/identidad histórica en este subcorte.

## Verificación ejecutada

- RED/GREEN: `go test ./internal/documentverification -run '^TestTypedIdentityUploadRejectsPDFBeforeStorage$' -count=1` — RED esperado antes del guard; PASS después.
- `env -u DATABASE_URL -u DATABASE_DSN -u DB_DSN -u TEST_DATABASE_URL -u PGHOST -u PGPORT -u PGDATABASE -u PGUSER -u PGPASSFILE go test -race ./internal/documentverification ./internal/documentverification/transport/http ./internal/documentverification/inspection -count=1` — PASS (tres paquetes).
- El mismo aislamiento de variables con `go vet` en los tres paquetes — PASS; `go build -o /dev/null ./cmd/api` — PASS.
- Sin acceso a DB/Cloud, SQL/DDL, migraciones, almacenamiento productivo, proveedor, datos reales, push, deploy o integración a `main`.

## Límite de evidencia

La prueba nueva usa un inspector fake y un almacenamiento fake: prueba el rechazo antes de escribir, no el recorrido integrado de navegador→BFF→Go→storage, la inspección en producción, PostgreSQL ni GCS. La fase VI-02 permanece parcial hasta cubrir esos gates por separado.
