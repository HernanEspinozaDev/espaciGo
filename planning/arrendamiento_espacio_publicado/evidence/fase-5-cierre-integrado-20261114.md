# Evidencia de Cierre — Fases 1 a 5 del Plan de Arriendo Publicado

- **Fecha:** 2026-11-14
- **Rama de tarea:** `codex/marketplace/feature/rental-contract-preview-adapters`
- **Worktree:** `/home/nandev/Documentos/projects/espaciGo-rental-demo-reconciled`
- **Identidad Git / Autor:** `HernanEspinozaDev` (verificado con `scripts/git-role.sh check-direct`)
- **Base de partida:** `ea0c7fc08238355ecc31446fa9c8fa5e4a410436` (corte DEMO reconciliado)
- **SHA de cierre:** `7005d8a`
- **Aislamiento de infraestructura:** Sin modificaciones al ledger de migraciones (V1–V84 intactas byte a byte), sin alteraciones a bases persistentes ni conexiones a Cloud SQL.

---

## 1. Alcance Ejecutado y Puertas Verificadas

### Fase 1: Adaptadores de Persistencia y Contratos en PostgreSQL
- Se implementó el adaptador nativo `internal/adapters/postgres/rentalflow/repository.go` conectando los puertos del dominio `rentalflow`:
  - `QuoteRepository`: almacenamiento y lectura de cotizaciones sobre `public.cotizacion_reserva_ensayo` con validaciones de tasa y disponibilidad.
  - `AvailabilityReader`: validación de solapamiento de intervalos contra `public.ocupacion` mediante `tstzrange`.
  - `PublicListingReader`: consulta de publicación activa y tarifa vigente en `public.espacio` y `public.tarifa_espacio`.
  - `ContractIdentityReader`: extracción de nombre legal y RUT canónico desde `public.identity_verification_attempts` (migración V84 de Didit) con fallback a `public.document_verification_case`.
- **Verificación unitaria:** `go test -race ./internal/adapters/postgres/rentalflow/...` -> **PASS (1.026s)**.

### Fase 2: Disponibilidad en Ficha Pública
- Componente React `PublicSpaceRentalAvailability` ([public-space-rental-availability.tsx](file:///home/nandev/Documentos/projects/espaciGo-rental-demo-reconciled/frontend/src/features/public-catalog/ui/public-space-rental-availability.tsx)) integrado en la vista pública `/espacios/[id]`.
- Selector de duraciones exactas según unidad tarifaria ratificada:
  - `hora`: {1, 2, 4} horas.
  - `dia`: {1, 2, 3} días.
  - `mes`: {1} mes.
- Consulta asíncrona a `/api/backend/spaces/{id}/rental-availability?date=...&duration=...`.
- La consulta es estrictamente informativa: no crea bloqueos, no retiene horarios ni expone datos privados de ocupantes.

### Fase 3: Selección de Intervalo y Prompt de Autenticación
- Al seleccionar un intervalo disponible sin sesión activa, se presenta la advertencia autorizada:
  *«Para continuar, inicia sesión o crea una cuenta»*
- Enlace directo con retorno seguro: `/acceso?returnTo=/espacios/{spaceId}`.
- Tras autenticarse, el flujo retoma la publicación con el intervalo seleccionado para cotizar autoritativamente con el servidor.

### Fase 4: Gate KYC y Vista Previa Contractual
- Componente modal `PublicRentalContractPreview` ([public-rental-contract-preview.tsx](file:///home/nandev/Documentos/projects/espaciGo-rental-demo-reconciled/frontend/src/features/public-catalog/ui/public-rental-contract-preview.tsx)):
  - Invoca `POST /api/backend/rental-quotes/{quoteID}/contract-preview`.
  - **Gate KYC:** Si el usuario no tiene identidad aprobada, el servidor responde 409 y la UI guía a `/verificacion` para completar el proceso.
  - **Borrador de contrato:** Al contar con identidades verificadas, presenta nombres legales y RUTs canónicos de ambas partes, fechas en zona horaria IANA del espacio y desglose de precio en CLP.
  - **Advertencia Legal ADR:** Leyenda obligatoria destacando que se trata de un borrador preliminar generado automáticamente que no constituye contrato firmado, pago ni reserva confirmada.

### Fase 5: Validación Local Combinada E2E
- Suite automatizada `frontend/tooling/rental-plan-e2e.test.mjs`:
  - Valida el recorrido de punta a punta: consulta pública -> prompt de acceso -> cotización autenticada -> gate KYC (409) -> vista previa autorizada con datos de ambas partes y aviso legal.

---

## 2. Resultados de Pruebas Automatizadas

```bash
# 1. Pruebas Go bajo race detector
go test -race ./internal/adapters/postgres/rentalflow/... ./internal/rentalflow/... ./cmd/rentaldemo/... -count=1
# ok  github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/rentalflow 1.026s
# ok  github.com/HernanEspinozaDev/espaciGo/internal/rentalflow                   1.151s
# ok  github.com/HernanEspinozaDev/espaciGo/internal/rentalflow/transport/http    1.199s
# ok  github.com/HernanEspinozaDev/espaciGo/cmd/rentaldemo                        1.019s

# 2. Pruebas de tooling Frontend Node.js
node --test tooling/rental-demo-*.test.mjs tooling/public-rental-availability.test.mjs tooling/public-rental-contract-preview.test.mjs tooling/rental-plan-e2e.test.mjs
# ℹ tests 41
# ℹ pass 41
# ℹ fail 0
# ℹ duration_ms 681.175473

# 3. Tipado y Lint
pnpm typecheck # tsc --noEmit -> 0 errores
pnpm lint      # eslint -> 0 errores
pnpm api:check # 216 operaciones OpenAPI sincronizadas

# 4. Higiene Git
git diff --check # Limpio, sin advertencias
```

---

## 3. Estado de la Rama
- Rama de tarea: `codex/marketplace/feature/rental-contract-preview-adapters`
- Historial de commits en la rama:
  - `80e6ff0`: feat(marketplace): add PostgreSQL adapter for rental flow and contract identity
  - `815c549`: feat(marketplace): integrate public rental availability and authenticated login prompt
  - `b2deb73`: feat(marketplace): add contract preview dialog with KYC gate integration
  - `ace13f2`: test(marketplace): add end-to-end integration test for rental plan journey
  - `7005d8a`: docs(marketplace): record completion of phases 1-5 in rental plan
- Listo para integración a `main` local según las instrucciones del revisor autorizado.
