# Galería de Mis espacios → marketplace: corrección de sincronización

## Alcance y estado

Reporte humano: las fotografías añadidas desde la edición de una publicación activa aparecen en Mis espacios pero no en el marketplace. Se conserva el identificador de la publicación únicamente en el contexto privado del encargo; no se leyó ni modificó ese registro para este diagnóstico.

Base revisada: `fee545186040a947365e3d4e4df386562b8c8c9b`, worktree `espaciGo-main-integration`, `main` coincidente con `origin/main` al preflight. Este corte continúa la autorización expresa de trabajar/publicar el flujo de imágenes en main sin pruebas manuales previas. No se omiten hooks ni CI; no hay PR, force-push, migraciones nuevas, IAM ni cambio de controles de acceso. La QA online sigue reservada al usuario.

**Estado de este recibo:** corrección de fuente preparada y revisión independiente `deleg_83a402f3` aprobada sin defectos concretos de lógica/seguridad; todavía sin publicación ni despliegue de este candidato al redactar el recibo. Las fotos guardadas antes de la corrección no se modifican automáticamente. La pregunta de autorización para una inspección/reparación acotada de las fotos existentes quedó sin respuesta; no se interpreta como consentimiento.

## Causas verificadas en fuente

1. `internal/adapters/postgres/media/repository.go`, `FinalizeSpace`: el INSERT de nuevas relaciones `space_media` omitía `visible`; V68 define el default `false`. El catálogo y la ruta de variantes públicas exigen `sm.visible`. La subida podía terminar `ready` y ser legible para su propietario sin aparecer públicamente.
2. `internal/adapters/postgres/publiccatalog/repository.go`, `List`: la portada se emitía en `images` sin `source`. El adaptador Next `go-public-catalog-adapter.ts`, `publicImages`, descarta referencias sin `source=media|synthetic`. La tarjeta conservaba el ID pero recaía en el endpoint legacy PNG, que no sirve las variantes WebP modernas.
3. `publicImages`: las comparaciones con `portada_foto_id`/`portada_media_id`, ambas anulables, podían producir SQL NULL y fallar al escanear `PublicImageRef.IsCover` de tipo bool cuando no había portada asignada o se mezclaban fuentes.

Estos defectos de fuente no acreditan el estado de las fotos concretas del reporte. No se consultaron filas de negocio, objetos GCS, aplicación pública ni navegador del usuario.

## Corrección acotada

- Nuevas fotos de la galería: INSERT con `visible=true` explícito dentro de la misma transacción que establece `ready` y actualiza la revisión del espacio. No requiere una acción adicional de publicación por foto.
- Borradores/publicaciones ocultas siguen excluidos por la elegibilidad del espacio. Fotos retiradas, assets no listos, espacios ajenos y cuentas no elegibles conservan sus filtros existentes. El bucket sigue privado.
- Los reemplazos continúan heredando `oldVisible`, incluido `false`: reemplazar contenido no revoca una retirada explícita.
- La selección SQL de portada transporta el discriminador `source` hasta el DTO de la tarjeta.
- `COALESCE(...,false)` convierte las comparaciones de portada de ambas fuentes en booleanos no nulos.
- No se cambia el esquema ni el default histórico V68, ni se aplica un backfill indiscriminado. Una reparación de las fotos anteriores requiere autorización y acceso concretos separados.

## Cobertura preparada para CI posterior al push

`internal/migrator/public_gallery_contract_test.go` añade una comprobación de repositorios reales bajo `espacigo_runtime` dentro del harness existente PostGIS aislado, sin red ni puertos publicados. Se conecta desde `assertRuntimeAPISchemaContract` y el verificador de CI exige sus resultados tanto desde esquema vacío como desde V75.

La prueba prepara metadata sintética en su DB descartable y verifica: finalización visible/lista; detalle sin portada nullable; selección de portada y `source=media` en tarjeta; segunda foto visible sin republicación; consulta de variante; retirada explícita y reemplazo que conserva privacidad; exclusión pública de borrador/oculta con acceso privado del propietario. La limpieza elimina solo su nueva cuenta/espacio, respetando la FK de reemplazo antes del postcheck de cero cuentas del harness.

No carga archivos ni prueba GCS, codec, browser/BFF o la publicación del usuario. **No se ejecutó manualmente antes de publicar** por instrucción humana. No se afirma RED/GREEN ni PASS de esta cobertura hasta consultar los resultados automáticos del SHA publicado.

## Revisión y límites

- Revisión del padre: SQL parametrizado; ningún filtro de elegibilidad/visibilidad retirado; no secretos ni rutas nuevas; reemplazo y guards CAS/owner permanecen intactos.
- El diagnóstico delegado frontend inicial citó ubicaciones distintas de la fuente fijada. Se rechazó como evidencia de ese checkout y se contrastó directamente el adaptador, las páginas y la migración V68. No se incorpora su propuesta de exigir un botón de publicación por fotografía: contradice la regla de producto del encargo.
- Implementaciones `deleg_fa9cade7` y revisión independiente `deleg_83a402f3` terminadas. El padre revisó sus diffs y los registros de herramientas de ambos implementadores: no ejecutaron suites manuales ni accedieron a servicios/datos reales. Revisión independiente source-only: `passed=true`, sin hallazgos de lógica o seguridad. La falta de ejecución sigue explícita; la sugerencia temporal del fixture no prueba un fallo reproducido y no cambia fuente tras la revisión.
- `git diff --check` pasó para los cambios ya rastreados; se repetirá con todos los archivos nuevos incluidos en el índice antes del commit.
- No se ejecutaron manualmente suites, builds, typecheck, lint o conexiones SQL. Los controles automáticos de herramientas/hooks existentes permanecen activos; una comprobación automática de archivo aislado indicó que no resolvía la función de prueba ubicada en otro archivo del mismo paquete. No es una compilación del paquete ni un resultado de CI.
- No se declara resuelta la publicación concreta ni aceptado el flujo funcional por el usuario.

## Primer candidato publicado y fallo detectado por CI

Publicado `6514928419135946a2beb9c10617fdab0ed0f077` en main, hooks/identidad `HernanEspinozaDev` y ref remoto releídos; worktree limpio tras push. El trigger exacto `548b8eff-e489-4307-a77f-455b56aa4e57` inició el build canónico `3b7d1888-0648-42b7-ae73-0d2ec386ba67` para ese SHA. No hubo ejecución manual de pruebas antes de publicar.

Frontend CI `43542450479` terminó success. El job backend de Application quality `43542450491` terminó success (tests/vet/build automáticos); su job de imágenes seguía activo al inspeccionar. Database migration quality `43542450615` falló en `TestCurrentSchemaMigrationFromEmptyAndV75/empty/api-schema-contract/public-gallery-sync`: `owner gallery remains available while borrador: [] err=context deadline exceeded`. El build cloud seguía en `verify-source-quality`, con `release-main` queued: no se confundió la publicación Git con un despliegue.

La nueva prueba alcanzó las aserciones precedentes de foto nueva visible, detalle sin portada, tarjeta moderna, segunda foto, variante y reemplazo retirado; no se declara PASS global de esa secuencia porque la prueba terminó fallida. La causa se contrastó en `media.Repository.ListSpace`: solicitaba `r.variants` dentro del bucle con `rows` aún abiertos. El pool limitado a una conexión del harness quedaba retenido por la primera consulta; la consulta anidada esperaba esa conexión hasta vencer el contexto. `GetSpace` y `GetProfilePhoto` usan QueryRow/Scan y `ClaimCleanup` ya cierra sus rows antes de las lecturas secundarias.

Corrección acotada preparada: recoger metadata, comprobar error y cerrar rows antes de consultar variantes, conservando orden, filtros, rechazo sin lista parcial y el pool/test originales. No elevar MaxConns, ampliar timeout, omitir la regresión, tocar la base compartida ni saltar el quality gate. Los fallos posteriores de contexto/roles del mismo cluster descartable no autorizan limpiezas en otra base ni cambios a las guardas de V84. La ejecución siguiente debe volver a completar la suite automática del nuevo SHA.

Implementación incremental `deleg_a5a289d4` verificada en el diff por el padre; revisión independiente `deleg_d680650b` aprobó `ListSpace` sin hallazgos. No se ejecutaron pruebas manuales del candidato incremental. Readback final del primer build: FAILURE a `2026-11-14T01:47:25.002389Z`, `verify-source-quality` FAILURE/exit1; `release-main` permaneció QUEUED sin timing ni exitCode. No se solicitó reintento, desbloqueo, SQL o despliegue manual de esa ejecución.
