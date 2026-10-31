# F4 — integración local de galería con PostgreSQL y storage privado

**Corte:** 2026-10-29. **Rama:** `codex/backend/feature/local-close-02-espacios-galeria-documentos`. **HEAD:** `3723e51e0b3d7658915c753163b43c6f705e9e17` más los cambios locales sin commit.

## Target y límites

- PostgreSQL sintético F6 ya existente: `127.0.0.1:15432/espacigo_f6_synth_20261028`, V1–V78; se comprobó identidad y conectividad en transacciones de solo lectura. No se recreó ni se migró la base.
- `mediafs` usó el storage privado sintético del contenedor, validado como `0700:postgres:postgres`; el proxy temporal a `127.0.0.1:15432` se inició solo para la prueba y se detuvo al terminar.
- El test configura el dueño mediante un authenticator sintético, ejecuta el handler Go real y usa PostgreSQL/filesystem reales. El pool del test tiene identidad `postgres` para crear/limpiar fixtures; esto no prueba permisos de runtime ni login browser/BFF. F6 mantiene ese gate separado.
- Solo fixtures de imagen sintéticos. No se ejecutó DDL ni se usaron cuentas/documentos reales, Cloud, datos persistentes, publicación o deploy.

## Cobertura real ejecutada

`TestSpacePhotoReplacementAtomic` pasó en una ejecución final (`f4_gallery_integration_exit=0`) después de `go test`, `go vet`, compilación del test de integración y `git diff --check` satisfactorios. La prueba contra PostgreSQL/storage verificó:

- Carga multipart, variantes WebP en storage privado, lectura autenticada, digest/MIME/cache y aislamiento del propietario ajeno (`404`).
- Reemplazo con idempotencia, preservación de slot/orden/portada/alt y CAS de revisión; conflicto de revisión distinta.
- Selección de portada, listado rehidratado y cambio de orden con `If-Match` y ETag renovado.
- Retirada lógica persistida (`retired_at`, `visible=false`, activo `deleting`), bloqueo de lectura posterior y conservación de los bytes privados; no se afirma borrado físico.
- Carrera real de dos reservas desde 29 slots: exactamente una obtiene el slot 30 y la otra `ErrLimit`; la reserva/finalización en el 30 funciona y una inserción directa para el 31 es rechazada por PostgreSQL.
- Carrera Reserve/Finalize con lock PostgreSQL controlado: el cambio concurrente de revisión produce precondición fallida sin mutación parcial del objetivo.
- Rehidratación después de iniciar un proceso hijo nuevo: abre otro pool/store/servidor HTTP y lee la variante WebP ya persistida.
- Limpieza de fixtures y almacenamiento propios comprobada por la suite.

## Diagnóstico/corrección

La primera repetición de la nueva cobertura falló al reordenar: PostgreSQL reportó `operator does not exist: smallint >= text (SQLSTATE 42883)`. El dato llegó como parámetro de posición sin tipo explícito en las expresiones de reordenamiento. Se añadieron casts `smallint` explícitos a ambos caminos PostgreSQL de actualización (CAS y legado); la prueba roja se volvió a ejecutar y pasó. No se dejó el código de diagnóstico temporal en el test.

## Estado y límites restantes

**F4 completada para el alcance local real de handler→servicio→PostgreSQL→storage privado, incluidos los criterios listados arriba.** La integración autenticada browser→Next BFF→Go se verifica dentro de F6, no se infiere de este test. La matriz de UI previa sigue siendo mock. No se evaluó corpus fotográfico representativo ni se declara cumplido el SLO de peso/calidad WebP; tampoco hay evidencia de storage cloud.

Log privado de la última ejecución: `.local/testing/f6-runtime/f4-gallery-final-20261029.log` (modo 0600).