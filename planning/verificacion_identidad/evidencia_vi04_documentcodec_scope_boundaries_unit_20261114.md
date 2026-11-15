# Evidencia — VI-02 / VI-04, fronteras de ámbito y canonización en documentcodec

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc4` (trabajo local sobre rama de tarea).
- **Objetivo:** Registrar pruebas de fronteras de serialización, canonicidad estricta de AAD e invariantes de aislamiento criptográfico en `internal/adapters/documentcodec`.

## 1. Alcance cubierto

Se extendió la suite de pruebas en `internal/adapters/documentcodec/codec_boundaries_test.go` (`TestCodecCanonicalScopesOnly`) para cubrir las siguientes fronteras:

1. **Aislamiento por ámbito de identidad y espacio:**
   - Construcción y validación de `IdentityScope` y `SpaceScope`.
   - Validación de que `KeyForScope` y `ScopeFromAAD` verifican el formato canónico y preservan la identidad de AAD sin mutaciones en round-trips canónicos.
   - Invariante de que el desacople de scope en `TestCodecRejectsEveryCrossScopeDimension` rechaza sustituciones en dimensiones clave (propietario, espacio, caso, archivo, propósito) fallando con `ErrCorrupt`.

2. **Rechazo de identificadores no canónicos o malformados en `KeyForScope` / `ScopeFromAAD`:**
   - Espacios, strings en mayúsculas, UUID nil (`00000000-0000-0000-0000-000000000000`), secuencias con `..` y longitudes anómalas.

## 2. Ejecución y resultados registrados

```bash
go test -race -v ./internal/adapters/documentcodec/...
go test -coverprofile=/tmp/cov_codec.out ./internal/adapters/documentcodec/...
go tool cover -func=/tmp/cov_codec.out
```

- `KeyForScope`: **100,0%** de statements.
- Cobertura global de statements en `internal/adapters/documentcodec`: **92,0%**.
- Pruebas `-race`: **PASS** en la suite ejecutada localmente.

## 3. Conformidad con directivas y límites

- Primitivas criptográficas de la biblioteca estándar de Go (`crypto/aes`, `crypto/cipher`, `crypto/rand`) sin dependencias de red ni I/O.
- Cero DDL / SQL y cero conexión a PostgreSQL o servicios Cloud.
- Test-only con estructuras en memoria; las compuertas de persistencia, integración y despliegue permanecen abiertas sin cambios en `main`.
