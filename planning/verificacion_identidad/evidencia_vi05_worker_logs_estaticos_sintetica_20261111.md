# Evidencia — VI-05, mensajes estáticos del worker de retención

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios previos preservados.

## Alcance

- Se extrajo `runDocumentRetentionWorker` y se conectaron a él las dos rutas de startup que ejecutan el worker de retención. Las cadenas de log siguen siendo constantes y el callback descarta el contenido de los errores de pase y de detención.
- `TestDocumentRetentionWorkerLogsOnlyStaticFailureMessages` usa un runner fake que entrega errores sintéticos con marcador de ruta privada tanto al callback como al retorno. Comprueba intervalo de un segundo, mensajes exactos estáticos y que no aparece el error crudo.
- Es cobertura del helper productivo de logging con runner fake; no es una captura de logs de producción ni una revisión de todos los sinks.

## Verificación local

- `go test ./cmd/api -run '^TestDocumentRetentionWorkerLogsOnlyStaticFailureMessages$' -count=1`: PASS.
- El mismo test con `-race`: PASS.
- `go vet ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluidos los archivos nuevos: PASS.
- Variables DB/PG y opt-in del integration gate retiradas en comandos Go.

## Límites

La revisión independiente `deleg_af8af875` aprobó sin hallazgos; confirmó que ambos callsites productivos usan el helper y que las conclusiones están acotadas a mensajes estáticos con runner fake. Reiteró que esto no demuestra privacidad global de logs. La prueba confirma únicamente los mensajes emitidos por este helper ante ese runner; no demuestra ausencia global de datos sensibles en logs, traces, métricas, volcados, exportaciones u otros procesos. Tampoco prueba ejecución del worker productivo, DB/Cloud, durabilidad, persistencia o borrado. Sin SQL/DDL, conexión a bases, Cloud, proveedor, navegador ni datos reales.
