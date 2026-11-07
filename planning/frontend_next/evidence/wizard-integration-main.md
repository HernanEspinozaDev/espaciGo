# Conciliación normal del wizard con main avanzado

## Alcance y entradas

La conciliación ocurre en `codex/frontend/fix/wizard-01-mejoras-creacion-espacios`, sin actualizar todavía main. Entradas: tarea publicada `c6bca78e5b72eb46920a817ad2761d432d562a21` (código probado `0659dda11e9bc60576e33a6bd60c79d5b7088a06`) y main `214c75dc612374c868cc4209ecef521b497d0c19`. Se inició `git merge --no-ff --no-commit` tras comprobar worktree limpio y guardar snapshots privados de los tres paths superpuestos. No reset, force-push ni sustitución de archivos completos desde stages/HEAD.

El único conflicto textual fue el enlace del wizard: se conservó `/anfitrion/espacios`, manteniendo el formato de main. El padre comprobó igualdad byte a byte con el snapshot previo y resolvió el índice. **El árbol frontend completo coincide con la tarea publicada**, por lo que su build `3vVei_2oiWumIiG9a51RW` y sus gates siguen correspondiendo a esas fuentes; no se reconstruyó un standalone vivo innecesariamente.

Se comprobó que `AGENTS.md`, `cloudbuild.yaml`, `db/release-policy.json` e `internal/migrator/runner.go` coinciden exactamente con el main entrante. No hay cambios en migraciones aplicadas. El flujo vigente es el orquestador de main con Jobs privados; no se recreó bastión, se abrió SSH ni se ejecutó migración cloud desde local.

## Adaptación semántica del CLI

El auto-merge de `cmd/dbmigrate/main.go` no bastaba: los modos nuevos podían retornar antes de validar las flags administrativas wizard. Se añadió validación inmediatamente después de `flag.Parse`, antes de inventory/schema/release y de cargar conexión:

- Wizard conserva sólo V60/V64/V66 y confirmación exacta del ensayo; confirmación sin versión se rechaza.
- Se rechazan combinaciones con inventory, schema, release y opciones de release sueltas, conservando los conflictos anteriores con plan, ACL, F5 y cloud-owner.
- Sin flags wizard se mantiene el dispatcher de main y su inventario offline.
- Los tests compilan el CLI una vez en un directorio temporal, verifican exit 2 previo a configuración/conexión y prueban inventario offline sin base de datos.

No se ejecutó una migración wizard legítima ni se repitió DDL en el ensayo ya migrado. La primera revisión independiente encontró un bloqueo adicional: el parser estándar de Go deja de leer flags ante un argumento posicional, de modo que `unexpected -wizard-admin-version=60 -confirm-wizard-admin=espacigo_wizard_20261106` podía caer en el flujo ordinario. El padre lo reprodujo con entorno sin credenciales: llegó a `DATABASE_URL is required`, sin conexión. Se corrigió mediante rechazo global de `flag.NArg() != 0` inmediatamente después de parsear y antes de cualquier configuración o dispatcher. Cuatro regresiones de CLI cubren operandos antes de flags, después de `--`, después de `-plan-only` y sin flags: RED observado, luego GREEN con error fijo y exit 2, no sólo cualquier error. El padre repitió Go global y vet después de esta corrección: PASS. La segunda revisión independiente aprobó la adaptación sin bloqueadores, ejecutando las pruebas focales del CLI; el padre confirmó la posición del guard y el diff actual antes de cerrar el gate local de integración.

## Verificación del padre sobre las fuentes combinadas

- `go test -count=1 ./...`: PASS.
- `go vet ./...`: PASS.
- Once unitarias del orquestador heredado `test_main_release.py`: PASS con FakeCloud/temporales; no Jobs, SQL, tráfico ni locks reales.
- Validador estructural de Cloud Build: PASS con `/usr/bin/python3` y PyYAML 6.0.3 ya instalado. El intérprete Hermes no tenía PyYAML; no se instaló otra dependencia ni se confundió ese fallo con uno del pipeline.
- `git diff --check` y comprobación del índice: PASS.

El padre construyó API desde las fuentes combinadas, SHA-256 del binario `de81e2c8bfedfe67a5a6a4eef6ac4ba7b8a017c34148978a6f0cf045d3f6d6ce`, y reinició sólo su API local verificada, con el mismo destino, identidad limitada y perfil worker-free. Readiness y categorías públicas: 200. Next, DB, cuenta, volumen, clave de cursor y credenciales se preservaron.

Después del reinicio:

- PostgreSQL read-only: borrador día excluido y publicación horaria visible con exactamente una auditoría del propietario, dos PASS/ningún SKIP.
- Reconciliación pública anónima sin PUT de publicación: un PASS.
- Reconciliación del horario con rechazo CAS 412 y readback intacto: un PASS.

No se repitieron altas, edición ni publicación. Esos resultados son integración local con el binario combinado, no pruebas funcionales online.

## Preflight de entrega

Se verificó que el worktree dedicado de main está limpio y en `214c75dc612374c868cc4209ecef521b497d0c19`. El trigger regional existente está activo, apunta a `cloudbuild.yaml` y su scope efectivo `full` se obtiene del default YAML vigente; no se modificó el trigger ni se expusieron otras sustituciones.

Gate local de integración aprobado. Pendiente: finalizar/publicar el merge en la rama de tarea, integrar normalmente en el main limpio y verificar el release automático del SHA exacto con sus imágenes, catálogo, revisiones y tráfico. Una lectura de metadata previa no demuestra despliegue. La QA online del producto y aceptación siguen reservadas al usuario.
