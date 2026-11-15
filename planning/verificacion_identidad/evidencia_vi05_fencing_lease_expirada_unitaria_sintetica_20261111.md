# Evidencia — VI-05, fence local ante lease reemplazada o expirada

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; no se alteraron cambios preexistentes.

## Alcance

- Añadidos `TestProcessClaimedRetentionJobFencesMismatchedOrExpiredLeaseBeforeDeletion` y `TestProcessClaimedRetentionJobRunsDeletionForExactLiveLease`, caracterizaciones del `Repository.ProcessClaimedRetentionJob` real usando un `pgx.Tx` sintético que devuelve filas prefijadas.
- Dos escenarios negativos: el lease persistido no coincide con el claim y coincide pero ya expiró. Ambos deben devolver `dv.ErrConflict`, no invocar callback de storage, no confirmar la transacción y llegar al rollback diferido.
- Escenario positivo sintético: con estado `procesando` y lease exacta/vigente, el fake observa el orden callback→completion→audit event→`Commit`. Esto verifica el orden de llamadas en el código, no su atomicidad real.
- El fake rechaza SQL no esperado y el test comprueba la cantidad de lecturas; no implementa ni valida semántica de PostgreSQL.
- Sin cambios de producción.

## Verificación local

- `go test ./internal/adapters/postgres/documentverification -run '^TestProcessClaimedRetentionJob(FencesMismatchedOrExpiredLeaseBeforeDeletion|RunsDeletionForExactLiveLease)$' -count=1`: PASS.
- El mismo filtro con `-race`: PASS.
- `go vet ./internal/adapters/postgres/documentverification`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` para el test y `git diff --check`: PASS.
- Variables de base de datos/PG y opt-in del integration gate retiradas en comandos Go.

## Límites

La revisión independiente `deleg_7e239419` señaló correctamente que la versión previa no comprobaba el orden de `Commit` respecto del audit Exec; se añadió `Commit` a la traza fake y ahora el test exige el orden completo. La re-revisión `deleg_95efeacf` aprobó la versión corregida sin hallazgos y confirmó que las conclusiones permanecen acotadas a control de flujo fake. La revisión anterior `deleg_28bfa8c4` aprobó la cobertura negativa inicial. Caracteriza las ramas de fence/continuación de la lógica Go con resultados fake; no prueba locks, aislamiento, igualdad temporal ni competencia real de lease en PostgreSQL. La prueba de concurrencia/persistencia sigue pendiente bajo su autorización separada. Tampoco demuestra almacenamiento remoto, borrado durable, reintentos durables o backup/recuperación. Solo contexto sintético local; sin SQL/DDL, conexiones a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` ni despliegue.
