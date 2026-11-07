# Ensayo nativo: proyección semanal y exclusión pública

## Alcance verificado por el revisor

Worktree: `espaciGo-wizard-improvements`; rama `codex/frontend/fix/mejoras-creacion-espacios`; HEAD base `771134d165046b290b1f5ad3a1074a6357c2327f`. Las fuentes están modificadas localmente; este SHA no identifica una implementación publicada.

Se ejecutó desde la raíz del worktree:

```sh
ESPACIGO_WIZARD_PUBLIC_SQL_READ=1 \
PGPASSFILE="$PWD/.local/testing/wizard-improvements/espacigo_api_login.pgpass" \
go test ./internal/adapters/postgres/publiccatalog \
  -run '^TestWizardNativeDetailReadOnlyDraftExcludedFromPublicCatalog$' -count=1 -v
```

Resultado independiente: **PASS**, incluido después del cambio persistido del cierre del lunes a 17:30. La prueba fija el destino `espacigo_wizard_20261106` en loopback, verifica `session_user=espacigo_api_login`, `current_user=espacigo_runtime` y `default_transaction_read_only=on`. No crea fixtures ni modifica elegibilidad, roles, esquema o migraciones.

- El identificador se obtiene del recibo privado verificado; no se incluye un UUID fijo en la prueba versionada.
- La expresión SQL correlacionada es la misma constante privada utilizada por `detailQuery()`.
- Dentro de una transacción `REPEATABLE READ READ ONLY`, compara el DTO completo con los tramos almacenados: siete días ISO, días vacíos, orden y horas/minutos de todos los períodos.
- Por separado, `Detail` devuelve `ErrNotFound` para el borrador, conservando su exclusión del catálogo público.

La primera ejecución real detectó SQLSTATE `42601` cerca de `ELSE`: faltaba cerrar `jsonb_build_object`. Se corrigió la sintaxis y se repitió la ejecución real, no sólo las pruebas estáticas.

## Regresión conjunta Go

El revisor ejecutó `go test ./...` sobre las fuentes actuales: **PASS**. Log local: `/home/nandev/.hermes/cache/scratch/wizard-go-global-current.log`. La prueba nativa es opt-in y se ejecutó además explícitamente como se describe arriba; el global sin flag no la sustituye. `git diff --check`: PASS.

## Límites vigentes

La consulta de prueba de la proyección selecciona el borrador propio directamente; no lo convierte en una fila pública elegible ni demuestra una respuesta HTTP anónima positiva. La ejecución de `Detail` acredita sintaxis y exclusión, no el recorrido positivo con todos los filtros de publicación. Ese criterio continúa pendiente; no se alterarán KYC, publicación ni otros estados para fabricar una aprobación.

La edición nativa del lunes se confirmó por readback independiente. Su recorrido original falló después de guardar por una expectativa incorrecta sobre el ETag de una configuración de zona horaria sin cambios; no se repitió el guardado. Posteriormente, el revisor ejecutó el caso separado `reconciles the saved hourly schedule and rejects stale CAS`: **1 passed**, con rehidratación tras recarga, rechazo 412 del ETag anterior observado y readback intacto. La lectura independiente del recibo confirmó `state=verified` y las marcas `readback`, `reload` y `stale412`. Véase `wizard-native-editor-cas.md`. Esto no convierte el recorrido original en una ejecución limpia.

No hay commit, push, integración a main, despliegue ni aceptación online acreditados de este conjunto por esta evidencia.
