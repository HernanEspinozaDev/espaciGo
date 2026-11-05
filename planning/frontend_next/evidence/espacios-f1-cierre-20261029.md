# Cierre F1 — contrato y delta de espacios

**Fecha de verificación:** 2026-10-29. **Checkout:** `/home/nandev/Documentos/projects/espaciGo`, rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`, HEAD `3723e51e0b3d7658915c753163b43c6f705e9e17`.

## Alcance cerrado

F1 cierra el contrato y el delta mínimo de persistencia para espacios, no la integración funcional completa ni las fases posteriores. El contrato canónico queda en el plan maestro §2–§7 y en `planning/openapi.yaml`:

- Revisión compartida fuerte basada en `espacio.actualizado_en`; `attribute_schema_version` permanece separado. `If-Match` único y fuerte: ausente 428, sintaxis/tag inválido 400, revisión obsoleta 412; 409 queda para conflictos de dominio/cupo. El BFF conserva status/código y ETag.
- Alcance CAS: borrador/contenido, publicación, tarifa, zona horaria y horarios semanales, además de portada, metadatos/visibilidad, orden, retiro y reemplazo de galería. Los escritores protegidos comparan bajo lock y los cambios efectivos avanzan la revisión en su transacción; los caminos protegidos no hacen fallback a writers sin versión.
- Alta de galería es append-only/idempotente y reserva cupo bajo coordinación de espacio; reemplazo conserva el slot anterior hasta que la nueva variante esté lista y no consume un slot lógico adicional. Máximo 30 slots por espacio, incluidas reservas pendientes; espacios de un mismo propietario tienen cupos separados.
- Storage: lectura dual V1/V2 antes de nuevas escrituras V2; no renombrar objetos históricos. Nuevas claves de espacio no incluyen datos personales y las escrituras son condicionadas.
- Contratos OpenAPI/gateway inventariados; no se añade una migración para CAS, que reutiliza `actualizado_en`/`spaceversion.Bump`.

## Delta de datos fijado

La ampliación de cupo y namespaces de imagen quedó implementada en la cadena versionada V68–V76, aplicada al PostgreSQL nativo únicamente bajo autorización F2 y con backup/postcheck descritos en `evidence/espacios-f2-local-postgres.md`. V77–V78 pertenecen al target sintético F6, no al nativo. F1 no reserva V79 ni requiere otro DDL: no se identificó una columna nueva para CAS. No se ejecutó DDL durante esta verificación.

## Revisión y evidencia

La revisión cruzó los criterios de salida F1 del plan con el contrato §7, handlers/repositorios actuales, OpenAPI y evidencia por corte:

- RED→GREEN de precondiciones de borrador: `evidence/espacios-ejecucion-cierre-20261028.md`.
- Tarifa y CAS de publicación/contenido: `evidence/espacios-cas-tarifa-20261028.md`, `evidence/espacios-cas-publicacion-contenido.md`.
- Horarios owner-scoped, CAS de galería y sus límites: `evidence/espacios-ejecucion-cierre-20261028.md` §§ continuación 2026-10-29 y corte CAS de galería.
- La suite RED/GREEN y la revisión de cada delta se distinguen de la prueba PostgreSQL; ningún test fake se presenta como prueba de serialización real.

Verificación ejecutada en este checkout el 2026-10-29:

- `env -u TEST_DATABASE_URL -u IMG01F_TEST_DATABASE_URL -u DATABASE_URL -u PGHOST -u PGPORT -u PGDATABASE -u PGPASSWORD go test -count=1 ./...` — PASS.
- La misma desactivación de destinos opt-in para `go vet` en `./...` no se ejecutó en esta verificación; `go vet` focalizado sí pasó para spaces, gallery, media, pricing, occupancy, booking y sus adaptadores PostgreSQL.
- Pruebas focalizadas `go test -count=1` de esos dominios/handlers/adaptadores — PASS.
- `node tooling/api-check.mjs` desde `frontend/` — PASS; contrato, allowlist e inventario actuales, 185 operaciones.
- `git diff --check` — PASS antes de agregar esta evidencia.
- `pnpm` no está disponible en el entorno; no se ejecutaron gates frontend adicionales en este cierre de contrato.

## Límites que permanecen abiertos

Este cierre **no** declara F2/F3/F4 completas ni integración browser→BFF→Go→PostgreSQL/storage. La ruta owner-scoped `weekly-hours` no tiene evidencia PostgreSQL real; los tests PostgreSQL opt-in se mantuvieron desactivados. F3 conserva la rehidratación/CRUD persistidos y validación real; F4 conserva integración autenticada, corpus fotográfico representativo y reconciliación durable. F5 conserva sus gates de persistencia/ACL/runtime. F6 permanece parcial por decisión expresa; F7 requiere autorización independiente. No se tocaron cuentas, documentos reales, base de datos, Cloud SQL, GCP, Terraform, IAM, ramas remotas ni despliegues.
