# Entrega del wizard por main — seguimiento verificable

## Integración y primer intento

- La revisión final del CLI aprobó el guard de argumentos posicionales y la exclusividad de modos sin bloqueadores. Go global y vet posteriores: PASS. Dos SQL reales y dos reconciliaciones browser: PASS, no SKIP.
- Merge de conciliación `79f874124b7f8c4f7c47246773cd1933bd0299ba`, padres tarea `c6bca78e5b72eb46920a817ad2761d432d562a21` y main `214c75dc612374c868cc4209ecef521b497d0c19`.
- Se publicó primero la rama de tarea y se verificaron ref exacto, autor/committer HernanEspinozaDev y worktree limpio. Después se integró mediante fast-forward en el worktree main-sync limpio y se publicó main normalmente, con hooks y lectura remota exacta. Sin PR, force-push ni reescritura.
- El trigger automático creó Cloud Build `9662d633-d6b9-4b27-8e85-f4b785d9d599` para ese SHA.

## Gate de calidad fallido: no desplegado

El build terminó FAILURE. Metadata releída: `verify-source-quality=FAILURE`; `release-main=QUEUED`, sin timing de ejecución. El orquestador de release no se inició; no se reintentó el build ni se ejecutó una secuencia manual, migración o recuperación de lock para esquivar calidad.

GitHub Actions del mismo SHA:

- Frontend architecture and quality `42337288679`: SUCCESS.
- Database migration quality `42337288583`: FAILURE.
- Application quality `42337288605`: FAILURE.

Ambos fallos corresponden exactamente a `TestReviewedWizardAdminInventoryMatchesSource` y `TestWizardAdminIdentityAcceptsOnlyFixedLocalConfiguration`: los tests leen archivos privados reales de `.local` y por eso pasaban en el ensayo existente, pero fallan en un clon sin ellos. El error de padres privados 0700 es correcto en producción y no debe omitirse ni convertirse en SKIP.

El padre reprodujo ambos errores en una extracción aislada de `git archive` del SHA publicado, sin `.local`, credenciales ni base de datos. Esto reabre la portabilidad de las unitarias, no demuestra un fallo de SQL ni de la interfaz. El PASS local anterior no acreditaba un clon limpio.

## Corrección acotada y comprobación en fuente limpia

Se reemplazó esa dependencia por fixtures temporales: raíz aislada, copias de SQL versionado, manifest de prueba calculado a partir de esas copias y passfile sintético sin credenciales. Directorios 0700 y archivos 0600; validadores reales sin debilitar. `t.Chdir` y `t.Setenv` restauran contexto al terminar; el passfile ficticio también se fija para evitar que el parser lea el personal por defecto. No se copiaron archivos privados, modificaron identidades reales ni conectó una base.

El padre aplicó únicamente los dos archivos de test modificados a su extracción versionada limpia que había reproducido el RED. `go test -count=1 ./...` y `go vet ./...` pasaron allí sin credenciales ni `.local`; comprobación posterior confirmó que no se creó `.local` en esa raíz y que no cambió ningún archivo de producción. Es evidencia de portabilidad de tests, no una nueva aprobación de migraciones ni un ensayo SQL real.

La revisión independiente aprobó la corrección sin problemas de seguridad ni lógica. La sugerencia no bloqueante sobre archivos SQL no versionados se contrastó en el padre: los 83 SQL presentes coinciden exactamente con las 83 rutas versionadas. Se conserva el fixture sin dependencia de Git para poder probar extracciones limpias sin `.git`.

Pendientes: publicación normal del seguimiento y verificación de CI/release del nuevo SHA. No se declara imagen nueva, migración, revisión Ready, tráfico ni aceptación online de este conjunto. La QA online sigue reservada al usuario.
