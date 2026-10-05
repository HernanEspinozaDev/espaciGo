# Evidencia M04-ATTR-01 — 2026-10-05

## Alcance ejecutado

- Se conserva V5 sin cambios (SHA-256 idéntico a `origin/main`: `f5ca7a06da755c8bbf8adba1ed7c539c69fb50743b3f58012079eb565b564f86`). V6 crea las tablas y perfiles v1; V7 completa el orden de atributos sin alterar el checksum V6 ya aplicado (`67bf11f666e9c3f2687e7bd6c3c0fcd82f075c51ff012f874ed3638b64cf2968`).
- El test PostgreSQL comprueba perfiles en las ocho categorías, lectura/escritura de `false` y `0`, ownership, rollback por versión destino inexistente y cambio de categoría sin mezclar atributos.
- Las pruebas HTTP cubren los tipos, opcionalidad, claves desconocidas, `null`, rangos, límite 16 KiB, respuestas y request contra esquemas OpenAPI, ruta de perfil y rechazo 422.

## Comprobaciones

- `scripts/test-m04-attributes-postgres.sh` — **PASS**, suite Go completa: 22 paquetes, 93 pruebas/casos visibles, cero `SKIP`/`FAIL`. Usa imagen `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`, red Docker `none`, socket Unix y almacenamiento temporal. `DATABASE_URL` se retira y solo se define `TEST_DATABASE_URL` efímera.
- `scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces` — **PASS** tras agregar la prueba de rollback transaccional.
- `npm run build --prefix mock` — **PASS**, TypeScript compilado.
- `go vet ./...` — **PASS**.
- `git diff --check` — **PASS**.
- Se verificó limpieza del test descartable: sin contenedor `espacigo-m04-test-*` ni directorio `espacigo-m04-test.*` en `/tmp` después de finalizar.

## Entorno local persistente

- `scripts/dev-env.sh up -d` reconstruyó backend/mock y aplicó V6 incrementalmente; todos los servicios quedaron healthy. El reintento del migrador verificó el historial ya aplicado sin reiniciar la base.
- El volumen conserva su nombre `espacigo_pgdata`; conteo de borradores: 0 antes/después. Las migraciones registradas son V1–V7; la consulta confirmó 82/82 atributos con orden determinista.
- Los secretos locales se conservaron; sus SHA-256 antes/después coinciden con `aabeee5bd4997cdd0368a6fa208915f6c1871270b39d0cfd761384f26f75be48` (admin) y `88826b1e5986ce1a4e5f3ce2f55bb4d57a7682549d47df8937e20b170ceb7d54` (runtime).
- El rol runtime tiene `SELECT` y no `INSERT` sobre perfiles, y sí `INSERT` en características mediante el servicio; readiness API, página mock y `app.js` devolvieron HTTP 200.

## Límites

La entrega sigue siendo de borradores privados. No habilita publicación, reservas, pagos, almacenamiento/galería ni permisos comerciales. Los pendientes M02/M03, DB02-09 y limpieza de servidor #123 no se alteraron. La validación manual final del recorrido en navegador queda para revisión del usuario.
