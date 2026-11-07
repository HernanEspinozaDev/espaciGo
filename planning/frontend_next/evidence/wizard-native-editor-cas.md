# Edición semanal nativa y reconciliación CAS

## Resultado independiente

Worktree `espaciGo-wizard-improvements`, rama `codex/frontend/fix/mejoras-creacion-espacios`, base `771134d165046b290b1f5ad3a1074a6357c2327f`, con cambios locales sin publicar.

El revisor ejecutó desde `frontend/`:

```sh
ESPACIGO_WIZARD_NATIVE=1 pnpm exec playwright test \
  --config=playwright.wizard-native.config.ts --workers=1 \
  --grep 'reconciles the saved'
```

Resultado real: **1 passed (2.1s)**. Después, una lectura independiente del recibo privado confirmó `state=verified` y `readback=true`, `reload=true`, `stale412=true`. No se incluyen identificadores, cuenta, ETags ni credenciales en este documento.

## Comportamiento acreditado

- La edición previa del borrador horario existente cambió el cierre del lunes de 18:00 a 17:30 mediante el editor y un solo clic de guardado. Un readback independiente confirmó su persistencia.
- La reconciliación no repite ese guardado ni crea espacios: verifica siete días ISO, lunes 09:00–17:30, martes a viernes 09:00–18:00 y fin de semana vacío; conserva `America/Santiago`.
- El editor muestra 17:30 y lo vuelve a mostrar tras recargar y abrir Horarios.
- Una única solicitud negativa explícita mediante el BFF HTTPS intenta restaurar 18:00 con un ETag anterior realmente observado. El servidor responde **412**; el readback posterior conserva el horario editado.
- No hay PUT del tráfico de página durante la reconciliación ni replay automático. La solicitud negativa de `APIRequestContext` se verifica por su respuesta y se distingue de los eventos de tráfico de Page.

Destino exclusivo: Next HTTPS `127.0.0.1:4206`, API local worker-free `127.0.0.1:18086`, PostgreSQL del ensayo `espacigo_wizard_20261106`. Cuenta sintética existente; ninguna alta, recuperación ni modificación de credenciales.

## Intentos y límites

El recorrido original de edición no fue limpio: después del guardado falló una expectativa del test que exigía cambiar el ETag incluso para una zona horaria sin cambios. Se conservó el recibo y se pasó a reconciliación, sin reejecutar la mutación. En la preparación también se corrigieron la ruta de disponibilidad, los cuerpos PUT que no admiten `time_zone` y la distinción de eventos Page/APIRequestContext. Estos fallos de test no se presentan como aprobaciones ni se borran del historial.

El gate nativo de edición, recarga y rechazo CAS queda demostrado por la ejecución separada anterior. No demuestra aceptación visual humana, respuesta pública positiva de un espacio elegible, disponibilidad por fechas, reservas, integración a main, despliegue o aceptación online.

## Regresión de fuentes actualizada

- `go test ./...`: PASS.
- Unitarias frontend: 476 en total; 475 PASS, 1 SKIP, 0 FAIL.
- `api:check`: tipos, allowlist e inventario actualizados; 197 operaciones.
- TypeScript: PASS.
- Lint global: 0 errores, 4 advertencias.
- `format:check` global: PASS tras formatear el harness HTTPS.

Build definitivo, suite browser conjunta y evidencia visual nativa se mantienen como gates separados pendientes de su ejecución/revisión final.
