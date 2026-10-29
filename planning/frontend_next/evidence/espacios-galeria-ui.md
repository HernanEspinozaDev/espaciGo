# Evidencia — primera integración de galería real UI

**Fecha de evidencia inicial:** 2026-10-27  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`  
**Rama / HEAD:** `main` / `3956d2f` (cambios locales sin commit)  
**Alcance:** frontend Next.js; no es prueba integral contra Go/PostgreSQL.

> **Evidencia cronológica, no estado actual.** El test inicial del 2026-10-27 y los cortes F4 siguientes son mock UI salvo la integración handler→PostgreSQL descrita explícitamente. El lookup `ready` del replacement se añadió después; los estados vigentes y los gaps browser→BFF están en `plan_creacion_espacios_galeria_documentos.md` §8 y `espacios-f4-pending-replay-ui.md`.

## Actualización F4 — 2026-10-28

- El límite visible del editor se alineó con el cupo local F2: **30 fotos por espacio**. La ayuda muestra «30 por espacio».
- El E2E controlado con Playwright parte de 4 fotos: selecciona 27 (31 en total), comprueba el error de límite y que el contador de `POST` no cambie; luego acepta 26, llega a 30, muestra 30 botones «Eliminar foto» y deshabilita el input.
- El mismo E2E verifica que el borrador conserva el ETag sintético de `GET /spaces/{spaceId}` y envía `If-Match` en el `PUT`; es cobertura del cliente con mock, no del servidor Go.
- Gates verificados en esta actualización: `pnpm format:check`, `pnpm lint`, `pnpm typecheck`, `pnpm test` (260/260), `pnpm api:check` (181 operaciones), `pnpm build` (Next.js 16.4.3), Playwright `tests/phase7-host-listings.spec.ts` (8/8) y la prueba enfocada de navegación por pestañas (1/1). El primer E2E sobre el artefacto anterior falló; tras reconstruir, el caso pasó. La suite también detectó ausencia de `If-Match` antes de corregir el cliente; pasó después.
- El recorrido usa `mockGateway` local y solicitudes de Playwright a loopback: **no demuestra** browser→BFF→Go→PostgreSQL/storage, ni persistencia real de imágenes. F4 permanece parcial; siguen pendientes esa integración visible y un corpus fotográfico representativo. La cancelación en vuelo, que seguía pendiente en la evidencia inicial, ahora aborta localmente el fetch y consulta lookup; no es cancelación remota. La prueba mock vigente está en `espacios-f4-pending-replay-ui.md`.
- Esta rebanada frontend no modificó backend, SQL, GCP, Terraform, IAM, volúmenes ni datos persistentes. La migración local F2 y su alcance están documentados por separado en `espacios-f2-local-postgres.md`.

## Cambios locales — evidencia inicial (2026-10-27)

- Mi cuenta incluye el acceso **Mis espacios** a `/anfitrion/espacios`.
- El editor recibe JPEG/PNG/WebP como formatos de entrada y exige coherencia extensión/MIME como validación rápida; no los convierte en el navegador. El flujo Go (`internal/media/service.go` + `internal/media/codec.go`) valida los bytes, aplica orientación/limpieza y crea derivados display/thumbnail WebP; `MediaStore` solo acepta `image/webp`. El codec usa quality 80 y lados máximos 1600/480. Una medición local sintética inicial figura abajo; aún falta corpus fotográfico representativo y comprobar el recorrido real BFF→Go→GCS.
- Se conserva portada, reordenamiento y eliminación según las rutas existentes. El E2E comprueba el DELETE de la galería con respuesta mock HTTP 200 `{photo_id, removed:true}`, alineada con Go; no es una llamada a Go real. La miniatura se muestra solo si el DTO contiene una variante `thumbnail` validada; solo se aceptan variantes `thumbnail`/`display`, `image/webp` y URL exacta del contrato Go. La ruta Go se remapea a la misma origen `/api/backend/...`.
- La descarga privada usa `PrivateBlobUrlRegistry`; se revoca al desmontar/cambiar actor, espacio o foto. No se usa bucket ni URL firmada desde navegador.
- Fixture E2E: PNG local 1×1 con CRC comprobado. La respuesta Go/BFF está simulada en Playwright.

## Verificación ejecutada — evidencia inicial (2026-10-27)

| Gate                | Resultado          |
| ------------------- | ------------------ |
| `pnpm format:check` | PASS               |
| `pnpm lint`         | PASS               |
| `pnpm typecheck`    | PASS               |
|                     | `pnpm test`        | PASS — 260/260                                                                   |
|                     | `pnpm build`       | PASS — Next.js 16.4.3                                                            |
|                     | `pnpm api:check`   | PASS — 181 operaciones                                                           |
|                     |                    | Playwright `tests/phase6-account.spec.ts` + `tests/phase7-host-listings.spec.ts` | PASS — 14/14; incluye cola diferida con máximo dos POST activos, cancelación de la tercera sin POST ni conteo como éxito, retry fallido→exitoso con UUIDv4 `Idempotency-Key` y bytes idénticos, validación y cupo 10 |
|                     | `git diff --check` | PASS                                                                             |

- El E2E usa mocks locales y verifica límites de archivo/cupo; no demuestra transferencia efectiva navegador→BFF→Go→PostgreSQL ni persistencia real.
- El editor gestiona cada selección como lote: máximo dos POST activos, estado accesible por archivo (`En cola`, `Subiendo`, `Carga completada`, cancelada o error), spinner indeterminado y resumen que cuenta solo cargas completadas. E2E con respuestas diferidas verifica que el tercer archivo espera, puede cancelarse sin emitir POST, permanece cancelado al terminar las dos cargas activas y no se cuenta como éxito; también verifica que validaciones/cupo 10 siguen operando. Para cada carga fallida, «Reintentar» vuelve a enviar únicamente ese archivo y conserva el mismo `File`, `is_cover` y UUIDv4 `Idempotency-Key`; el E2E compara clave y bytes y comprueba una única foto por éxito. La rebanada posterior F4 añadió cancelación local del fetch ya despachado con lookup: `pending→ready` reconcilia, y tras `not_found` no hay replay automático; el reintento explícito reutiliza la clave. El adaptador usa `fetch`, por lo que no se muestra porcentaje de bytes. Esto no prueba cancelación ni rollback remoto.

## Medición local del codec — fixtures sintéticos

Ejecuté `go test -overlay=/home/nandev/.hermes/cache/scratch/codec_metrics_overlay.json -run '^TestTemporarySyntheticWebPMetrics$' -count=1 -v ./internal/media`. El overlay cargó un test temporal desde scratch, sin editar fuentes/tests Go del repositorio ni escribir en DB/storage. `Convert` usa WebP quality 80; los números siguientes son salidas reales del codec local. Las metas son ≤614.400 bytes para display (600 KiB) y ≤102.400 bytes para thumbnail (100 KiB).

| Fixture de entrada (JPEG)                           | Bytes entrada | Display (dimensiones; bytes) | Thumbnail (dimensiones; bytes) | SLO          |
| --------------------------------------------------- | ------------: | ---------------------------- | ------------------------------ | ------------ |
| Gradiente sintético 1600×1200, q85                  |       918.232 | 1600×1200; 539.322           | 480×360; 77.236                | Cumple ambos |
| Textura sintética por bloques 16 px, 1600×1200, q85 |       632.290 | 1600×1200; 307.516           | 480×360; 84.596                | Cumple ambos |
| Mismo patrón de gradiente, 1600×1200, q75           |       729.393 | 1600×1200; 572.736           | 480×360; 78.260                | Cumple ambos |
| Textura sintética por bloques 64 px, 5000×5000, q50 |     5.773.978 | 1600×1600; 476.860           | 480×480; 100.982               | Cumple ambos |

Los cuatro fixtures deterministas quedaron bajo el límite de entrada de 10 MiB y cumplieron las metas. Son patrones generados en memoria, no fotografías; esta medición **no** cierra el SLO con un corpus fotográfico representativo ni acredita el recorrido BFF→Go→GCS.

## Límites y bloqueos observados al 2026-10-27

- Al 2026-10-27 el límite visible era **10 fotos**. La actualización F4 de arriba lo alinea a 30 en el editor; el delta local F2 se documenta aparte en `espacios-f2-local-postgres.md`.
- La inspección del 2026-10-27 verificó el acceso autenticado al PostgreSQL nativo `127.0.0.1:5432` como `espacigo_runtime` y la lectura de catálogo en transacción `READ ONLY`; entonces no se leyeron filas de espacios ni hubo escrituras. La migración local posterior está documentada en `espacios-f2-local-postgres.md`.
- La inspección del 2026-10-27 no vinculó un API HTTP de galería con esa base. La sonda Go one-shot READ ONLY del 2026-10-28 verificó el target local, pero no el recorrido browser→BFF→Go; la API Docker en `127.0.0.1:18081` no se usó como sustituto.
- No se probó un recorrido de galería autenticado browser→BFF→Go→PostgreSQL/storage; esa integración sigue pendiente. No hubo commit, push ni despliegue.
- La cola local limita dos uploads concurrentes y muestra estados accesibles por archivo, sin porcentaje ficticio. El reintento por archivo y la cancelación de elementos aún en cola están implementados y probados con mocks; una actualización F4 posterior añadió aborto local de carga ya enviada y consulta de estado autoritativo, probada con mocks (no es cancelación ni rollback remoto). El replacement pending conserva archivo/clave/ETag/target, mantiene visible la foto anterior y ofrece lookup/retry explícito; `deleting` descarta el archivo candidato como limpieza terminal y permite iniciar otro intento, siempre mock-only. En el corte de esta evidencia aún no estaba cubierto lookup `ready` del replacement; la regresión Playwright mock posterior lo añadió, como se documenta en `espacios-f4-pending-replay-ui.md`. Siguen pendientes el corpus fotográfico representativo y documentos privados. El aumento de cupo 10→30 quedó cubierto en el editor local por la actualización F4, no como integración real.

## Siguiente paso

Al corte inicial del 2026-10-27, antes de intentar browser→BFF→Go→PostgreSQL/storage se debía seleccionar un target aislado y demostrar runtime sin workers mutadores. Ese gate se resolvió después solo para F6 mediante autorización exacta; no autoriza F5. No usar la DB persistente por inferencia ni sustituir un target de fase por otro.
## Actualización posterior — ejecución en target sintético (2026-10-28)

El bloqueo inicial de target/perfil quedó superado por la autorización explícita del stack temporal `espacigo_f6_synth_20261028`, sin Mailpit ni flujo de cuenta. La prueba IMG01F posterior ahora usa el handler Go HTTP real, PostgreSQL tmpfs y filesystem privado, con stub de autorización sintético; la carrera Reserve→Finalize y los edge cases V78 también pasan. `cmd/api` worker-free había respondido 200 en `/health/ready`; la ruta protegida devolvió 401 sin credenciales. El 2026-10-28 se reruneó `env PLAYWRIGHT_PORT=4205 npm exec --yes --package=pnpm@12.15.0 -- pnpm exec playwright test tests/phase7-host-listings.spec.ts --project=chromium-local`: 18/18 con `mockGateway`, así que aún no es browser→BFF→sesión Go real. Se autorizó sólo una fila `usuario` sintética `.invalid` como fixture FK; no hubo signup/login, correo/clave/token real ni Mailpit. F4 sigue parcial por BFF autenticado, corpus fotográfico representativo y cancelación enviada con reconciliación. Los procesos/contenedor exactos se retiraron; puertos 15432/18083/4205 quedaron libres en ese corte. **Ese cleanup es histórico:** el owner reautorizó después el target F6 exacto, cuyo estado posterior está en `espacios-f6-autorizacion-auth-local.md` y §8 del plan; no se usó para F5.

