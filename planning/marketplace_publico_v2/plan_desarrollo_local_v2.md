# Desarrollo local versión 2 del marketplace público

**Identificador:** MARKETPLACE-LOCAL-2. **Cortes:** MPV2-00–MPV2-13.
**Fecha:** 2026-10-14. **Estado:** MPV2-00–12 fusionados técnicamente. MPV2-12 se revisó en [PR #306](https://github.com/HernanEspinozaDev/espaciGo/pull/306) y fusionó por `HernanEspinozaDev`, merge `c797f0eb734e2c7eda37eb70a50d4d509e82677d`. MPV2-13 es el corte vigente sobre ese SHA. La aceptación de producto sigue pendiente.

## 1. Objetivo y base de trabajo

Una persona sin cuenta debe poder entrar a EspaciGo, buscar espacios, filtrar por categoría y características, consultar tarjetas y fichas públicas y compartirlas. La autenticación se solicita al comenzar una reserva u otra operación privada realmente implementada. Al identificarse, conserva el espacio y los criterios elegidos, y Go vuelve a validar la operación.

La base de código inicial de MARKETPLACE-LOCAL-2 fue `a68da0c` (main, merge de #293), con Angular como frontend predeterminado, mock retirado y migraciones hasta V54. V55 se agregó en MPV2-02 y V1–V55 se conservan byte a byte. MPV2-04 se fusionó técnicamente en #298, merge `c3e5cd297a32863446df33d271931aacb6fc2a8b`; agrega V56 sin reescribir historial. MPV2-05 parte de ese merge de main, sin cambios de backend ni migraciones. El inventario/contrato está en [MPV2-00](contrato_mpv2_00.md).

Al comenzar la implementación se debe actualizar la referencia de `origin/main` y registrar su SHA exacto. No desarrollar sobre el checkout antiguo por coincidencia de ruta. Conservar los cambios locales ajenos y usar una rama `codex/` desde la base vigente.

Fuentes: conversación del usuario; [especificación aportada](referencias/especificacion_marketplace_publico_inmobiliario.md); decisiones arquitectónicas originales en `planning/frontend/referencias/decisiones_frontend_original.md`; guías SCSS; código y migraciones del baseline anterior. La especificación es orientativa: no demuestra que una capacidad exista ni reemplaza las reglas verificadas en Go.

## 2. Diagnóstico del estado actual

| Área | Evidencia en el código inspeccionado | Cambio necesario |
| --- | --- | --- |
| Acceso a catálogo | `frontend/src/app/app.routes.ts`: `/explorar` y `/espacios/:id` usan `roleGuard` con rol arrendatario. | Separar lectura pública de acciones privadas. |
| Estado de búsqueda | `features/explore/application/explore-facade.ts` requiere token, sesión, rol y modo no restringido. | Fachada pública que funcione sin identidad; no utilizar un Bearer oculto. |
| Transporte | `features/explore/infrastructure/` envía Bearer al catálogo, categorías y perfiles. | Puerto/adaptador público y operaciones OpenAPI públicas. |
| Categorías | `features/categories/categories.html` describe categorías y manda a `/acceso`; `home/categories.data.ts` es editorial. | Categorías operativas como filtros con códigos de Go y URL compartible. |
| Catálogo Go | `internal/booking/transport/http/handler.go` y `internal/booking/service.go`: lectura ligada al actor UUID. | Caso de lectura pública independiente del actor. |
| Persistencia | `internal/adapters/postgres/booking/repository.go`: combina borradores habilitados para participantes de fixtures y publicaciones normales `activa` con elegibilidad del propietario. | Consulta pública que excluya completamente borradores de fixtures y evalúe la política de difusión. |
| Búsqueda existente | Texto por coincidencia de subcadena en título/descripción/categoría; atributos JSONB; superficie; intervalo/precio; geografía de fixtures. | Reutilizar validaciones útiles; búsqueda pública indexada y semántica explícita. |
| Paginación | `CatalogPage` obtiene el conjunto de resultados y pagina en Go; cursor ligado al actor y clave de la instancia. | Paginar en SQL y definir cursores públicos que funcionen entre réplicas/reinicios. |
| Oferta | V31/V50: estados `borrador`, `activa`, `oculta`, `eliminada`; perfiles versionados, tarifas y ocupaciones existentes. | Conservar estados y fuente única de oferta; no duplicar inventario. |
| Imágenes | V32/V49 y `internal/gallery/transport/http/handler.go`: PNG sintético, galería/portada privadas del anfitrión. | Lectura pública explícita de imágenes de una publicación visible. No abrir toda la galería privada. |
| Ubicación | Dirección privada en `espacio`; geografía actual en `reserva_ensayo_local_ubicacion_sintetica`. | Datos de zona pública independientes; no geocodificar la dirección privada automáticamente. |
| Renderizado | `app.routes.server.ts` prerenderiza portada, categorías y cómo funciona; resto CSR. | SSR real para catálogo/ficha dinámicos y manejo de errores HTTP. |
| Retorno de identidad | `identity-facade.ts` permite un retorno específico `/reservas?contrato=...`. | Ampliación acotada para contexto de espacio, manteniendo invitaciones y seguridad. |

Conclusión: se necesita trabajo en **base de datos, backend y frontend**. La implementación V1 cumplía su alcance autenticado; esta versión cambia de forma explícita esa frontera de producto.

## 3. Decisiones de producto de V2

### 3.1. Lo que cambia

- Catálogo, búsqueda, filtros, ficha, imágenes de publicación y metadatos son públicos.
- La landing incluye un buscador funcional y accesos a catálogo filtrado.
- Las categorías seleccionan resultados; no forman una barrera previa al catálogo.
- Filtros y orden se representan en la URL; volver desde una ficha restaura criterios.
- Autenticación contextual al iniciar una acción privada, sin perder el espacio seleccionado.

### 3.2. Adaptación al producto real

Se conservan los códigos insertados en V5: `oficina`, `sala_multiproposito`, `bodega`, `estacionamiento`, `local_flexible`, `stand`, `quincho`, `parcela_eventos`. MPV2-00 los confirmó contra la migración y el uso de catálogo; no introducir alias no respaldados. No convertir el producto en arriendos residenciales ni agregar casa/departamento como categorías por copiar los ejemplos de la propuesta. Las modalidades CLP son `hora`, `dia`, `mes` según V5/V10; cada oferta publica su unidad vigente, no se presume que cada categoría tenga toda unidad.

CLP y modalidades **hora/día/mes** permanecen. Los precios de tarjetas muestran su unidad; no se normalizan todas las tarifas a un mes. Baños, equipamiento u otros atributos aparecen como filtros solo si el perfil de la categoría los admite. No se inventan dormitorios, mascotas, UF o datos ausentes.

El CTA inicial será **«Ver disponibilidad y reservar»** cuando el flujo actual lo permita. No mostrar visitas o postulaciones como implementadas: requieren un futuro alcance con modelo, API, permisos y criterios propios. El plan conserva contratos, fakes financieros y operaciones de arriendo existentes.

Se mantiene la paleta Terracota + Crema, con tokens vigentes y controles accesibles. Las fotos de ensayo siguen siendo sintéticas y se identifican en la demo. Subida de fotos reales, mapas externos, GCP, proveedores reales, dinero real, firma con validez legal, UF y ranking patrocinado quedan fuera de esta versión local.

### 3.3. Rutas

Se conservan `/explorar` y `/espacios/:id` para evitar romper enlaces. La primera es el catálogo público y la segunda la ficha pública por identificador estable de publicación. Un ID de espacio no es una credencial; la autorización depende de Go.

`/categorias` puede conservar una explicación editorial y enlaces a `/explorar?category_code=...`; las categorías filtrantes también están en la landing y la PLP. No obliga a pasar por esa página. No es obligatorio renombrar a `/arriendos` ni introducir slugs para entregar valor. Slugs legibles serán evolutivos si se necesitan; deben tener identificador estable y política de redirección, sin copiar direcciones ni datos personales.

Ejemplo canónico de URL de producto: `/explorar?category_code=oficina&rate_unit=hora&min_unit_price_clp=10000&sort=unit_price_asc&page_size=20`. MPV2-02 agrega, con los mismos nombres en URL/OpenAPI, `q`, `region_code` y `comuna_code`; el codec Angular no inventa alias. Parámetros vacíos/default se omiten y los valores se normalizan de forma determinista.

## 4. Política de visibilidad y minimización

### 4.1. Predicado único de lectura pública

Listado, ficha, fotos, sugerencias derivadas del inventario, facetas y sitemap deben aplicar el predicado común fijado en [contrato MPV2-00](contrato_mpv2_00.md). Condiciones verificadas: estado activo y categoría activa; cuenta activa y sin fila de bloqueo administrativo; KYC aprobado/elegibilidad vigente; no pertenecer al fixture de ensayo; tarifa/perfil válidos. `oculta`/`eliminada`, bloqueo, KYC/elegibilidad revocados y fixture retiran la lectura con respuesta genérica. El esquema carece de un estado de moderación publicitaria independiente; no afirmar que esté resuelto por migraciones existentes.

El catálogo no depende del UUID del visitante ni excluye la publicación por ser su propietario. La lectura anónima y autenticada tiene la misma proyección pública. El propietario conserva su editor privado y allí puede ver sus borradores.

La publicación activa actual pasa a ser potencialmente difundible mediante esta nueva API. Antes de probar sobre datos persistentes, inventariar qué campos y publicaciones quedarían expuestos. Los ensayos iniciales usan únicamente datos sintéticos desechables; no convertir datos existentes en públicos mediante un backfill indiscriminado.

### 4.2. DTO público deliberado

**Permitidos:** identificador estable de espacio/publicación, categoría/código/nombre, título y descripción pública, reglas públicas, superficie/capacidad cuando corresponda, atributos permitidos, tarifa vigente/moneda/unidad, zona IANA, región/comuna o etiqueta general, imágenes de publicación y estado de disponibilidad derivado si se consulta un intervalo.

**Excluidos:** dirección exacta, coordenada precisa, email, teléfono, UUID del propietario, documentos KYC/KYB, rutas de archivo, identificadores internos de blobs, hash de evidencias privadas, datos de cobro, sesiones, historial administrativo, ocupantes, IDs de reservas y detalles de bloqueos. No serializar `Draft` ni `CatalogItem` completos por conveniencia. Descripción/reglas son contenido público intencional del anunciante; el editor lo explica y evita datos de contacto en los fixtures. Si se incorpora texto de usuarios reales, su moderación requiere un alcance definido.

Ubicación pública inicialmente textual (región/comuna). Mapa público no es requisito del cierre. Si se añade posteriormente, una posición aproximada separada debe evitar reconstruir la dirección mediante distancia exacta; no reutilizar la ubicación precisa del fixture como si fuese pública.

No se publica automáticamente la reputación de una cuenta ni reseñas con identificadores de participantes. Las reseñas públicas existentes se auditan antes de integrarlas y, si requieren minimización, se corrigen con su propia prueba; no son condición del primer catálogo público.

### 4.3. Retirada y caché

Primera entrega: lectura pública `Cache-Control: no-store`, sin caché persistente de datos, HTML ni fotos. Cada descarga verifica publicación y estado de imagen. Ocultar, retirar foto, bloquear propietario o revocar elegibilidad impide **nuevas lecturas**, incluidos SSR y sitemap. No prometer borrar contenido que un visitante ya descargó.

No copiar fotos a un directorio estático público ni emitir enlaces permanentes que salten esa verificación. Evitar almacenar públicamente respuestas privadas en cachés compartidas. Futuras cachés requieren política explícita de invalidación y plazo de retirada con pruebas propias.

## 5. Datos y búsqueda: solución propuesta

### 5.1. Persistencia incremental

Reutilizar `espacio`, categorías/perfiles/atributos versionados, tarifas, galería, historial de publicación y ocupaciones. No crear una segunda tabla de propiedades que replique el inventario.

Agregar por migraciones **nuevas posteriores a V54**, reservando la numeración vigente al implementarlas:

- Ubicación pública validada: región/comuna y nombre público; tablas de referencia locales con códigos estables y FK. No descomponer automáticamente la dirección privada para obtener comuna.
- No agregar fecha de publicación para ordenar el catálogo inicial: su orden es categoría/título/ID. Agregar versión de contenido solo si una necesidad probada de retirada/moderación la requiere.
- Metadatos o vista/proyección para seleccionar contenido público y atributos habilitados. La política de visibilidad no se resuelve con un flag independiente que pueda quedar activo tras ocultar el espacio.
- Documento de búsqueda basado solo en campos públicos (título, descripción, categoría, zona). Índices para texto, orden, categoría, zona y tarifa según consultas reales.

Los metadatos nuevos empiezan nullable cuando los datos anteriores no permitan deducirlos. Una publicación sin zona declarada puede seguir mostrándose como «Zona no indicada», pero no coincide con un filtro de comuna. El anfitrión debe poder completarlos desde Angular; no dejar el mantenimiento limitado a SQL manual.

No modificar V1–V54 ni sus checksums. Probar actualización desde V54 y arranque de base vacía. Documentar backfill, restricciones, permisos del usuario runtime y reversión mediante nueva migración si fuera necesaria; el historial de reservas/contratos no cambia.

### 5.2. Método de búsqueda

Usar PostgreSQL inicialmente. En MPV2-02 buscar con configuración de texto `espacigo_es_unaccent`: `unaccent` más stemming español, `to_tsvector` materializado/actualizado desde campos públicos y GIN; `plainto_tsquery` con AND de lexemas para `q`; `ts_rank` para orden `relevance`. Trim/NFC/espacios se normalizan igual al indexar y consultar. No subcadena literal ni fallback silencioso a `strpos`. No añadir Elasticsearch ni servicio externo. Si la imagen PostgreSQL del repo no incluye `unaccent`, MPV2-02 añade/verifica la extensión en su migración; ese corte debe probar su instalación antes de aceptar.

- Buscar título, descripción pública, categoría y zona pública; nunca dirección privada ni datos KYC.
- Texto máximo propuesto: 120 caracteres, sin controles. Consulta parametrizada, timeouts y tamaño máximo; sin SQL construido a partir del usuario.
- Coincidencias de palabras y ranking explicitados mediante fixtures; no prometer tolerancia a errores, sinónimos o autocompletado avanzado si no se implementan.
- Ubicación por códigos de región/comuna; sugerencias de ese catálogo local. No enviar la búsqueda a un geocodificador externo.
- Filtros AND entre dimensiones; `category_code` recibe exactamente una categoría en MPV2-01/02. Selección múltiple se deja evolutiva y requerirá otro parámetro/versionado OpenAPI. No mezclar atributos de perfiles incompatibles.
- Perfiles versionados: campos filtrables y públicos con tipos, límites y unidades. No reinterpretar atributos guardados bajo versiones antiguas.

### 5.3. Precio y disponibilidad

Contrato elegido (parámetros canónicos en MPV2-01/02):

- Tarifa base: `rate_unit=hora|dia|mes` filtra modalidad; `min_unit_price_clp`/`max_unit_price_clp` filtran tarifa vigente inclusivamente en CLP, sin fechas. Cualquier rango u orden de tarifa base exige seleccionar unidad; `sort=unit_price_asc|unit_price_desc` solo compara la misma unidad.
- Total estimado (MPV2-02, opcional): requiere `start_at` y `end_at`; `min_total_clp`/`max_total_clp` filtran el total calculado por Go mediante `PriceUnits` y zona horaria, sin crear cotización. `sort=estimated_total_asc|estimated_total_desc` requiere intervalo. Puede combinarse con modalidad/rango de tarifa base. No es cotización ni garantía de disponibilidad.
- Todos los montos son enteros CLP exactos (`int64`), límites inclusivos, min≤max. La UI muestra siempre precio y unidad; Angular no multiplica ni convierte a `Number` impreciso.

Disponibilidad pública, si forma parte del filtro, no incluye ocupantes ni motivos internos. Go revalida tarifa/intervalo al cotizar y reservar. La búsqueda no debe ejecutar transiciones de reservas por ser consultada por un rastreador: hoy `Catalog` llama `repo.Expire`; separar el mantenimiento de vencimientos de la lectura pública y asegurar que GET público no muta reservas/ocupaciones.

### 5.4. Orden y paginación

`sort` acepta exactamente `default` (orden de categoría, título normalizado, ID), `title_asc|title_desc` (título normalizado e ID estables), `unit_price_asc|unit_price_desc` (requiere `rate_unit` explícito), `estimated_total_asc|estimated_total_desc` (requiere intervalo) y `relevance` (requiere q, MPV2-02). Toda opción termina con ID estable. La query de relevancia rankea texto; default no depende de una fecha de publicación que hoy no existe. No mostrar «recientes» ni «recomendados» sin criterio implementado. La UX debe explicar los prerrequisitos y no mezclar CLP por hora/día/mes.

Paginación en SQL: tamaño default propuesto 20, máximo 25, `LIMIT + 1` y cursor basado en clave de orden/ID/fingerprint de filtros. El cursor no contiene Bearer, actor ni identidad privada, no concede acceso y se valida contra los filtros/orden. Usar firma/configuración estable compartida entre réplicas/reinicios. Ante cursor inválido o vencido, 422 genérico y Angular reinicia resultados. Con inventario mutable no prometer un snapshot congelado: documentar la consistencia y probar retirada/alta entre páginas.

Evitar recuperar todo el catálogo para ordenar en Go. Recuentos totales/facetas solo si se calculan de forma correcta y con coste acotado; en su ausencia mostrar «N espacios en esta página» y siguiente página, sin inventar total. Probar planes SQL con dataset sintético y registrar mediciones antes de añadir índices indiscriminados.

## 6. API Go y arquitectura

Crear un caso de lectura pública del catálogo separado del flujo de reservas, con puerto de repositorio y adaptador PostgreSQL. Puede ubicarse en `internal/catalog/` si la auditoría confirma que evita seguir ampliando `booking`. Reutilizar validación de perfiles/precios y el predicado de publicación sin duplicar reglas. El cliente público no debe necesitar un UUID ficticio ni autenticación opcional que cambie datos.

Contrato propuesto en `planning/openapi.yaml` (ajustable en MPV2-00 si se encuentra una ruta pública equivalente):

| Operación | Autenticación | Entrega |
| --- | --- | --- |
| `GET /api/v1/public/spaces` | Ninguna | MPV2-01: tarjeta, categoría, tarifa base/unidad, modalidad, min/max unitario y cursor. MPV2-02: q, ubicación, atributos, total por intervalo y relevancia. |
| `GET /api/v1/public/spaces/{spaceID}` | Ninguna | Ficha pública; 404 genérico para no difundible. |
| `GET /api/v1/public/space-categories` | Ninguna | Taxonomía activa y etiquetas. |
| `GET /api/v1/public/space-categories/{code}/filters` | Ninguna | Perfil público minimizado de filtros admitidos. |
| `GET /api/v1/public/locations` | Ninguna | Catálogo/sugerencias región-comuna, límites de consulta. |
| `GET /api/v1/public/spaces/{spaceID}/images/{imageID}` | Ninguna | Imagen activa perteneciente a esa publicación visible. |
| Cotizaciones/reservas/contratos/pagos actuales | Bearer y autorización existentes | Operaciones privadas; no relajar los permisos. |

No quitar autenticación al handler combinado `/local/booking-trial/`: también ejecuta operaciones privadas y contempla fixtures restringidos. Las rutas anteriores conservan su contrato durante la transición; Angular migra sus lecturas a los nuevos DTO.

Definir límites de q/filtros/cursor, whitelist de orden, códigos de error, contenido binario, tipos de CLP y códigos de zona en OpenAPI; generar el cliente sin editar código generado. Anónimo y visitante con un Bearer inválido siguen pudiendo explorar: Angular no envía Bearer en esas lecturas.

Probar CORS desde origen permitido y SSR sin Origin; OPTIONS correcto; peticiones de mismo origen/rastreador sin cabeceras especiales. Límites de tamaño, frecuencia y consultas pesadas ajustables, 429 coherente y `Retry-After` cuando corresponda. No introducir CAPTCHA o login para navegar. Logs sin secretos ni información privada, con trazabilidad técnica de errores.

## 7. Angular: conservar arquitectura y cambiar experiencia

Mantener Angular 22.2.2 y lockfile del baseline; esta versión no incluye una actualización de dependencias. TypeScript estricto, standalone, Signals, DI, SCSS/tokens y reglas de arquitectura/stylelint existentes. No adoptar un almacén global por defecto.

Feature pública con presentación, fachada de búsqueda/estado, puerto de lectura y adaptador OpenAPI/HttpClient. Reutilizar componentes UI y código de `explore` cuando sus límites sean correctos; no duplicar la aplicación entera. Los componentes no hacen HTTP, la aplicación no contiene URLs ni Bearer, y los modelos de presentación no son DTO privados.

Lectura pública separada del coordinador de acción privada. La sesión no es una condición para renderizar tarjetas. Cambiar sesión o cerrar sesión limpia estado privado pero conserva la búsqueda pública. Generaciones/cancelación por criterios/ruta evitan que una respuesta antigua pise la búsqueda actual o mantenga un loading bloqueado. Cambios de ID/query sobre la misma instancia de ruta deben recargar e invalidar respuestas anteriores.

### PLP y landing

- Buscador operativo en portada; Enter/botón envía q y ubicación al catálogo.
- Categorías como selección filtrante con códigos de Go; sin hardcode de perfiles o catálogo operativo paralelo.
- Filtros a la izquierda en escritorio, diálogo/drawer en móvil con foco, Escape y retorno al botón.
- Texto, categoría, ubicación pública, modalidad, rango CLP, superficie y atributos de categoría respaldados. Intervalo disponible cuando la API lo soporte.
- Chips eliminables, limpiar, orden, paginación y resumen honesto; estados loading, API caída, sin resultados y cursor inválido diferenciados.
- La URL es la fuente del estado confirmado; formulario móvil puede tener borrador hasta Aplicar. Cambiar filtros reinicia cursor. Atrás/adelante y enlace directo restauran resultados.
- No usar filtros de fecha/precio que el servidor ignore; no presentar inventario sintético estático como resultado real.

### PDP

Galería/portada pública, título, características y reglas permitidas, zona aproximada textual, tarifa con moneda/unidad y CTA respaldado por el flujo actual. Identificación del ensayo en demo; no afirmar «verificado» salvo significado público específico aprobado por Go. Si se retira mientras se mira, explica la indisponibilidad y no permite continuar sobre datos antiguos.

### Registro diferido

Conservar un contexto mínimo: espacio, acción permitida, filtros públicos e intervalo opcional. Nada de Bearer, contraseñas, documentos ni claves de pago en la URL o almacenamiento persistente.

Ampliar el validador de destino con rutas internas y acciones enumeradas; rechazar esquemas externos, `//`, codificación ambigua y parámetros inesperados. Mantener las invitaciones `/reservas?contrato=...` entregadas en #292.

Registro no equivale a sesión: preservar el contexto entre registro, verificación de correo y login. Si hay navegación fuera de la pestaña, definir una solución acotada, por ejemplo contexto no sensible con caducidad en `sessionStorage`, validado otra vez al leerlo. No persistir Bearer ni inventar refresh/session cookies. Si se pierde el contexto, ofrecer volver al catálogo sin una operación automática.

Tras login: consultar Go, comprobar rol/capacidad, modo restringido, estado del espacio, tarifa y disponibilidad. Retornar a la ficha/paso apropiado. No ejecutar POST de reserva/pago automáticamente: pedir confirmación del usuario. Usuario no elegible recibe explicación y acceso al flujo de verificación existente.

## 8. SSR, indexación y entorno

SSR dinámico para `/explorar` y `/espacios/:id`; páginas informativas conservan prerender y paneles privados conservan CSR/noindex. No prerenderizar todas las combinaciones de filtros ni el inventario cambiante durante build.

Configurar URL interna del API para servidor Angular y origen público para canonical/enlaces; el navegador no recibe nombres Docker internos ni tokens técnicos. SSR llama los endpoints públicos sin identidad. Estado por petición, sin singleton mutable que mezcle usuarios/búsquedas; TransferState solo contiene DTO público. Comparar HTML SSR y datos hidratados y comprobar que no hay salto a login.

Ficha no visible devuelve 404 real con página adecuada; error de Go devuelve estado de servicio temporal apropiado, no 404 ni 200 vacío con apariencia de catálogo sano. Sitemap incluye solo publicaciones visibles mediante consulta paginada; canonical determinista. Búsquedas facetadas/cursor no se indexan por defecto; landing y fichas válidas pueden indexarse. Login/paneles privados noindex. robots.txt no constituye autorización.

Preparar metadatos título/descripción/sociales con datos públicos. La demo local demuestra HTML, códigos HTTP y sitemap; no demuestra indexación efectiva por Google ni autoriza despliegue público.

Compose conserva separación Angular/Go y redes actuales. El frontend sigue sin montar secretos o datos privados de backend. Los E2E mantienen montajes mínimos aislados. El reloj manual de negocio conserva su overlay de ensayo; no modifica la hora de máquina ni expiración técnica de sesiones. No activar servicios externos para cerrar V2.

Para la carga sintética de tratos autorizada por el usuario, se permite un reloj manual montado solo lectura en un proceso API dedicado y claramente marcado como aislado, aunque ese proceso comparta temporalmente la base persistente. Esta excepción requiere aprobación/fusión del PR que incorpora los resguardos, evidencia de que el reloj no activa barridos/reconciliadores globales de contratos, reservas, garantías o pagos, checkpoint idempotente y comparación de estado previo/posterior. La carga se limita a cuentas, espacios y reservas sintéticos; el proceso y el overlay se retiran al terminar. El reloj del sistema y las expiraciones técnicas de sesión no cambian.

## 9. Orden de trabajo y cortes revisables

Cada corte sigue **datos → Go/OpenAPI → Angular → evidencia**. Las tres áreas pueden preparar contratos y componentes en paralelo, pero ningún frontend integrado se acepta contra una API simulada en sustitución de Go. Una capa sin cambios debe justificar «sin migración» o «sin cambio backend»; no crear migraciones vacías.

Usar preferentemente un PR coherente por corte. Si se separan PR de DB, Go y Angular, enlazar dependencias y fusionar en ese orden con esquema/API compatibles; cerrar el corte solo al probar el recorrido completo. No producir tres versiones inconexas de catálogo. Mantener la aplicación arrancable entre merges.

| Corte | Base de datos primero | Go/OpenAPI después | Angular al integrar | Criterio de salida |
| --- | --- | --- | --- | --- |
| **MPV2-00 · Inventario y contrato** | Confirmar V54/schema, categorías, perfiles y datos publicables; diseñar delta. | Auditar permisos/rutas, política única y DTO; fijar semántica de búsqueda/precio. | Revisar guards, sesión, retorno, tokens y SSR; bosquejar PLP/PDP. | Decisiones y matriz de datos aprobadas; alcance sin ambigüedades. |
| **MPV2-01 · Primer catálogo público** | Sin migración si campos actuales bastan; jamás backfill geográfico. | Listado/detalle GET públicos, categorías activas, filtro categoría y modalidad/tarifa base, paginación SQL tamaño 20 (máx. 25) cursor estable, DTO mínimo y 404 genérico. | `/explorar` y ficha sin sesión; URL category/rate_unit/base-price/sort/page_size/cursor; precio con unidad, sin fecha obligatoria. | Anónimo recorre publicación activa, borrador/oculta no existen públicamente; puede comparar tarifas de una unidad; flujos privados protegidos. |
| **MPV2-02 · Búsqueda y filtros Go** | Ubicación pública nullable, catálogo region/comuna, tsvector español-unaccent e índices; sin derivar dirección. | q por términos/FTS, ubicación, filtros de atributos aprobados, filtros/sort de tarifa base, total estimado opcional con intervalo, sort relevancia y cursor común. | Codec canónico y controles de texto/zona/precio base/total. | Búsqueda PostgreSQL estable, filtros coherentes, limitados y paginados; no efectos privados. |
| **MPV2-03 · Editor y metadatos públicos** | Restricciones/versiones de metadatos; sin tocar snapshots históricos. | Lectura/escritura propia de zona y contenido publicable, autorización y publicación. | Editor anfitrión muestra qué se difundirá y permite completar datos. | Crear/editar/publicar un espacio sin SQL manual y verlo públicamente; tercero rechazado. |
| **MPV2-04 · Fotos y ficha completa** | Reutilizar V32/V49; migrar solo metadatos/relaciones nuevos imprescindibles. | Lectura pública controlada de imagen activa por publicación; retirada, portada y minimización. | Galería, tarifa/unidad, características, reglas y CTA coherentes. | Foto de activa visible; privada, ajena/retirada y evidencia KYC inaccesibles; ficha retirable. |
| **MPV2-05 · PLP, portada y categorías** | Sin migración salvo necesidad probada por consultas. | Ajustes de filtros/resumen si la UI los requiere, con contrato primero. | Panel escritorio/móvil, chips, orden, búsqueda en landing, categorías filtrantes, Atrás/adelante. | Exploración completa sin cuenta a 390×844 y escritorio, URL persistente y teclado accesible. |
| **MPV2-06 · Acción y autenticación contextual** | Sin migración para retorno; revisar compatibilidad cotización/reserva. | Revalidar espacio, permisos/intervalo/tarifa en operación privada; conservar idempotencia. | Acceso/registro/verificación/login y vuelta al espacio; no autoejecutar POST. | Anónimo elige espacio, entra, confirma y usa reserva existente; modo restringido y retirada tratados. |
| **MPV2-07 · SSR y SEO público** | Consulta paginada del inventario visible para sitemap; sin migración si índices bastan. | Lecturas anónimas SSR y estados de no visible/temporal; origen público configurable. | SSR dinámico, HTTP404/503, canonical, social, sitemap/robots y noindex privado. | HTML con ficha sin JS, sin Bearer ni datos privados; retirada también en HTML/sitemap. |
| **MPV2-08 · Regresión y operación local** | Actualización V54→V2 y base vacía; dataset y planes SQL medidos. | Pruebas seguridad/negocio, abuso limitado, CORS y caída/restauración; revisar consultas costosas. | E2E de visitante y retorno privado, carreras de rutas/criterios/sesión; accesibilidad y responsive. | Stack aislado reproducible, recorridos reales completos y límites documentados. |
| **MPV2-09 · Demo y decisión humana** | Registrar versiones de migración y estrategia de conservación/retirada. | Guía API y diagnóstico; evidencias de compatibilidad privadas. | Demo de recorrido público/privado y guía reproducible. | Presentar demo y registrar decisión humana; no asumir que la preparación o fusión técnica aceptan el producto. Estado: cambios solicitados. |
| **MPV2-10 · Orden y jerarquía** | Sin migración prevista; fixture suficiente para ocho categorías, empates y varias páginas. | Incorporar `title_asc`/`title_desc` a validación, OpenAPI, SQL, cursor y orden estable; conservar modalidades existentes y explicar prerrequisitos de precio/relevancia/total. | Categoría inmediatamente después de Buscar; ordenar entendible con borradores y acciones explícitas para aplicar/descartar; URL/cursor/Atrás-Adelante/retorno coherentes. | Router Angular real, Go/PostgreSQL, sort realmente cambia resultados; sin mezcla CLP hora/día/mes ni pérdida silenciosa de filtros. Captura servida posterior al recorrido. |
| **MPV2-11 · Perfiles públicos tipados** | Auditar ocho perfiles y ampliar allowlist/metadatos de forma aditiva si hace falta. | Filtros booleanos, rangos y enums validados contra perfil/versión, SQL parametrizado y OpenAPI. | Controles dinámicos desde perfil Go tipado, sin clases duplicadas ni JSON privado. | Fixture de ocho categorías con coincidencias y no coincidencias; permisos y DTO minimizados. |
| **MPV2-12 · Cuadrícula y densidad** | DTO/consulta de portada solo si la tarjeta requiere datos adicionales públicos. | No cambiar filtros/cursor por preferencias de presentación. | Lista/cuadrícula, 2/3/4 columnas desktop, 1/2 móvil, 12/24 resultados compatibles con 20 histórico y máximo 25; tarjeta pública y responsive. | URL/historial, retorno contextual, SSR/hidratación, accesibilidad, varias páginas/cantidades reales y sin N+1. |
| **MPV2-13 · Regresión y nueva aceptación** | Base vacía/actualización si los cortes agregaron migraciones. | Regresión API/SQL/privacidad y costos; integrar contratos de los cortes. | Recorrido final con demo nueva, evidencia desktop/móvil y límites. | Pruebas integrales y decisión humana explícita sobre commit fusionado; aceptación de producto continúa pendiente hasta recibirla. |

MPV2-00 es documentación e inventario; no construye filtros ni API. El alcance mínimo de MPV2-01 está fijado arriba y deja búsqueda textual, localización, atributos, total estimado, fotos y SSR a los cortes siguientes. MPV2-03 puede prepararse después del contrato de MPV2-02, pero no debe retrasarse hasta el cierre porque los nuevos datos necesitan un recorrido de mantenimiento.

## 10. PR, pruebas y evidencia

Autor de PR: **HernanMEC**, verificado mediante identidad efectiva de push y creación. Revisor: **HernanEspinozaDev**. Registrar head y base, alcance, operaciones OpenAPI, migraciones, pruebas realmente ejecutadas y límites. Una solicitud de revisión no equivale a aprobación. Cada corte espera revisión/fusión antes de continuar con su sucesor.

Aplicar `AGENTS.md` y `scripts/git-role.sh`: `main` es la única rama permanente; ramas temporales `codex/<área>/<tipo>/<corte>-<descripción>`, eliminadas tras fusionar. Para cortes con cambios DB/Go/Angular usar área `marketplace`. La limpieza histórica conserva la base persistente y las pruebas de código; los ensayos V2 se crean desde cero con datos locales nuevos.

La [matriz](matriz_aceptacion.md) define pruebas futuras, no resultados existentes. En cada corte correr pruebas enfocadas; consolidar Angular, `api:check`, build/SSR, lint/arquitectura y regresión Go/DB cuando cambie la capa correspondiente. No usar el número de pruebas como porcentaje de avance.

Ensayos con proyecto Compose, puertos y volumen propios. Antes de `up`, inspeccionar configuración resuelta y montajes; nunca resolver un puerto ocupado arrancando el proyecto persistente por defecto. No pruebas destructivas ni `down --volumes` sobre proyectos ajenos. Cuentas sintéticas recreables, con credenciales locales excluidas y permisos 0600; suficiente variedad de anfitriones/arrendatarios/administración/tercero para cada escenario, sin tratar cantidades de cuentas como aceptación funcional.

Capturas reales sin sesión en portada/catálogo/ficha, móvil 390×844 CSS px y escritorio 1365×900 (o dimensiones registradas equivalentes). Registrar viewport CSS y tamaño de archivo por separado, commit, hashes, configuración y estado de Go. Evidencia de teclado/foco, contraste y estados de carga/error/vacío. Pruebas simuladas se identifican como tales y se complementan con recorridos conectados a Go.

## 11. Definición de terminado

V2 se acepta cuando una ventana privada sin token permite buscar, filtrar y consultar publicaciones/fotos; una categoría abre resultados filtrados; solo al reservar se pide identidad; registro/login conserva el contexto y Go confirma la operación sin relajar permisos. Publicaciones no visibles y datos privados permanecen inaccesibles por API, imagen, SSR, facetas y sitemap.

También exige actualización DB compatible, consultas limitadas/medidas, HTML público indexable, paneles privados preservados, evidencia móvil/escritorio y demo aislada reproducible. El usuario da aceptación explícita a MPV2-09 y se registra el commit exacto.

No equivale a puesta en producción, cumplimiento legal certificado, indexación efectiva en Google ni habilitación de proveedores. Las funciones futuras se dejan en backlog, sin CTA que parezca operativo.

## 12. Estado de seguimiento

MPV2-00 está revisado y fusionado en PR #294 (merge 916176c85186fa95fa84d99911279309a2653a19). MPV2-01 se fusionó técnicamente en #295 (merge 1222cf734dd7b6bd486bdc71e277f5b345830be2); MPV2-02 en #296 (merge a8af7b04691c61363c7348f376b5b16857c17995); MPV2-03 en #297 (merge 0fe7785352ba91972d4ec403d24b4cd085d5477e); MPV2-04 en #298 (merge c3e5cd297a32863446df33d271931aacb6fc2a8b). MPV2-05 fue revisado y fusionado técnicamente en PR #299, merge `fe8d720e1ce0e2e6f296fcf01953f334a5adec52`; no implica aceptación final de producto. MPV2-06 fue aprobado y fusionado técnicamente en [PR #300](https://github.com/HernanEspinozaDev/espaciGo/pull/300), merge `a285d4531279a8f93738712247f4f5c80cdeb9f6`, HEAD revisado `f2b336c`; CI `38479450050` terminó exitosamente. MPV2-07 fue aprobado y fusionado técnicamente en [PR #301](https://github.com/HernanEspinozaDev/espaciGo/pull/301), merge `f259d50d92d44b2e68d180c9b2aef07b40e942a3`; no tiene migración ni cambios Go/OpenAPI. La evidencia SSR/sitemap está en [evidencia MPV2-07](evidencia/mpv2_07/README.md). MPV2-08 fue aprobado y fusionado técnicamente en [PR #302](https://github.com/HernanEspinozaDev/espaciGo/pull/302), merge `6075b7800b45f15e779d11c3ab5853dadd26238e`; revisión HernanEspinozaDev; CI `38486628906` exitoso. Conserva V1–V56, sin cambios OpenAPI. Revisión final repitió el fixture de 1.000 ofertas/950 visibles y EXPLAIN (14,344 ms página de inventario; 4,692 ms cursor filtrado), sin extrapolación a producción. La falta de cuota por IP/429 sigue registrada antes de exposición externa.

MPV2-09 fusionó documentación y evidencia para revisión sobre el código `6075b7800b45f15e779d11c3ab5853dadd26238e`; no añade migraciones, cambios Go/OpenAPI ni alcance funcional. El usuario revisó y pidió cambios: ordenamiento usable, ubicación de categoría después de Buscar, filtros/resultados por categoría y jerarquía explícita entre borradores y criterios aplicados. Por tanto MPV2-09 se registra **NO aceptado / cambios solicitados**, aunque sus fusiones técnicas e historial siguen válidos. La pila local aislada, los datos y las credenciales privadas continúan conservados en su worktree dedicado, sin actualizarlo durante los nuevos cortes. La evidencia FE-14 temporal pasó 7/7; la pila de revisión se mantiene a petición del usuario. Guía y criterio de decisión: [aceptacion_mpv2_09.md](aceptacion_mpv2_09.md); capturas/hashes: [evidencia MPV2-09](evidencia/mpv2_09/README.md).

La ampliación autorizada y las decisiones UX/contrato se versionan en [plan de perfeccionamiento público](plan_perfeccionamiento_publico.md). MPV2-10–13 son cortes de la misma MARKETPLACE-LOCAL-2, no una rama permanente nueva. MPV2-10 se fusionó en PR #304 (`5029a1ab73f4e9c365d5352299849b7dbe80991f`); MPV2-11 se aprobó y fusionó en PR #305 (`2856a304fbbfdbf1b5dcb0544b7c6d504e574898`); MPV2-12 se aprobó y fusionó en PR #306 (`c797f0eb734e2c7eda37eb70a50d4d509e82677d`). Son fusiones técnicas, sin aceptación final de producto. La evidencia queda en `evidencia/mpv2_11/README.md` y `evidencia/mpv2_12/README.md`. MPV2-13 consolida regresión, registra límites y prepara una demo nueva; no declara el marketplace terminado ni aceptado.


## MPV2-13 — estado de regresión y demo (2026-10-14)

La regresión consolidada se ejecutó sobre la aplicación fusionada `c797f0eb734e2c7eda37eb70a50d4d509e82677d`: V1–V57 sin cambios; backend con 91 integraciones requeridas y cero skips; Angular 267 pruebas, lint/arquitectura, contrato OpenAPI y build SSR; FE-14 real 16/16. La demo aislada nueva pasó 5/5 recorridos públicos y se retiene para revisión en el proyecto documentado. Evidencia, entorno y límites: [MPV2-13](evidencia/mpv2_13/README.md); pasos de revisión: [guía de aceptación técnica](aceptacion_mpv2_13.md). Esto acredita resultados técnicos; aceptación de producto sigue pendiente.
