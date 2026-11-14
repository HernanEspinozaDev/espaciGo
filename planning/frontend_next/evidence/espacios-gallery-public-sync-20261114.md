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
