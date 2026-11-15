# Evidencia — VI-01, compuertas de servicio y límites de parámetros en identityverification

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc3459c55b6ef2220d91244ea1d9da7a5fb`.
- **Entorno de ejecución:** Entorno local aislado con fixtures sintéticos en memoria. Sin acceso ni llamadas a Cloud Storage, Secret Manager, Cloud SQL, bases PostgreSQL persistentes ni DDL/SQL.
- **Archivo de pruebas modificado:** `internal/identityverification/service_test.go`.

## 1. Alcance cubierto

Se implementaron pruebas unitarias en `TestServiceConstructorAndParameterBoundaries` para complementar la suite existente de `internal/identityverification` y cubrir las compuertas fail-closed y límites de parámetros de `internal/identityverification/service.go`:

1. **Constructor de servicio (`NewService`)**:
   - Rechazo fail-closed con `ErrInvalid` cuando el repositorio suministrado es nulo (`NewService(nil)`).
   - Retorno exitoso de la instancia configurada ante repositorio válido.

2. **Receptor nulo y dependencias nulas en `RequireForBooking`**:
   - Rechazo fail-closed con `ErrInvalid` ante invocación con receptor nulo (`(*Service)(nil).RequireForBooking(...)`).
   - Rechazo fail-closed con `ErrInvalid` cuando la instancia posee un repositorio nulo (`(&Service{repository: nil}).RequireForBooking(...)`).

3. **Validación estricta de parámetros en `RequireForBooking`**:
   - Rechazo fail-closed con `ErrInvalid` ante `hostID` vacío o con solo espacios en blanco (`"   "`).
   - Rechazo fail-closed con `ErrInvalid` ante `renterID` vacío o con solo espacios en blanco (`"   "`).
   - Rechazo fail-closed con `ErrInvalid` ante timestamp de referencia `now` con valor cero (`time.Time{}`).

## 2. Resultados de ejecución local registrada

```bash
$ go test -race -v -run '^TestServiceConstructorAndParameterBoundaries$' ./internal/identityverification
=== RUN   TestServiceConstructorAndParameterBoundaries
--- PASS: TestServiceConstructorAndParameterBoundaries (0.00s)
PASS
ok  	github.com/HernanEspinozaDev/espaciGo/internal/identityverification	1.030s

$ go test -race -count=1 ./internal/identityverification && go test -coverprofile=/tmp/cov_idver2.out ./internal/identityverification && go tool cover -func=/tmp/cov_idver2.out
ok  	github.com/HernanEspinozaDev/espaciGo/internal/identityverification	1.018s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/identityverification	0.006s	coverage: 100.0% of statements
github.com/HernanEspinozaDev/espaciGo/internal/identityverification/service.go:50:	NewService		100.0%
github.com/HernanEspinozaDev/espaciGo/internal/identityverification/service.go:60:	RequireForBooking	100.0%
github.com/HernanEspinozaDev/espaciGo/internal/identityverification/service.go:82:	currentFor		100.0%
total:											(statements)		100.0%
```

## 3. Conformidad con directivas y límites

- Cobertura de statements en `internal/identityverification`: **100,0%** (todas las funciones al 100,0%).
- Cero DDL/SQL ejecutado.
- Cero acceso a bases persistentes o Cloud Storage.
- `git diff --check`: limpio.
