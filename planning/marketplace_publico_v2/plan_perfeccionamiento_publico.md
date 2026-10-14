# Perfeccionamiento del marketplace público antes de la aceptación

## Estado y alcance autorizado

El usuario NO acepta todavía MPV2-09. Las fusiones técnicas MPV2-00–09 siguen válidas como historial, pero no constituyen aceptación de producto. Base actual de main: ab9039beffa43eb70007e2db64546205cde4c21e; aplicación de la demo: 6075b7800b45f15e779d11c3ab5853dadd26238e.

Se autorizan mejoras de ordenamiento, cuadrícula/cantidad visible, filtros propios de cada categoría y categoría inmediatamente después de Buscar. Las dos capturas del usuario son referencias de distribución, no requisitos de copiar marcas, colores, taxonomía residencial, mapas, UF ni capacidades ajenas.

Este plan es una ampliación de MARKETPLACE-LOCAL-2: cortes MPV2-10–13, uno por rama/PR y revisión/fusión entre cortes. Desarrollo debe versionarlo en planning/marketplace_publico_v2/plan_perfeccionamiento_publico.md y actualizar plan, matriz y rango de identificadores de AGENTS.md; no crear una rama permanente nueva. Este archivo local es el encargo preparado, no una implementación.

## Diagnóstico verificado

- Go representa categorías mediante perfiles de atributos versionados (V6, spaces.Profile/AttributeDefinition); no requiere una clase duplicada por categoría en Angular.
- Existen integer, number, boolean, enum y enum_list, con etiquetas, orden, unidades, límites y opciones.
- V55 limita atributo_filtro_publico a boolean y selecciona dos atributos por categoría. El endpoint público y Angular no expresan todavía el perfil completo de filtros tipados.
- La API real responde 422 a sort=unit_price_asc sin rate_unit; responde 200 con rate_unit=hora. Esto evita comparar tarifas por hora/día/mes como importes equivalentes. No se ha demostrado aún un fallo del algoritmo de ordenamiento; sí existe una barrera de UX que debe corregirse y una validación pendiente con suficientes resultados.
- El filtro parcela_eventos de la demo actual devuelve 200 con items=[]: no hay ejemplos para evaluar esa categoría.
- El selector de orden opera sobre filtros aplicados, mientras el formulario tiene borradores: hay que hacer visible y probar esa interacción.
- Las tarjetas actuales son filas con ilustración, sin selector de vista ni cantidad. La API permite page_size hasta 25; no subir el límite para resolver densidad visual.

## Decisiones de UX y contrato

1. Sidebar: Buscar → Categoría → Ubicación → Tarifa/precio → Características de la categoría → Opciones avanzadas. Categoría cambia inmediatamente los controles específicos y elimina criterios incompatibles; los resultados solo cambian conforme la política explícita de Aplicar/Buscar.
2. Controles comunes permanecen separados de los específicos. Sin categoría no se muestran atributos de una categoría arbitraria.
3. Orden usable sin prerrequisitos: conservar el orden predeterminado y agregar Nombre A–Z/Z–A con SQL y cursor estables. Precio asc/desc requiere una unidad seleccionada por la persona; relevancia requiere texto; total estimado requiere intervalo. Mostrar por qué una opción no aplica y una acción para completar el requisito; no dejar un selector aparentemente averiado ni elegir silenciosamente una unidad.
4. El cambio de orden usa estado confirmado, limpia cursor y agrega historial. Evitar perder silenciosamente ediciones pendientes: ofrecer aplicar o descartar antes del cambio que las sustituya.
5. Vista Lista/Cuadrícula. Cuadrícula inicial recomendada: tres columnas en escritorio y dos en móvil si caben. Selector de columnas/densidad: 2/3/4 escritorio, 1/2 móvil; límites responsive claros y sin desbordamiento.
6. Cantidad de resultados por solicitud: 12 o 24, manteniendo compatibilidad con enlaces anteriores de 20 y el máximo Go de 25. Cambiar cantidad reinicia cursor y valida el contrato; cambiar vista/columnas no cambia filtros ni solicita datos innecesarios.
7. URL conserva filtros, orden y page_size. Preferencias de presentación se validan aparte, no se envían a Go ni forman parte de su fingerprint de cursor. Deben conservarse en Atrás/Adelante y retorno contextual. Canonical/robots y SSR deben tratar esas combinaciones sin crear URLs indexables duplicadas.
8. Tarjetas: foto pública controlada o placeholder honesto, título, categoría, comuna/región, CLP/unidad y enlace a ficha; imagen retirada/error sin icono roto. Nunca cargar detalles privados ni hacer una petición de detalle por tarjeta.
9. No mostrar conteos globales/facetas inventados. Distinguir resultados cargados de total del inventario. Total exacto y fecha de publicación son mejoras opcionales posteriores, salvo necesidad demostrada del corte.

## Filtros por categoría: candidatos existentes, no capacidades implementadas

La fuente sigue siendo el perfil Go versionado, con una política pública explícita. No publicar indiscriminadamente JSON privado ni inferir publicabilidad del viejo filter_candidate. Auditarlos contra V6 y la versión vigente antes de ampliar la allowlist.

| Categoría | Ejemplos de atributos existentes para controles propios |
| --- | --- |
| Oficina | Puestos de trabajo; tipo de uso; mobiliario; escritorios; Wi-Fi; climatización |
| Sala multipropósito | Disposiciones admitidas; sillas; mesas; proyector; pantalla; videoconferencia |
| Bodega | Altura y volumen útil; acceso de carga; ancho/alto de acceso; muelle; tipo de almacenamiento |
| Estacionamiento | Largo/ancho; vehículos admitidos; altura máxima; techado; tipo de acceso; carga eléctrica |
| Local flexible | Acceso a nivel de calle; vitrina; frente; probadores; superficie de bodega; climatización |
| Stand | Ancho/fondo; lados abiertos; altura; potencia; montaje; mesas/sillas |
| Quincho | Tipo de parrilla; techado; asientos; parrillas; mesas; lavaplatos; refrigerador |
| Parcela y eventos | Superficies exterior/cubierta; estacionamientos; salón; cocina; piscina; acceso de bus |

### Semántica tipada

- Booleano: cualquiera / sí / no; falso es criterio explícito, no ausencia.
- Integer/number: mínimo/máximo con unidad, precisión y límites del perfil; no usar conversiones monetarias para atributos ni casts SQL sin comprobar tipo.
- Enum: selección de opciones suministradas por Go, no texto arbitrario.
- Enum_list: varias opciones; OR entre opciones del mismo atributo, AND entre atributos distintos. Declarar esa semántica y probarla.
- Atributo ausente no satisface filtros positivos ni falsos; no inferir valores que no fueron declarados.
- Cada criterio usa categoría/versión/código/operador/valor validados. Rechazar atributos desconocidos, opciones ajenas, pares invertidos, duplicados ambiguos y valores fuera de tipo/límite.
- Migración nueva aditiva para tipos/metadatos públicos imprescindibles; no editar V1–V56 ni snapshots históricos. Mantener consultas parametrizadas e índices medidos.
- OpenAPI primero; regenerar SDK; migración del codec/retorno/cursor con tratamiento claro de enlaces anteriores. Preservar filtros booleanos existentes compatibles.

## Cortes y criterios de revisión

| Corte | Trabajo | Capas y prueba de terminado |
| --- | --- | --- |
| MPV2-10 · Ordenamiento y jerarquía | Versionar este plan, registrar no aceptación, diagnosticar selector, corregir interacción con borradores, categorías después de buscar, órdenes nombre A–Z/Z–A y ayudas de precio/relevancia/total | Go/OpenAPI si nuevo sort; Angular/SSR/retorno. Datos suficientes para comparar todas las opciones, empates y varias páginas. Orden realmente cambia resultados y URL; sin mezclar unidades ni perder ediciones; pruebas Router y Go/SQL y recorrido servido |
| MPV2-11 · Perfiles públicos tipados | Auditar ocho perfiles, ampliar allowlist/metadata, filtros SQL y controles dinámicos | DB nueva si necesaria → Go/OpenAPI → Angular. Pruebas booleanos, rangos, enums y multiselección; permisos/minimización; cambios de categoría y respuestas tardías; fixture de ocho categorías con coincidencias y no coincidencias |
| MPV2-12 · Cuadrícula, densidad y cantidad | Lista/cuadrícula, columnas responsive, 12/24 resultados, tarjeta con foto pública | DTO/consulta de portada si necesaria → Angular/tokens. URL, Atrás/Adelante, retorno contextual, SSR/hidratación, sin N+1 ni datos privados; foto retirada; teclado y 390×844/1365×900; varias páginas y cantidades reales |
| MPV2-13 · Regresión y nueva aceptación | Consolidar código fusionado, evidencia y renovar demo/guía | Regresión DB/Go/Angular/SSR; datos en ocho categorías, variedad de modalidades, precios y atributos; no mezclar versiones de capturas/demo. Producto continúa pendiente hasta decisión explícita humana |

No empezar un sucesor antes de fusionar el anterior. Correcciones en el mismo PR. Las ramas siguen codex/marketplace/<feature|fix|chore>/mpv2-XX-descripcion.

## Arquitectura, pruebas y entornos

- Mantener Angular 22/TypeScript estricto, features y hexagonal ligera selectiva, Signals, adaptadores OpenAPI/HttpClient, SCSS/tokens. HTTP solo en infraestructura; Go conserva autorización, reglas, perfiles y visibilidad pública. Usar angular-developer y AGENTS del frontend; no duplicar lógica de negocio en el componente.
- Conservación de migraciones aceptadas y datos privados. El DTO público solo contiene atributos aprobados; ninguna dirección, identidad, contacto, token o evidencia KYC.
- Revisar éxito/error tardíos de perfil, categoría, filtros, sort y páginas; retorno después de login; modo restringido; no auto POST; precios int64 exactos.
- Ensayos separados con Go real y PostgreSQL/Mailpit desechables, suficientes publicaciones para todos los casos. No aceptar solo API simulada. Credenciales excluidas 0600, sin logs/capturas de claves.
- Preservar espacigo_pgdata y la demo MPV2-09 retenida mientras el usuario la revisa. Las modificaciones nuevas se ensayan en otro proyecto/puertos/volumen; no actualizar en silencio la demo antigua.
- Controles SSR/no-store/retirada/canonical/sitemap siguen activos. Exposición solo loopback; cuota pública 429 continúa como limitación antes de desplegar externamente.
- Pruebas realmente ejecutadas, HEAD/entorno, métricas y límites. Lint/arquitectura, api:check sin ng serve concurrente, build, Angular, Go e integración pertinente. Nueva migración: base vacía y actualización; no rescribir historial.
- Desarrollo, publicación y respuestas: HernanMEC; revisión/fusión: HernanEspinozaDev; identidad efectiva comprobada antes y después. Comunicación obligatoria de entrega/bloqueo a este chat.

## Sugerencias de mejora incluidas

Mostrar un aviso breve de cambios sin aplicar; filtros específicos agrupados bajo el nombre de la categoría; controles avanzados plegables; imagen con proporción estable para evitar saltos; restauración del foco al cerrar panel; estado vacío con Limpiar filtros y Cambiar categoría; indicadores de unidad siempre junto al precio. No añadir avisos, favoritos, mapas o conteos globales sin contrato y alcance propios.

## Aceptación

MPV2-09 permanece NO aceptado / cambios solicitados. Tras MPV2-13, presentar otra demo con el commit de aplicación fusionado y pedir decisión humana. Ningún PR técnico ni instrucción de continuar equivale a aceptación final.
