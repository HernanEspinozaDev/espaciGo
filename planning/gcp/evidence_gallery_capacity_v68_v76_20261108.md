# Evidencia PostgreSQL desechable: transición V68→V76 (2026-11-08)

## Resultado

**PASS.** Se ejecutó `bash scripts/test-migration-runner-postgres.sh v76` desde `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`. El worktree ya tenía cambios locales KYC/migraciones; no se limpiaron ni sustituyeron.

El verificador reportó **7 resultados PostgreSQL requeridos PASS, ninguno faltante ni omitido**. El test creó esquema hasta V67, insertó diez filas sintéticas legacy, aplicó V68 y después únicamente V69–V76. El ledger del fixture terminó en V76 (76 filas); el prefijo V77+ no se aplicó.

## Aserciones ejecutadas

- V68 backfillea las diez fotos legacy en orden determinista.
- Con V68, el intento de insertar el undécimo asset real falla con SQLSTATE `23514` y `space media limit of 10 reached`; no queda fila parcial.
- V69–V76 se aplican sobre ese mismo fixture, conservan las diez filas y el grant condicional `INSERT` de `espacigo_runtime`.
- Después de poblar 29 slots, dos writers concurrentes compiten por el siguiente: uno logra el slot 30 y el otro recibe el límite 30.
- Se acepta la posición 30 y se rechaza el asset 31 con SQLSTATE `23514` y `space media limit of 30 reached`; no queda fila parcial.

Esto confirma el cambio semántico de 10 a 30 en la secuencia fuente y la concurrencia del límite nuevo; no cambia ni reescribe V68.

## Aislamiento y alcance

El runner informó contenedor propio `espacigo-migration-ci-2027679`, imagen `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`, `network=none`, sin puertos publicados ni volúmenes, PGDATA en tmpfs y socket Unix privado bajo el scratch de la sesión. El script confirmó la retirada y ausencia del contenedor.

**Reejecución de cierre (2026-11-08):** el runner volvió a informar 7 resultados requeridos PASS, 0 faltantes/omitidos; contenedor propio `espacigo-migration-ci-2052478`, `network=none`, sin puertos/volúmenes, PGDATA tmpfs y socket privado. El propio script verificó la retirada del contenedor.

No se usaron el PostgreSQL nativo, Cloud SQL, IMG01F, datos/cuentas reales ni credenciales reales. No se ejecutó DDL persistente, despliegue, bootstrap ni cambio de permisos fuera del contenedor efímero.

## Límites restantes

El caso no prueba la rama de backfill V68 que rechaza más de diez filas preexistentes, ni la concurrencia al límite histórico de diez. La integración opt-in `TestSpacePhotoReplacementAtomic` y su target IMG01F no se ejecutaron. La autoridad/ACL efectiva y los ledgers actuales de destinos persistentes permanecen sin verificar; tampoco se ejecutaron V77–V84 en este test. La auditoría semántica global continúa abierta.
