# MPV2-00 — Inventario y contrato público

**Fecha:** 2026-10-13. **Baseline auditado:** `a68da0c` (main, PR #293). **Estado:** contrato propuesto para revisión; no implementado.

Este documento fija el punto de partida de MPV2. Los campos/rutas marcados como futuros requieren cambios y no son capacidades disponibles en el baseline.

## Inventario verificado

| Capa | Evidencia en `a68da0c` | Consecuencia del contrato |
| --- | --- | --- |
| Datos | Migraciones V1–V54; categoría inicial en V5, atributos versionados en V6, tarifas CLP versionadas en V10, allowlist de ensayo en V11, estados de publicación en V31, galería sintética en V32 y portada sintética en V49. V50 agrega `eliminada`; V52–V54 son retención/cierre. | No reescribir V1–V54. No hay ubicación real pública región/comuna ni fotos reales. V11 identifica drafts sintéticos restringidos a participantes. |
| Go | `internal/booking/transport/http/handler.go`: `GET /api/v1/local/booking-trial/catalog` y `GET .../catalog/{spaceID}` se autentican antes del despacho. `internal/adapters/postgres/booking/repository.go` permite drafts de fixture a los participantes o publicaciones `activa` de otro propietario cuya cuenta esté activa y KYC elegible. | Crear operaciones públicas separadas del trial de reservas. No reutilizar la autorización por actor ni serializar `CatalogItem` automáticamente como DTO público. |
| Consulta existente | `internal/booking/service.go`: q hasta 120 runas, espacios consecutivos colapsados, controles de caracteres; `repository.go` aplica `strpos(lower(título+descripción+categoría), lower(q))`, filtro de categoría, área mínima, atributos y disponibilidad. | La búsqueda actual es subcadena sensible a acentos, no incluye ubicación y no ofrece región/comuna, precio directo, orden elegible ni idioma/tokenización. |
| Precio existente | `PriceUnits` en `internal/booking`; si hay intervalo se estima total según unidad/zona horaria. Los rangos `min_total_clp`/`max_total_clp` requieren inicio y fin. El valor unitario es CLP entero mayor que 5.000. | La UI distingue precio por hora/día/mes. Un precio de intervalo es estimación del backend, no cotización ni garantía de disponibilidad. No mezclar unidades sin intervalo. |
| Paginación existente | `CatalogPage` pagina después de cargar y ordenar el conjunto en memoria; cursor autenticado, firmado, incluye actor, fingerprint de filtros/modo/tamaño y clave del último elemento. Default 5, máximo 25. | No es apto para lectura pública ni para réplicas/reinicios. V2 define cursor público sin actor y consulta SQL limitada, con orden estable y filtros vinculados. |
| Angular | `app.routes.ts` exige rol arrendatario para `/explorar` y `/espacios/:id`. `features/explore` usa adaptador con Bearer para catálogo, categorías y perfil. `app.routes.server.ts` deja esas rutas en CSR. | Mantener URLs; separar ports/adapters públicos por feature, cargar anónimamente y decidir SSR dinámico en corte posterior. Go mantiene autorización de operaciones privadas. |
| Identidad | `identity-facade.ts` acepta `returnUrl` solo mediante `safeBookingInvitationReturnUrl` para `/reservas?contrato=...`; login por defecto retorna a `/mi-cuenta`. | Agregar un retorno contextual enumerado/validado para ficha y acción elegida, sin reemplazar invitaciones ni aceptar URL externa. No preservar bearer ni ejecutar reserva al volver. |
| Fotos | Galería/portada V32/V49 son sintéticas y sus handlers/repository mantienen autorización del propietario; evidencias privadas tienen almacenamiento/política propia. | No abrir la galería existente. Un futuro handler público sirve únicamente fotos asociadas a publicación visible y revalida cada petición. |

### Códigos y unidades confirmados

V5 inserta y `categoria_espacio.codigo` usa como PK exactamente estos códigos activos iniciales; los nombres provienen de la misma migración. V10 restringe `modalidad` a los tres valores mostrados y `moneda` a CLP:

| Código | Nombre inicial | Unidades admitidas |
| --- | --- | --- |
| `oficina` | Oficina | `hora`, `dia`, `mes` |
| `sala_multiproposito` | Sala o espacio multipropósito | `hora`, `dia`, `mes` |
| `bodega` | Bodega | `hora`, `dia`, `mes` |
| `estacionamiento` | Estacionamiento | `hora`, `dia`, `mes` |
| `local_flexible` | Local flexible | `hora`, `dia`, `mes` |
| `stand` | Stand | `hora`, `dia`, `mes` |
| `quincho` | Quincho | `hora`, `dia`, `mes` |
| `parcela_eventos` | Parcela o espacio para eventos | `hora`, `dia`, `mes` |

Estos son valores sembrados inicialmente, no una promesa de que toda categoría/oferta tenga las tres modalidades en producción. El precio se muestra siempre con su unidad concreta. No se infieren dormitorios/baños ni equivalencias residenciales.

## Predicado de visibilidad pública (una sola regla)

Definir en Go una política de dominio `VisiblePublicamente` que el listado, detalle, facetas, fotos, SSR y sitemap consuman mediante repositorios/servicios comunes. Para el esquema actual, el criterio mínimo verificable es:

1. `espacio.estado = 'activa'`; categoría `activa`.
2. propietario `usuario.estado = 'activo'` y no existe fila en `bloqueo_cuenta_administrativo_local` (bloqueo actual; el historial de desbloqueo no cuenta como bloqueo vigente).
3. KYC de propietario aprobado y elegibilidad tipo `kyc` vigente (`estado='elegible'`, no revocada) en `elegibilidad_verificacion_local`.
4. No pertenece a `reserva_ensayo_local_fixture` habilitado, incluso si un dato inconsistente cambiara el estado del espacio.
5. Tiene tarifa vigente, perfil/atributos compatibles y los campos mínimos publicables requeridos por la siguiente migración.

El UUID del visitante y que sea propietario no cambian la proyección: mismas respuestas anónimas/autenticadas y puede consultar su propia publicación activa desde el catálogo público. La allowlist solo autoriza el ensayo a sus dos participantes; nunca autoriza difusión.

**Brecha de modelo por resolver en el primer corte de implementación:** no se encontró una entidad de moderación de publicaciones ni una bandera de retirada de difusión independiente de `estado`. V50 `eliminada` y `oculta` sí excluyen; la retirada de publicación no equivale a moderación. No declarar moderación, consentimiento publicitario ni retirada de privacidad ya operativos. Antes de ampliar el predicado, decidir si basta ocultar publicación o se requiere estado/bitácora específico y diseñarlo como delta nuevo.

La retirada efectiva incluye publicación oculta/eliminada, cuenta bloqueada o inactiva, KYC/elegibilidad revocados y fixture habilitado. Listado, detalle y foto devuelven ausencia/404 genérico sin explicar cuál condición falló. Toda lectura pública (incluidas búsquedas sin disponibilidad) es GET sin Bearer/cookie y de solo lectura: no llamar `Expire`, crear cotizaciones, cambiar estados, ocupar/liberar inventario ni generar auditoría de actor. La ruta actual `Catalog` llama `repo.Expire` antes de consultar: separar el efecto de vencimientos antes de habilitar anónimos. La verificación de disponibilidad puede leer ocupaciones y devolver un booleano derivado, pero no alterarlas.

## Matriz de campos

| Campo/fuente | Tratamiento público propuesto | Regla |
| --- | --- | --- |
| `espacio.id` | Sí, identificador opaco estable | Nunca sirve como autorización. |
| `categoria_codigo`, nombre | Sí | Solo categoría activa; códigos exactos arriba. |
| `titulo`, `descripcion`, `reglas_uso` | Sí, después de reglas de publicación/moderación por definir | Contenido redactado para difusión; validar longitud y no copiar datos privados de fixtures. |
| `superficie_m2`, `capacidad_maxima` | Sí cuando el dueño los marque/completen para la ficha | Son campos del espacio hoy; verificar coherencia por categoría. |
| perfil/valores de `espacio_caracteristicas` | Lista permitida por campo, no JSON entero | V6 perfiles actuales tienen `filter_candidate:false`; publicar/filtrar solo atributos revisados explícitamente. |
| tarifa vigente, moneda, unidad | Sí | Precio exacto `int64` CLP; mostrar unidad y fecha/alcance de estimación. No redondear a `Number` inseguro en JS. |
| zona horaria | Solo para cálculo/render de intervalos | No revela dirección por sí sola; limitar metadatos innecesarios en la tarjeta. |
| región/comuna/etiqueta pública | Sí, después de capturarla por separado | No existe fuente de comuna/región en V1; elegir catálogo/códigos válidos antes de migrar. |
| disponibilidad | Solo booleano/estado acotado para intervalo consultado, si el contrato la necesita | Nunca enviar ocupaciones, reservas o identidades; la lectura no muta. |
| foto de publicación | Solo imagen marcada explícitamente como foto de publicación, ligada al espacio, vigente y visible por el predicado común | Servir mediante GET controlado, revalidar retiro en cada descarga, tipo/tamaño permitidos. No exponer ruta de blob ni ID de evidencia. V32/V49 sintéticas no son una galería pública y no se habilitan por inferencia. |
| dirección exacta/coordenada precisa | Privado | No seleccionar en DTO, HTML, logs públicos ni sitemap. |
| propietario/UUID, email, teléfono, cuenta/bloqueo, KYC/KYB, documentos | Privado | Tampoco divulgar motivo de rechazo o elegibilidad. |
| fixture/participantes, tarifa histórica, cotizaciones, reservas, ocupantes, bloqueos, sesiones, auditoría, IDs/rutas de blob y evidencia | Privado | No serializar agregados/repositorios completos. |

## Contrato de búsqueda, filtros, precio y páginas

Rutas públicas canónicas para Go/OpenAPI y el adaptador Angular:

| Operación | Corte | Contrato |
| --- | --- | --- |
| `GET /api/v1/public/spaces` | MPV2-01 base; filtros de texto/ubicación/atributos en MPV2-02 | Lista DTO públicos; `category_code`, modalidad/precio base y página desde MPV2-01. |
| `GET /api/v1/public/spaces/{spaceID}` | MPV2-01 | Ficha pública; 404 genérico si no visible. |
| `GET /api/v1/public/space-categories` | MPV2-01 | Categorías activas: código, nombre y orden. |
| `GET /api/v1/public/space-categories/{code}/filters` | MPV2-02 | Solo filtros de atributos aprobados y versión pública del perfil. |
| `GET /api/v1/public/locations` | MPV2-02 | Códigos/nombres región-comuna; referencias locales, búsqueda/sugerencias acotadas. |
| `GET /api/v1/public/spaces/{spaceID}/images/{imageID}` | MPV2-04 | Imagen de publicación explícitamente pública, vinculada y vigente; 404 genérico si no visible. |

Todas las lecturas públicas deben responder sin Authorization ni cookies; si se adjunta sesión no cambia la proyección. No migrar las rutas privadas `/api/v1/local/booking-trial/*` a acceso público. No exponer rutas `/photos` o `/categories` paralelas con otro significado.

- `q` (MPV2-02): máximo 120 caracteres Unicode; trim, NFC, espacios colapsados; búsqueda de texto PostgreSQL con configuración `espacigo_es_unaccent` (diccionario `unaccent` más stemming español). `plainto_tsquery` aplica AND entre lexemas normalizados (no subcadena literal); solo busca título, descripción pública, nombre/código de categoría y zona pública una vez exista. `ts_rank` sirve para `sort=relevance`; GIN limita candidatos. Stopwords/puntuación que normalicen a consulta vacía devuelven 200 con cero resultados. El documento tsvector y query excluyen dirección/contacto. Requiere validar/instalar configuración de búsqueda en la migración nueva; no usar extensión/servicio externo ni fallback silencioso a semántica distinta.
- `category_code`: un código activo por solicitud desde MPV2-01; sirve como filtro, nunca como paso de acceso. Múltiples categorías quedan evolutivas y no forman parte del primer contrato.
- `min_area_m2`: inclusivo y positivo; capacidades/atributos se admiten solo si el perfil público aprobado los marca explícitamente; booleanos comparan `true`/`false` exactos y la versión del perfil se resuelve en servidor.
- Tarifa base (MPV2-01): `rate_unit=hora|dia|mes` selecciona modalidad; `min_unit_price_clp`/`max_unit_price_clp` son inclusivos, entero CLP exacto, `0 ≤ min ≤ max` cuando ambos están; cualquier filtro o sort por tarifa unitaria requiere `rate_unit`. Se compara solo el mismo valor de modalidad y nunca se convierte hora/día/mes. `sort=unit_price_asc|unit_price_desc` requiere modalidad. Se puede usar sin fechas.
- Total estimado (MPV2-02, opcional y separado): `start_at` y `end_at` RFC3339 son ambos obligatorios si se usa `min_total_clp`, `max_total_clp` o `sort=estimated_total_asc|estimated_total_desc`; cálculo Go con `PriceUnits`, TZ y tarifa vigente, CLP entero y límites inclusivos. Puede combinarse con filtro de tarifa base y modalidad. No es cotización ni garantiza disponibilidad.
- Zona (MPV2-02): `region_code` y `comuna_code` son filtros exactos desde referencia pública. No se aceptan coordenadas/radio: el baseline solo tiene puntos sintéticos y no se deriva ubicación desde dirección privada.
- Orden: `sort` admite `default` (categoría `orden`, título normalizado, `space_id`), `title_asc|title_desc` (título normalizado y `space_id` estable), `unit_price_asc|unit_price_desc` (modalidad requerida), `estimated_total_asc|estimated_total_desc` (intervalo requerido) y `relevance` (requiere q, MPV2-02). No se ordena por distancia. Precio compara solo una modalidad CLP explícitamente elegida; no mezcla hora/día/mes. Todo orden acaba con ID estable.
- Página: `page_size` opcional, default decidido 20, máximo 25; cursor opaco/versionado firmado y validado contra criterios normalizados, orden y tamaño; no ligado a actor, sesión ni secreto. SQL aplica visibilidad/filtros/orden/cursor con `LIMIT n+1`; no carga todo el inventario en memoria. Cursor por configuración/firma estable entre réplicas/reinicios. Sin snapshot: altas/retiradas entre páginas pueden mover filas, pero cada nueva página revalida visibilidad.
- HTTP 422 `invalid_request` genérico para parámetro desconocido/duplicado, q mayor a 120, código inactivo/desconocido, valor fuera de rango, modalidad ausente o incompatible para precio, intervalo incompleto, fin no posterior a inicio, min mayor que max, sort sin requisitos o cursor/tamaño inválido (`page_size` debe ser 1–25 si aparece). HTTP 404 `not_found` genérico para espacio/imagen inexistente o no visible. Angular limpia cursor al cambiar filtros.

## Delta de datos y secuencia

**MPV2-00:** sin migración. No hay tablas ni permisos modificados.

**Diseño por corte, no ejecutado aún:** MPV2-01 puede no tener migración si los campos actuales bastan; contiene precio unitario y modalidad existentes, filtro de modalidad/base, categoría y listado/detalle público. En MPV2-02 se agrega ubicación pública separada, nullable y con códigos de fuente versionada; jamás derivada de dirección ni coordenadas. Se mantiene oferta legacy con zona NULL y etiqueta «Zona no indicada»; solo se excluye de filtro regional. No hay backfill geográfico ni default inventado. El mismo corte define tsvector/Gin con configuración española-unaccent, cursor/índices necesarios medidos; atributos filtrables solo si se aprueban explícitamente. MPV2-04 requerirá una marca explícita de imagen publicable y handler controlado; no backfill de galería/portada V32/V49. El estado independiente de moderación, si producto lo requiere, es delta posterior; no existe hoy. Migraciones V1–V54 no se tocan.

Mantener tablas actuales como fuente de estado/tarifa/atributos. No duplicar `estado`, tarifa ni unidad en tabla paralela. Migración nueva aditiva, idempotencia según estándar del repo, actualización desde V54 y base vacía probadas en stack efímero.

## Angular, contextualidad y renderizado

- Conservar `/explorar` y `/espacios/:id`; eliminar el guard solo tras existir endpoint/DTO público real. Feature `explore` debe tener puerto público independiente de `hosting` y del trial privado; la presentación no hace HTTP. Adaptador OpenAPI tipado; filtros serializados por codec canónico en query params; Signals en estado de vista.
- Navegación ficha→login/registro/verificación debe guardar retorno interno validado con allowlist: ID de espacio, acción permitida y retorno a `/explorar` con filtros normalizados. Prohibir origen/esquema externo, `//`, credenciales y parámetros ambiguos. Preservar el retorno existente `/reservas?contrato=...`. Tras autenticar, Go revalida espacio, rol, elegibilidad, precio y disponibilidad; solicitar confirmación y no enviar una mutación automáticamente.
- `/explorar` y ficha son candidatas a SSR dinámico en MPV2-07, no hoy. Estado por petición, endpoint público sin token, TransferState solo con DTO público; 404 para no visible, 503 temporal para backend no disponible. No prerenderizar combinaciones del catálogo ni incluir filtros/cursor en sitemap. Parámetros de API internos solo en servidor; canonical solo con origen público.

## Alcance decidido por corte

- MPV2-01: listado/detalle GET sin sesión, filtro por una categoría, listado de categorías activas, DTO mínimo con tarifa base CLP y unidad, filtro/orden de tarifa base con modalidad explícita, página default 20/máximo 25 y cursor SQL estable. Sin fecha obligatoria ni estimación; sin q, comuna, atributos, SSR ni foto pública.
- MPV2-02: q PostgreSQL FTS español/acento-insensible, total estimado opcional por intervalo, ubicación pública y filtro región/comuna, perfil/filtros de atributos aprobados, ranking textual. `GET /space-categories/{code}/filters` y `/locations` pertenecen a este corte.
- MPV2-04: única ruta pública de imágenes con elegibilidad explícita y verificación en cada GET.
- MPV2-07: SSR/SEO.

Pendientes de producto que no alteran el contrato MPV2-01: fuente/catálogo de códigos de región/comuna y decisión de etiqueta libre; mínimos por categoría para publicar; decidir si se requiere moderación/retiro independiente de ocultar. El tamaño de página (20/25), semántica dual de precio, búsqueda española FTS y nombres de rutas están decididos arriba; no requieren confirmación para implementarse.

## Verificaciones realizadas en MPV2-00

Auditoría de fuente a `a68da0c`; `git fetch origin --prune` y fast-forward de `main`; `scripts/git-role.sh development` reportó `gh`, Git y autor como HernanMEC; hook `.githooks` instalado localmente. No se ejecutaron pruebas, migraciones ni servicios y no se tocó `espacigo_pgdata`.
