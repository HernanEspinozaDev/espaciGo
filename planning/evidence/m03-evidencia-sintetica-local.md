# M03 — evidencia sintética local (#47)

## Límite y almacenamiento

El único elemento cargable es un PNG fijo generado por la API al recibir `fixture_code: synthetic-png-v1`; el cliente no envía bytes ni puede elegir un archivo. PostgreSQL conserva código de fixture, tipo, tamaño, SHA-256 y fechas. El archivo se escribe bajo `$XDG_DATA_HOME/espacigo/m03-evidence` en el host o, si `XDG_DATA_HOME` no está definido, bajo `~/.local/share/espacigo/m03-evidence`; directorio `0700` y blobs `0600`. El contenedor solo lo monta en el backend (`/var/lib/espacigo/private-evidence`); el mock y rutas públicas estáticas no acceden al directorio. `scripts/dev-env.sh config` o `up` crea/valida el directorio sin borrarlo.

No subir RUT, identificaciones, fotos, selfies ni datos reales. No hay proveedor productivo, revisión documental, plazo de retención ni flujo de borrado legal; siguen pendientes en Issue #142. El fixture no acredita identidad ni concede permisos comerciales.

## Recorrido manual

1. `scripts/dev-env.sh up` aplica únicamente migraciones pendientes y reutiliza `espacigo_pgdata` y secretos existentes.
2. Abre `http://localhost:8081`, crea/verifica una cuenta de prueba e inicia sesión.
3. Crea un caso KYC o KYB sintético. Copia su UUID en “Caso propio para evidencia”.
4. Pulsa “Crear fixture privado” y luego “Consultar evidencias”. “Consultar PNG” descarga el recurso a la vista previa mediante la API autenticada. Un ID ajeno debe responder `404`; un token sin sesión `401` y una cuenta sin rol administrador no accede a rutas de revisión (`403`). “Probar rechazo 422” envía un código fixture inválido y confirma que no se almacena.
5. Para limpiar, pulsa “Eliminar fixture” (titular) o usa `DELETE /api/v1/admin/verifications/{caseId}/evidence/{evidenceId}` con rol administrador. Esto borra el blob y su fila de metadatos. No uses `clean`, `docker compose down --volumes` ni borres `espacigo_pgdata`/secretos/directorio completo para limpiar fixtures.

La API OpenAPI está publicada localmente en `http://localhost:8080/openapi.yaml`. Todas las respuestas llevan `Cache-Control: no-store`; no se emiten tokens ni se escriben blobs en logs.

La persistencia/aislamiento PostgreSQL se puede repetir sin tocar la base del prototipo: `bash scripts/test-m03-evidence-postgres.sh` crea una base temporal dentro de un PostgreSQL desechable y la elimina al terminar. El recorrido automático de API comprueba `401`, `403`, `404`, listado, descarga y eliminación con identidades sintéticas.
