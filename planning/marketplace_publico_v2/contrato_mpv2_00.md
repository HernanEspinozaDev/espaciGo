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

Este contrato se implementará en operaciones nuevas `GET /api/v1/public/spaces`, `GET /api/v1/public/spaces/{spaceID}`, `GET /api/v1/public/categories` y `GET /api/v1/public/spaces/{spaceID}/photos/{photoID}` (nombres finales quedan versionados en OpenAPI). Deben funcionar sin Authorization ni cookies; si una sesión se adjunta, no cambia la proyección. Categorías entrega solo código/nombre/orden activos y los campos de filtro públicamente admitidos. No migrar las rutas privadas `/api/v1/local/booking-trial/*` a acceso público.

- `q`: opcional, máximo 120 caracteres Unicode; trim, NFC y colapso de espacios; coincidencia literal de subcadena, insensible a mayúsculas y acentos. Busca solo título, descripción pública y nombre/código público de categoría; `%` y `_` son literales. El índice/normalizador se definirá en DB/API para que búsqueda y paginación usen idéntica clave; no puede incluir dirección/contacto. Es mejora explícita respecto al `lower/strpos` sensible a acentos actual.
- `category_code`: un código activo por solicitud; sirve como filtro, nunca como paso de acceso. Múltiples categorías requieren una decisión/contrato aparte.
- `min_area_m2`: inclusivo y positivo; capacidades/atributos se admiten solo si el perfil público aprobado los marca explícitamente; booleanos comparan `true`/`false` exactos y la versión del perfil se resuelve en servidor.
- Zona: región/comuna exactas por código canónico una vez elegida fuente. No se aceptan coordenadas/radio públicos: baseline solo tiene puntos sintéticos de ensayo y la dirección privada no puede geocodificarse automáticamente.
- Precio de unidad: sin intervalo se puede filtrar/ordenar solo dentro de una modalidad explícitamente seleccionada; sin mezclar horas/días/meses. **Opción recomendada:** mantener filtros mínimo/máximo sobre total estimado con intervalo obligatorio, ya existentes en forma privada. Estimación respeta `PriceUnits`, zona horaria y tarifa actual; CLP entero, bordes inclusivos y mínimo ≤ máximo. No representa cotización ni disponibilidad garantizada. Exponer ambas semánticas simultáneamente requiere parámetros distintos.
- Orden inicial: relevancia no existe aún. Sin intervalo usar categoría `orden`, título normalizado, `space_id`; con intervalo y orden de precio usar total estimado ascendente y `space_id`. Orden desconocido se rechaza (422); no se ofrecerá distancia sin localización pública.
- Página: `page_size` opcional default 20, máximo 25 (cambio respecto al endpoint privado default 5); cursor opaco/versionado firmado, enlazado a filtros normalizados, orden y tamaño, no a actor ni secretos. SQL aplica visibilidad/filtros/orden/cursor y `LIMIT n+1`; empates incluyen ID estable. Query pública no recorre primero todo el inventario en memoria. Parámetros no permitidos, categorías inactivas/desconocidas, límites fuera de rango, atributos no públicos/incompatibles, rango invertido, fecha ausente, tamaño fuera de 1–25 y cursor alterado/expirado dan HTTP 422 `invalid_request` genérico. Detalle/foto inexistente o no visible da HTTP 404 `not_found` genérico. Angular reinicia el cursor al cambiar criterios. Sin garantía de snapshot: alta/retirada entre páginas puede mover filas, pero nunca exponer retiradas en una nueva lectura.
- Solicitudes inválidas, incluyendo rango negativo/invertido, categoría inactiva, atributos incompatibles, tamaños excedidos, orden no soportado o cursor alterado, reciben error estable sin filtrar por actor. Detalle inexistente/no visible tiene el mismo 404 genérico.

## Delta de datos y secuencia

**MPV2-00:** sin migración. No hay tablas ni permisos modificados.

**Diseño a llevar a MPV2-01/02, no ejecutado aún:** (a) agregar ubicación pública separada de `espacio.direccion`, con región/comuna nullable y códigos tomados de fuente explícita; jamás derivarlos automáticamente de dirección ni aceptar coordenadas exactas en tabla pública. Nueva columna/relación queda `NULL` al migrar V54; no poblar con parseo, geocodificación ni default inventado. Permitir ofertas legacy sin zona pública (mostrar «Zona no indicada» y excluirlas solo de filtros regionales), salvo que el producto decida que ubicación sea requisito de publicación. El anfitrión completa la ubicación pública desde el editor. (b) requerir una foto marcada expresamente como publicable; no backfill de fotos V32/V49. (c) revisar requerimiento para distinguir contenido publicable/retiro por privacidad/moderación, pues hoy no hay entidad de moderación publicitaria. (d) índices para consulta de ofertas visibles por estado/categoría, orden/filtros más usados y FK de localización; revisar con `EXPLAIN` sobre DB efímera antes de fijarlos. (e) si se habilitan atributos como facetas, cambiar definición pública explícita versionada; no flip indiscriminado del `filter_candidate` actual. (f) no backfill de datos persistentes existentes a público ni siembra geográfica sin revisar origen, consentimiento y valores.

Mantener tablas actuales como fuente de estado/tarifa/atributos. No duplicar `estado`, tarifa ni unidad en tabla paralela. Migración nueva aditiva, idempotencia según estándar del repo, actualización desde V54 y base vacía probadas en stack efímero.

## Angular, contextualidad y renderizado

- Conservar `/explorar` y `/espacios/:id`; eliminar el guard solo tras existir endpoint/DTO público real. Feature `explore` debe tener puerto público independiente de `hosting` y del trial privado; la presentación no hace HTTP. Adaptador OpenAPI tipado; filtros serializados por codec canónico en query params; Signals en estado de vista.
- Navegación ficha→login/registro/verificación debe guardar retorno interno validado con allowlist: ID de espacio, acción permitida y retorno a `/explorar` con filtros normalizados. Prohibir origen/esquema externo, `//`, credenciales y parámetros ambiguos. Preservar el retorno existente `/reservas?contrato=...`. Tras autenticar, Go revalida espacio, rol, elegibilidad, precio y disponibilidad; solicitar confirmación y no enviar una mutación automáticamente.
- `/explorar` y ficha son candidatas a SSR dinámico en MPV2-07, no hoy. Estado por petición, endpoint público sin token, TransferState solo con DTO público; 404 para no visible, 503 temporal para backend no disponible. No prerenderizar combinaciones del catálogo ni incluir filtros/cursor en sitemap. Parámetros de API internos solo en servidor; canonical solo con origen público.

## Pendientes de producto

1. Seleccionar fuente/catálogo oficial o mantenido para códigos de región/comuna; resolver si se permite una etiqueta libre cuando falte dato.
2. Confirmar si el filtro de precio visible en catálogo será total estimado para fechas seleccionadas (recomendado) o precio unitario por modalidad. El plan/matriz ya asumen total estimado si hay intervalo.
3. Definir qué campos de superficie/capacidad son imprescindibles para publicar en cada categoría y qué atributos de V6 pueden presentarse públicamente.
4. Definir política explícita de moderación/retiro de contenido si se requiere más que `oculta` y revocación KYC/cuenta.
5. Confirmar tamaño default de página pública 20 (recomendado) frente a mantener 5 del endpoint de ensayo.

## Verificaciones realizadas en MPV2-00

Auditoría de fuente a `a68da0c`; `git fetch origin --prune` y fast-forward de `main`; `scripts/git-role.sh development` reportó `gh`, Git y autor como HernanMEC; hook `.githooks` instalado localmente. No se ejecutaron pruebas, migraciones ni servicios y no se tocó `espacigo_pgdata`.
