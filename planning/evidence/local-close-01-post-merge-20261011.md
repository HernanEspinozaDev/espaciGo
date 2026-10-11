# LOCAL-CLOSE-01 — comprobación posterior al merge de V53/V54

Fecha: 2026-10-11. PR #266 está fusionado en `main` mediante `0a47cafc24f0570b53e19c7d81d7ed8a806b50a1` (19:20:59 UTC). La cuenta activa de GitHub verificada fue `HernanMEC`. El árbol de trabajo quedó en `main` sincronizado con `origin/main`.

## Integración y equivalencia

El código ejecutable de `cmd/api`, `db`, `internal` y `scripts` en el merge coincide con el commit validado `03e9c77fb08815b63f2c8d8b3cc96ffe635ca491`. Se reutilizó la regresión integral ya publicada para ese código; no se volvió a ejecutar el runner completo.

## Volumen persistente

- `bash scripts/dev-env.sh up -d` inició el stack local y aplicó únicamente las migraciones pendientes sobre `espacigo_pgdata`.
- Backend, PostgreSQL, Mailpit y mock quedaron `running healthy`.
- `bash scripts/dev-env.sh verify-http`: PASS en liveness/readiness, CORS/preflight, HTML/config/JS/CSS del mock y conectividad mock→API.
- `schema_migrations` confirma V53 (`2c34092c9db8cab94c0e6ebab1e72d82b061eead465b913d46e91aeb77842e68`) y V54 (`7fc01ac4ee0ee83db339fadfe95fc032a25cd2621184abe4d2408245ce926af3`).
- Grants efectivos para `espacigo_runtime` sobre `contrato_ensayo_local`, en orden SELECT/INSERT/UPDATE/DELETE, y EXECUTE sobre `purge_expired_local_contracts_and_publication_history`: `t|t|f|f|t`. Es decir, puede consultar/crear, no modificar ni borrar directamente, y solo puede ejecutar la purga acotada autorizada.
- No se alteraron secretos ni se reinició/eliminó el volumen. No se adelantó ningún reloj, se invocó ningún purgador ni se borraron datos persistentes.

## Exportación contractual de ambas partes

Se ejecutó la integración focalizada en PostgreSQL desechable:

```sh
GO_TEST_RUN='^TestLocalContractExportIsParticipantScopedMinimizedAndReportsRetiredPDF$' \
  bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity
```

Resultado: PASS. La prueba llama al handler HTTP real sobre PostgreSQL, descarga y abre los ZIP de anfitrión y arrendatario, comprueba snapshot y firmas/historial minimizados, valida el PDF incluido y la ausencia de identificadores/contacto de la contraparte. Un tercero no recibe el contrato. También verifica que una exportación no muta los registros y que PDF ya retirado se anuncia en el manifiesto sin reconstrucción. No se usaron datos personales reales ni se modificó el volumen persistente.

## Cierre presentado

La matriz LOCAL-1 queda actualizada con la evidencia post-merge. No se identifica una brecha funcional local obligatoria pendiente para el alcance ratificado. Proveedores reales, validez jurídica, documentos fiscales reales, criterios generales de los Issues padre, ubicaciones de respaldos no inventariadas y GCP permanecen separados según decisiones vigentes. #246 sigue abierta; este registro presenta LOCAL-1 para aceptación explícita y no lo declara aceptado ni cerrado.
