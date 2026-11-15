# Evidencia VI-03 — revocación antes de entregar el detalle reviewer

- **Fecha:** 2026-11-13 (`America/Santiago`).
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD observado:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Alcance:** caracterización local de la ruta HTTP de detalle identity cuando el grant exacto desaparece en la revalidación final posterior a la lectura sintética de la declaración. Sin datos reales, DB, SQL/DDL, Cloud ni proveedores externos.

## Cambio y resultado

Se añadió `TestReviewerIdentityDetailSuppressesDeclarationWhenGrantRevokedAfterRead`. La ruta registrada usa el handler y `ReviewerAccessService` reales, con autenticación, repositorio, declaración y storage sintéticos. La autorización fake contiene el grant identity exacto y otro grant identity del mismo actor para un scope distinto. Al terminar exitosamente el callback que leyó la declaración, el fake elimina solo el grant exacto y conserva el del otro scope; la revalidación final debe denegar el detalle. La solicitud termina con 404 neutral (`not found`), sin ETag ni valores privados de la declaración en el cuerpo y con `Cache-Control: private, no-store`; se verifican tres lecturas scoped, una lectura de declaración, el grant no coincidente retenido y cero lecturas de documentos. La revocación se dispara por el retorno del lector, no por asumir un número de llamada.

Es una caracterización test-only, sin cambio productivo ni RED/GREEN. Al endurecer el fake, una primera fixture con una capacidad space adicional fue rechazada por la regla fail-closed de capacidades mixtas; se sustituyó por otro grant identity de scope distinto, que permite comprobar eliminación exacta del scope solicitado sin relajar esa regla.

## Verificación ejecutada

Con entorno mínimo `env -i` para evitar configuración heredada de persistencia:

- Focal con detector de carreras, `-count=20` — PASS.
- `go test -race -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS.

La revisión source-only independiente `deleg_01a3cbcc` aprobó el candidato final sin hallazgos. Confirmó que solo se elimina el grant exacto después del retorno exitoso del lector, que se conserva el grant del otro scope y que el handler no entrega declaración ni ETag ante la revalidación fallida. El revisor no ejecutó pruebas; la repetición focal y las suites anteriores son la evidencia de ejecución.

## Límites

El fake prueba el orden del flujo y que el handler no entrega la declaración cuando la revalidación final simulada niega el grant. La aserción de tres lecturas refleja la secuencia actual de `LoadDetail`; el disparo de la revocación, en cambio, se liga al retorno del lector. No prueba transacciones, locks, PostgreSQL, revocación concurrente persistida ni autorización efectiva; tampoco demuestra que una revocación posterior al último chequeo y anterior a la transmisión HTTP se comporte de forma atómica. No se ejecutó SQL/DDL ni se accedió a DB persistente o Cloud. VI-03 y el MVP permanecen parciales.

Sin commit, push, deploy ni integración a `main`.