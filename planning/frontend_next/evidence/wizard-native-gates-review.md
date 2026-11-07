# Revisión de gates tras ensayo nativo del wizard

## Actualización posterior verificada por el revisor

La matriz de la revisión inicial de abajo conserva su valor histórico; sus pendientes se actualizan con estas ejecuciones posteriores:

- **Edición/readback/recarga/CAS nativos: PASS.** El lunes quedó en 09:00–17:30. La reconciliación separada pasó 1/1, rechazó con 412 un ETag anterior observado y confirmó readback intacto, sin repetir el guardado. Recibo privado leído independientemente como verificado. Evidencia: `wizard-native-editor-cas.md`.
- **SQL semanal real: PASS para proyección y exclusión del borrador.** Se compara el DTO completo con los tramos de PostgreSQL en una misma transacción read-only repetible; `Detail` conserva `ErrNotFound`. No acredita una respuesta pública positiva con fila elegible. Evidencia: `wizard-native-public-sql.md`.
- **Regresión actualizada:** Go global PASS; frontend 475 PASS, 1 SKIP, 0 FAIL; API check, TypeScript y formato global PASS; lint sin errores y con cuatro advertencias. Build definitivo, browser conjunto y revisión visual nativa siguen pendientes.
- **Precisión de conteos:** día y mes pasaron como dos casos combinados en una corrida; no son dos pruebas independientes por cada unidad. Hora tuvo creación y reconciliación separadas, no una creación limpia reejecutada.
- Sin commit/push, integración, despliegue ni aceptación online acreditados de este conjunto.

## Revisión inicial

Fecha de revisión: 2026-11-06. Worktree `/home/nandev/Documentos/projects/espaciGo-wizard-improvements`, rama `codex/frontend/fix/mejoras-creacion-espacios`, HEAD base `771134d165046b290b1f5ad3a1074a6357c2327f`; fuentes locales modificadas/no versionadas. No es un SHA de implementación publicado. Se revisaron el plan `plan_mejora_creacion_espacios.md` y evidencias F1–F4, ledger y preparación nativa. La matriz distingue evidencia automatizada/controlled de ejecución Go+PostgreSQL y de integración pública.

## Resultado por criterio

| Gate del plan | Estado después del ensayo | Evidencia / brecha exacta | Siguiente acción |
|---|---|---|---|
| F1 JSON, etiquetas y validación | Parcial hasta cierre F5 | F1 local controlada y recorrido de creación con mocks PASS; ensayo nativo creó el borrador hora, pero el primer test reportó fallo después de mutar y leerlo. El readback posterior reconcilia estado persistido; no reescribirlo como un caso limpio 1/1 desde su primer recorrido. | Conservarlo como creación+reconciliación; no repetir creación. Reejecutar suites globales tras helpers recientes. |
| F2 región/comuna/CUT | Parcial | Catálogo/procedencia y controles están en F2; falta evidencia final repetida de la suite global/browser sobre el HEAD conjunto si se añadió cobertura después. Antártica permanece no soportada por backend. | Ejecutar caso/control de payload CUT y validaciones en artefacto final; no ampliar backend ni migrar por inferencia. |
| F3 stepper/validación | Gate local aprobado, F5 parcial | Navegación, revalidación, no-POST inválido y ruta privada comprobados en navegador controlado; esto no acredita por sí solo UX visual humana ni integración nativa completa. | Repetir afectadas en build final y registrar la revisión visual local permitida, si el gate visual la exige. |
| F4 horario privado | Parcial | Editor/calendario con mocks y Go unitarios PASS. El test nativo día confirmó creación y readback 2/2; mes también 2/2 según recibos del ensayo. No hay todavía gate nativo de edición de horario con CAS/If-Match ni relectura posterior de modificación. | Añadir un caso controlado autorizado sobre recurso ya creado: modificar y releer horario, comprobar If-Match/concurrencia y no duplicar recursos. Mantener día/mes sin weekly PUT cuando no aplica. |
| F4 proyección pública | Parcial / contractualmente implementada | OpenAPI/DTO minimizado y pruebas Go de dominio/query existen; ficha pública sólo consume `weekly_schedule` opcional. No se ha ejecutado la consulta PostgreSQL real contra datos elegibles, ni probado respuesta anónima HTTP de la ruta pública respaldada por esa fila. Mock/fixture de UI no es esa prueba. | Probar query contra estado legal existente del ensayo y ruta pública real; inspeccionar campos minimizados y elegibilidad. Si no existe fila legal elegible, registrar bloqueo de fixture/schema, no fabricar SQL/estado ni habilitar publicación. |
| F5 nativo hora/día/mes | Parcial con datos persistidos | Según recibos nativos del padre: hora creada y reconciliada con lectura independiente (1/1); día y mes creados y releídos con zona horaria (2/2). La creación hora tuvo fallo post-mutation antes de reconciliar; no declararla 3/3 de recorridos limpios. Sin weekly PUT para tarifas día/mes. | Usar recibos/readbacks existentes; no volver a crear ni saltar el guard de intención. Completar sólo gates que no mutan o editar explícitamente los borradores existentes con CAS. |
| Globales y build definitivo | Pendiente | La evidencia F1/F4 recoge totales anteriores (hasta 470 pruebas), no los helpers/cambios añadidos después del último corte; el plan no registra una corrida global final sobre árbol actual. | Un responsable corre unitarias, typecheck, lint, format, API check, Go relevante y build; luego Playwright sobre ese artefacto recién construido. Declarar skips y verificar recuentos del runner. |
| Integración/publicación/rollout | No iniciada | Rama local dirty, sin commit/push/integración; ninguna comprobación de servicio público ni despliegue. Evidencia local no implica elegibilidad de publicación. | Tras cerrar criterios y revisar diff/hooks/identidad, seguir cadena autorizada de rama→integración normal; para rollout acreditar SHA→build→digest→Ready→tráfico leído. No afirmar publicación previa. |

## Límites y secuencia

La evidencia nativa proviene de una sola cuenta sintética verificada, API local worker-free en `127.0.0.1:18086`, PostgreSQL aislado del ensayo y Next HTTPS local `127.0.0.1:4206`; la identidad y credenciales no se reproducen aquí. No es producción, marketplace público, disponibilidad por fecha, reserva, firma, pago, despliegue ni aceptación de producto. La elegibilidad pública debe seguir la política vigente y el DTO mínimo; el horario semanal no revela ocupaciones ni garantiza disponibilidad.

Orden recomendado: (1) terminar gates globales sobre fuentes actuales y reconstruir; (2) probar edición/readback CAS sin crear nuevos borradores; (3) probar query PostgreSQL y endpoint público únicamente con estado legal ya existente; (4) inspección de capturas/UX local de los tres tipos y Axe si corresponde; (5) revisión integral de diff y evidencia, luego decidir publicación/integración conforme al plan. No ampliar flags, ACL, permisos públicos ni datos para forzar un PASS.
