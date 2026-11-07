# Mejora del wizard — Fase 4, revisión controlada

Base/worktree/rama de `wizard-mejoras-f1.md`, fuentes aún sin commit. F4 tiene **pruebas locales controladas aprobadas**, no integración SQL/nativa ni despliegue.

## Fuentes y contrato

- `WeeklyScheduleSlots` compartido por wizard, editor y wrapper público; presets por día, toggles por hora, edición exacta y modo de solo lectura. Helpers operan en minutos sin redondear extremos fraccionarios, conservan otros días y rechazan un tercer tramo sin alterar el estado previo.
- Wizard conserva configuración IANA y secuencia configureTimeZone→ETag renovado→weekly-hours. No envía horario semanal para tarifas día/mes, que el contrato de horario por hora no admite.
- Editor mantiene su flujo de guardado/CAS y sus barreras de sesión/selección. Conflictos412 muestran mensaje humano y no disparan reenvío automático.
- Ficha pública consume solamente el nuevo `weekly_schedule` opcional del detalle público. El DTO minimizado incluye enabled, time_zone y siete días de periodos open/close; no identidad de propietario, revisión, bloqueos ni reservas. Se conserva la elegibilidad pública existente. La query correlacionada obtiene elegibilidad y horario en el mismo SELECT; requiere todavía prueba PostgreSQL real.
- Ausencia e inactividad se presentan explícitamente. Horario semanal no significa disponibilidad real de una fecha ni habilita reserva, firma o pago.

## Gates del padre

Sobre el build conjunto F1–F4:

- `pnpm typecheck`, `pnpm api:check`, `pnpm build`: PASS; inventario197operaciones, sin endpoints nuevos.
- `pnpm lint`: 0 errores,3advertencias anteriores ajenas al corte.
- `pnpm test`: **470 tests:469PASS,0FAIL,1SKIP** (baseline histórico opcional).
- Helpers de calendario: **7PASS** repetidos por padre.
- Go público: `go test ./internal/publiccatalog/... ./internal/adapters/postgres/publiccatalog -count=1`: PASS. DTO allowlist/omisión y guard textual no equivalen a integración SQL.
- Browser privado: `playwright test tests/wizard-improvements.spec.ts tests/weekly-schedule-editor.spec.ts --workers=1`: **2PASS**, loopback4203, output privado de ensayo en scratch `espacigo-wizard-evidence/f4-private-final`. Editor conserva09:30–10:30, modifica solo martes, rechaza tercer tramo, envía siete días, renueva If-Match y tras segundo guardado412 no repite PUT.
- Browser público dedicado: `playwright test --config=playwright.public-calendar.config.ts --workers=1`: **2PASS**, Next4207+fixture18088, output scratch `espacigo-wizard-evidence/f4-public-final`. Navega martes14:15–16:45 en solo lectura, sin peticiones de escritura; estados ausente/inactivo. Axe completo sin violaciones en1365×900 y390×844, sin desbordamiento horizontal, días con objetivo táctil mínimo44×44.
- Padre inspeccionó capturas desktop/móvil: calendario y marca visibles, sin desbordamiento; estas primeras capturas muestran lunes vacío, no el estado del martes ejercitado. Se solicita mover la captura después de comprobar el horario informado, preservando las anteriores como evidencia del estado inicial.
- `git diff --check`: PASS; puertos4203/4204/4207/18088 libres al finalizar las corridas.

## Revisión y pendientes

Se rechazó una primera consulta que agregaba JOIN después del WHERE y no estaba cubierta por tests SQL. Se corrigió a subconsulta correlacionada; no se afirma que un test textual pruebe su ejecución real. También se corrigieron fixtures y selectores que confundían el announcer de Next con alertas de aplicación.

Pendientes: suite global de navegador tras adaptar controles históricos, revisión local independiente completa, ensayo con Go/PostgreSQL nuevo y creación persistida/readback, backup y cierre de recursos propios. No se han aplicado migraciones a ninguna base ni modificado sus83archivos. Nada publicado, integrado a main o desplegado en este corte.
