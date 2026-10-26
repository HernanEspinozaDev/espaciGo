# Plan de marketing digital — EspaciGo

## 0. Control del documento

| Campo                  | Valor                                                   |
| ---------------------- | ------------------------------------------------------- |
| Documento              | Plan de marketing digital para marketplace público      |
| Producto               | EspaciGo                                                |
| Versión                | `0.1.1`                                                 |
| Estado                 | Propuesta editorial; sin campañas ni medición activadas |
| Fecha base             | 2026-10-24                                              |
| Mercado inicial        | Chile                                                   |
| Idioma                 | Español de Chile                                        |
| Responsable de negocio | Por asignar                                             |
| Próxima revisión       | Antes de activar campañas o publicar nuevas landings    |

### Historial

| Versión | Fecha      | Cambio                                                                                                           |
| ------- | ---------- | ---------------------------------------------------------------------------------------------------------------- |
| `0.1.0` | 2026-10-24 | Primera versión: audiencias, posicionamiento, mensajes, copy, SEO, campañas, funnel, KPIs, calendario y límites. |
| `0.1.1` | 2026-10-25 | Alineación editorial con Fase 4 local y límites verificados de inventario, API/BD y flujo de publicación.        |

### Reglas de versionado

- **PATCH**: ajustes de redacción, UTMs, metadatos o criterios sin cambiar la estrategia.
- **MINOR**: nueva audiencia, landing, campaña, evento o experimento.
- **MAJOR**: cambio de posicionamiento, modelo de adquisición, mercado o definición de conversión.
- Cada cambio debe indicar responsable, fecha, supuesto afectado y evidencia usada.

---

## 1. Propósito y resultado esperado

Este plan separa la estrategia de marketing del plan técnico del frontend. Su función es orientar:

1. La portada y las páginas públicas.
2. La adquisición de **demanda**: personas que quieren arrendar.
3. La adquisición de **oferta**: personas u organizaciones que quieren publicar y arrendar un espacio propio.
4. El posicionamiento conjunto de:
   - **inmuebles residenciales**: casas, departamentos, habitaciones o piezas, estudios o lofts, cabañas, casas anexas, dúplex y casas de veraneo cuando correspondan a la categoría;
   - **espacios flexibles**: oficinas, salas multipropósito, bodegas, estacionamientos, locales flexibles, stands, quinchos y parcelas o espacios para eventos.
5. La preparación de SEO, contenidos, campañas, medición y experimentos sin presentar capacidades o inventario que no existan.

### Resultado de negocio buscado

Construir liquidez local de manera controlada: primero oferta publicable y útil; luego demanda en las categorías y zonas donde el catálogo real pueda responder.

### Estado de producto que condiciona el plan

- El backend incluye la categoría `inmuebles` y atributos residenciales.
- Una publicación puede declarar una tarifa base por **hora, día o mes**; no se deben presentar simultáneamente tres tarifas si la publicación no las tiene.
- En la verificación de Fase 4, la categoría `inmuebles` respondió sin publicaciones públicas; no se presenta oferta residencial inventada.
- El frontend Next.js en `frontend-next-hermes` consulta catálogo, categorías, ubicaciones, filtros, fichas y la operación pública de documentos legales por medio de Go/gateway; no decide elegibilidad ni lee PostgreSQL. La respuesta legal compatible verificada entrega metadatos sintéticos sin texto del documento, por lo que la UI no inventa contenido legal.
- El API Go de HEAD no puede listar contra la base nativa actual porque consulta `public.space_media`, relación que no existe allí. La integración de Fase 4 se probó separadamente con el binario Go compatible `5995a5fcdbd428220cd8927499d1d33f84466980`, contra la misma base y sin migrarla; esto no equivale a validar el listado con el API de HEAD.
- El flujo de publicación no está habilitado; la página de anfitriones es informativa y no presenta formularios ni capta datos.
- La Fase 4 se mantiene exclusivamente en la rama de desarrollo `frontend-next-hermes`; no hay PR, merge ni despliegue. No se activaron campañas, analytics, píxeles, cookies, captación ni integraciones externas.
- Por lo anterior, **no se debe comprar tráfico de demanda residencial ni crear páginas SEO que aparenten oferta residencial disponible hasta que existan publicaciones públicas reales y el API de HEAD sea compatible con el esquema nativo vigente**.

Fuentes internas de producto:

- `planning/inmuebles/plan_categoria_inmuebles.md`
- `db/migrations/V000067__inmuebles_residential_rentals.sql`
- `planning/frontend_next/plan_frontend_next_hermes.md`
- `planning/frontend_next/evidence/fase-4-marketplace-publico.md`

---

## 2. Principios de marketing

1. **Dos lados, dos intenciones.** “Quiero arrendar” y “Quiero publicar” deben tener rutas, CTAs y medición separadas.
2. **Oferta antes que demanda masiva.** No enviar personas a categorías o comunas vacías.
3. **Categoría amplia, mensaje concreto.** La marca cubre inmuebles y espacios flexibles, pero cada anuncio o landing responde a una necesidad específica.
4. **Inventario real o copy condicional.** Categorías, ubicaciones, tarifas, atributos y disponibilidad se comunican desde datos reales.
5. **Sin capacidades aspiracionales.** No afirmar pagos seguros, verificación, garantías, contratos, seguros, moderación, soporte 24/7 ni reserva inmediata si el flujo productivo no lo demuestra.
6. **Búsqueda pública, acción contextual.** Explorar no debe exigir cuenta; una acción privada puede pedir acceso conservando el contexto.
7. **Español chileno claro.** Usar “arriendo”, “arrendar”, “pieza”, “comuna”, “bodega”, “quincho” y CLP según el caso. Evitar regionalismos de otros países como “alquiler” en el copy principal.
8. **No confundir alcance con disponibilidad.** Que EspaciGo soporte una categoría no significa que haya publicaciones activas en ella.
9. **Medición antes de optimización.** No declarar una campaña ganadora sin conversiones definidas, deduplicadas y atribuibles.
10. **Expansión SEO controlada.** No generar miles de combinaciones categoría × comuna × atributo sin inventario y contenido útil.

---

## 3. Modelo de doble demanda y oferta

### 3.1 Lado demanda: personas que buscan arrendar

| Segmento                   | Necesidad                        | Ejemplos de búsqueda                  | Mensaje principal                                                        | CTA inicial                                      |
| -------------------------- | -------------------------------- | ------------------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------ |
| Residencial estable        | Vivir por uno o más meses        | casa, departamento, pieza, estudio    | Encuentra publicaciones que se ajusten a tu tipo de vivienda y modalidad | `Buscar arriendos`                               |
| Residencial temporal       | Estadía por días o temporadas    | cabaña, casa de veraneo, casa por día | Explora alojamientos publicados para la duración informada               | `Explorar inmuebles`                             |
| Trabajo                    | Espacio para trabajar o reunirse | oficina, sala, cowork, local temporal | Encuentra un espacio según uso, capacidad y modalidad                    | `Buscar espacios de trabajo`                     |
| Almacenamiento y movilidad | Resolver una necesidad práctica  | bodega, estacionamiento               | Compara publicaciones por ubicación, características y tarifa            | `Explorar bodegas` / `Explorar estacionamientos` |
| Encuentros y eventos       | Reunión social, taller o evento  | quincho, parcela, sala, stand         | Revisa capacidad, reglas y equipamiento declarado                        | `Buscar espacios para eventos`                   |

### 3.2 Lado oferta: personas que quieren arrendar lo que tienen

| Segmento                  | Activo                   | Motivación                          | Objeción probable                                       | Mensaje principal                                     | CTA                                            |
| ------------------------- | ------------------------ | ----------------------------------- | ------------------------------------------------------- | ----------------------------------------------------- | ---------------------------------------------- |
| Propietario residencial   | Casa o departamento      | Dar uso a un inmueble disponible    | No saber cómo presentarlo                               | Publica sus características, modalidad y tarifa       | `Publicar mi inmueble`                         |
| Arrendador de pieza       | Habitación o pieza       | Ofrecer parte de una vivienda       | Necesidad de explicar privacidad y espacios compartidos | Indica qué se arrienda y qué espacios se comparten    | `Publicar una habitación`                      |
| Propietario estacional    | Cabaña o casa de veraneo | Arrendar por días o temporadas      | Demanda irregular                                       | Presenta tu espacio, reglas y modalidad disponible    | `Publicar casa de veraneo`                     |
| Operador flexible         | Oficina, sala o local    | Mejorar uso de capacidad disponible | Complejidad de horarios y atributos                     | Publica la modalidad y las características relevantes | `Publicar mi espacio`                          |
| Dueño de activo funcional | Bodega o estacionamiento | Rentabilizar capacidad ociosa       | Compatibilidad y acceso                                 | Describe medidas, acceso y tarifa con precisión       | `Publicar bodega` / `Publicar estacionamiento` |
| Anfitrión de eventos      | Quincho, parcela o stand | Ofrecer un recinto                  | Reglas de uso y capacidad                               | Declara capacidad, equipamiento y condiciones         | `Publicar espacio para eventos`                |

### 3.3 Regla de lenguaje para evitar ambigüedad

La frase **“arrienda tu espacio”** puede interpretarse como buscarlo o publicarlo. Se puede usar como titular de oferta, pero debe ir acompañada de un verbo inequívoco:

- Oferta: **“Arrienda tu espacio: publícalo en EspaciGo.”**
- Demanda: **“Encuentra un espacio para arrendar.”**
- Residencial demanda: **“Busca una casa para arrendar.”**
- Residencial oferta: **“Publica tu casa en arriendo.”**

---

## 4. Posicionamiento

### 4.1 Propuesta de posicionamiento

> **EspaciGo es un marketplace chileno para descubrir y publicar arriendos de inmuebles residenciales y espacios flexibles, con modalidades por hora, día o mes según cada publicación.**

Uso: declaración estratégica interna. En la UI se debe acortar y adaptar al estado real del inventario.

### 4.2 Promesa de marca

> **Arrienda el espacio que necesitas. Publica el espacio que tienes.**

### 4.3 Pilares

1. **Variedad de usos:** vivir, trabajar, guardar, estacionar, reunirse o celebrar.
2. **Flexibilidad declarada:** hora, día o mes cuando la publicación ofrezca esa modalidad.
3. **Información útil:** tipo, características, reglas, ubicación pública y tarifa declarada.
4. **Doble acceso:** buscar o publicar desde una misma marca con recorridos separados.

### 4.4 Diferenciación comunicable sin claims de infraestructura

- Reunir categorías residenciales y flexibles bajo una búsqueda común.
- Permitir que cada publicación explique qué se arrienda y bajo qué modalidad.
- Adaptar filtros y ficha al tipo de espacio.
- Mantener separadas la exploración pública y las acciones privadas.

### 4.5 No usar como promesa pública por ahora

- “Arrienda seguro”.
- “Publicaciones verificadas”.
- “Pago protegido” o “pago seguro”.
- “Garantía asegurada”.
- “Contrato legal incluido”.
- “Propietarios verificados”.
- “Reserva inmediata”.
- “Sin riesgo”.
- “Sin comisión”.
- “La mayor oferta de Chile”.
- “Miles de propiedades”.
- “En todo Chile”.
- “Mejor precio garantizado”.

Estas frases solo podrían evaluarse con alcance productivo, política, evidencia y aprobación específica.

---

## 5. Arquitectura de mensajes

### 5.1 Jerarquía

| Nivel           | Pregunta que responde       | Mensaje                                                                                           |
| --------------- | --------------------------- | ------------------------------------------------------------------------------------------------- |
| Marca           | ¿Qué es EspaciGo?           | Marketplace de arriendos residenciales y espacios flexibles                                       |
| Doble intención | ¿Qué puedo hacer?           | Buscar un arriendo o publicar un espacio                                                          |
| Categoría       | ¿Qué puedo buscar/publicar? | Casas, departamentos, habitaciones, oficinas, bodegas, estacionamientos, quinchos, parcelas y más |
| Modalidad       | ¿Por cuánto tiempo?         | Por hora, día o mes, según cada publicación                                                       |
| Evidencia       | ¿Qué voy a revisar?         | Características, reglas, ubicación pública, modalidad y tarifa declarada                          |
| Acción          | ¿Cuál es el siguiente paso? | Explorar / Ver detalle / Comenzar publicación                                                     |

### 5.2 Etiquetas de seguridad para copy

Todo copy nuevo debe clasificarse antes de publicarse:

- `[BASE]`: no depende de inventario ni de una capacidad transaccional.
- `[INVENTARIO]`: solo se muestra si existen resultados reales que respalden la frase.
- `[FLUJO]`: solo se muestra si la acción está habilitada de punta a punta.
- `[ESTADO]`: depende de un estado calculado por Go, por ejemplo disponibilidad.
- `[POLÍTICA]`: requiere aprobación legal, comercial o de privacidad.

Ejemplos:

| Copy                                | Etiqueta       | Condición                                      |
| ----------------------------------- | -------------- | ---------------------------------------------- |
| “Conoce las categorías de EspaciGo” | `[BASE]`       | Siempre                                        |
| “Explora casas en Providencia”      | `[INVENTARIO]` | Hay casas públicas reales en esa comuna        |
| “Publicar mi casa”                  | `[FLUJO]`      | El flujo permite crear y completar publicación |
| “Disponible del 3 al 7 de enero”    | `[ESTADO]`     | Go confirma el intervalo                       |
| “Sin comisión”                      | `[POLÍTICA]`   | Modelo de cobro aprobado y vigente             |

---

## 6. Copy listo para portada

### 6.1 Hero recomendado para lanzamiento con catálogo real

**Eyebrow**

> Casas, espacios y más formas de arrendar

**H1**

> Arrienda un espacio. Publica el tuyo.

**Bajada**

> Explora casas, departamentos y espacios flexibles publicados en EspaciGo, o crea una publicación para ofrecer tu espacio por hora, día o mes.

**CTA demanda**

> Buscar arriendos

**CTA oferta**

> Publicar mi espacio

**Apoyo bajo CTAs**

> La modalidad, tarifa y condiciones dependen de cada publicación.

Condiciones: `[INVENTARIO]` para “Explora…” y `[FLUJO]` para “Publicar mi espacio”.

### 6.2 Hero para el estado local verificado de Fase 4

**Eyebrow**

> Espacios para vivir, trabajar y hacer más

**H1**

> Arrienda el espacio que necesitas. Ofrece el espacio que tienes.

**Bajada**

> Encuentra publicaciones para vivir, trabajar, guardar, reunirte o celebrar. Casas, departamentos y espacios flexibles aparecen cuando el catálogo público los ofrece.

**CTA principal**

> Explorar arriendos

**CTA secundario**

> Conoce cómo ofrecer tu espacio

**Aviso visible**

> La categoría residencial no tiene publicaciones públicas en la verificación actual. Crear y publicar anuncios todavía no está habilitado en esta versión.

Esta variante refleja el frontend local de Fase 4. El catálogo solo muestra resultados elegibles devueltos por Go; el API de HEAD sigue bloqueado para listados por la incompatibilidad de esquema documentada arriba. No es una landing de adquisición productiva ni autoriza campañas.

### 6.3 Hero alternativo orientado a la doble intención

**H1**

> ¿Buscas un espacio o quieres publicar el tuyo?

**Bajada**

> Elige cómo quieres usar EspaciGo y continúa por un recorrido diseñado para esa tarea.

**Selector/CTAs**

- `Quiero arrendar`
- `Quiero publicar`

### 6.4 Franja residencial

**Título**

> Arriendos residenciales para distintas etapas y estadías

**Texto**

> Explora casas, departamentos, habitaciones, estudios, cabañas y otros inmuebles que hayan sido publicados en tu zona.

**CTA**

> Ver inmuebles publicados

No mostrar la sección ni indexar su landing como catálogo si no hay resultados reales.

### 6.5 Franja de espacios flexibles

**Título**

> Espacios que se adaptan a lo que necesitas hacer

**Texto**

> Busca oficinas, salas, bodegas, estacionamientos, quinchos, parcelas, locales y stands según la modalidad informada en cada publicación.

**CTA**

> Explorar espacios flexibles

### 6.6 Franja de captación de oferta

**Título**

> Dale un nuevo uso al espacio que tienes

**Texto**

> Publica sus características, modalidad, tarifa y condiciones para que las personas puedan evaluarlo.

**CTA principal**

> Comenzar una publicación

**CTA secundaria**

> Ver qué información necesitaré

No usar “gana dinero”, “genera ingresos garantizados” ni estimaciones de rentabilidad sin datos y metodología aprobados.

### 6.7 Copy solicitado para casa de veraneo

**Versión principal**

> ¿Tienes una casa de veraneo? Publícala y arriéndala en EspaciGo.

**Bajada**

> Indica qué se arrienda, la modalidad disponible, sus características y las reglas de uso.

**CTA**

> Publicar casa de veraneo

Condición: usar solo cuando el flujo de publicación esté habilitado. Si una zona aún no está abierta, reemplazar el CTA por una captación de interés que exista realmente, por ejemplo `Avísame cuando pueda publicar`; no mostrar formularios ficticios.

### 6.8 “Cómo funciona” sin promesas transaccionales

#### Para buscar

1. **Explora:** revisa categorías y publicaciones públicas.
2. **Compara:** consulta modalidad, tarifa, características y condiciones declaradas.
3. **Continúa:** inicia la acción disponible desde la ficha.

#### Para publicar

1. **Describe tu espacio:** elige la categoría y completa sus datos.
2. **Define cómo lo ofreces:** indica modalidad, tarifa y condiciones disponibles.
3. **Publica cuando cumpla los requisitos:** la visibilidad depende del estado definido por el sistema.

El tercer paso evita prometer aprobación automática.

### 6.9 FAQ sugeridas

- ¿Qué tipos de espacios se pueden publicar?
- ¿Puedo publicar una casa o un departamento?
- ¿Puedo publicar una habitación dentro de mi vivienda?
- ¿Puedo ofrecer una casa de veraneo o una cabaña?
- ¿Qué diferencia hay entre una tarifa por hora, día o mes?
- ¿Qué información aparece públicamente?
- ¿Cómo describo los espacios compartidos de una habitación?
- ¿Qué ocurre si todavía no hay publicaciones en mi comuna?

Las respuestas deben usar el contrato real del producto y no completar brechas con promesas comerciales.

---

## 7. Navegación y arquitectura pública

### 7.1 Navegación principal recomendada

1. `Explorar`
2. `Inmuebles`
3. `Espacios flexibles`
4. `Cómo funciona`
5. `Publicar`
6. `Ingresar` o `Mi cuenta`

En móvil, conservar un CTA visible para `Publicar` sin competir con la búsqueda.

### 7.2 Arquitectura lógica de páginas

Las rutas finales deben alinearse con el plan técnico y evitar duplicar `/products`, `/search` y `/explorar` con contenido equivalente. Se recomienda que `/explorar` sea la ruta pública canónica cuando termine la migración.

```text
/
├── /explorar
│   ├── /explorar/inmuebles
│   │   ├── /explorar/inmuebles/casas
│   │   ├── /explorar/inmuebles/departamentos
│   │   ├── /explorar/inmuebles/habitaciones
│   │   ├── /explorar/inmuebles/cabanas
│   │   └── /explorar/inmuebles/casas-de-veraneo
│   └── /explorar/espacios
│       ├── /explorar/espacios/oficinas
│       ├── /explorar/espacios/salas
│       ├── /explorar/espacios/bodegas
│       ├── /explorar/espacios/estacionamientos
│       ├── /explorar/espacios/quinchos
│       ├── /explorar/espacios/parcelas-eventos
│       ├── /explorar/espacios/locales-flexibles
│       └── /explorar/espacios/stands
├── /espacios/[id-o-slug]
├── /publicar
│   ├── /publicar/inmueble
│   ├── /publicar/casa
│   ├── /publicar/departamento
│   ├── /publicar/habitacion
│   ├── /publicar/casa-de-veraneo
│   └── /publicar/espacio-flexible
├── /como-funciona
├── /guias
│   ├── /guias/publicar-casa-en-arriendo
│   ├── /guias/publicar-habitacion
│   ├── /guias/describir-casa-de-veraneo
│   ├── /guias/publicar-bodega
│   └── /guias/elegir-modalidad-hora-dia-mes
└── páginas legales
```

### 7.3 Regla para páginas por ubicación

Crear una landing como `/explorar/inmuebles/casas/santiago/providencia` solo si:

1. la ubicación pública existe en el modelo;
2. hay publicaciones activas y visibles;
3. la página ofrece un conjunto útil y no una sola tarjeta aislada;
4. contiene introducción, filtros, resultados y enlaces internos propios;
5. puede mantenerse actualizada cuando una publicación deja de ser visible.

No crear páginas de todas las comunas “por si acaso”.

### 7.4 Regla para estados vacíos

- **Categoría sin oferta:** explicar que aún no hay publicaciones visibles y enlazar a categorías cercanas o al flujo de publicación.
- **Filtros demasiado estrictos:** ofrecer `Quitar filtros`, `Cambiar modalidad` y `Ver toda la categoría`.
- **Comuna sin oferta:** no inventar publicaciones cercanas; sugerir ampliar región solo mediante una acción explícita.
- **Ficha retirada:** devolver el estado HTTP y mensaje acordes; no conservar una página indexable con datos obsoletos.

---

## 8. Estrategia SEO

### 8.1 Objetivos SEO

1. Capturar búsquedas transaccionales de demanda solo donde exista oferta real.
2. Captar propietarios y operadores que quieren publicar.
3. Construir autoridad temática mediante guías útiles y específicas.
4. Evitar indexación masiva de facetas, parámetros y páginas vacías.

### 8.2 Clústeres de palabras clave — hipótesis a validar

No se atribuye volumen ni dificultad a estas frases. Deben validarse en Search Console, Keyword Planner y resultados reales antes de invertir.

#### Demanda residencial

- casas en arriendo
- casa en arriendo en `[comuna]`
- departamentos en arriendo
- pieza en arriendo
- habitación en arriendo
- estudio en arriendo
- cabaña en arriendo
- casa de veraneo en arriendo
- casa por día en `[zona]`
- arriendo mensual de casa

#### Demanda flexible

- oficina en arriendo por hora
- sala de reuniones por hora
- bodega en arriendo mensual
- estacionamiento en arriendo
- quincho en arriendo
- parcela para eventos
- local temporal en arriendo
- stand para evento

#### Oferta residencial

- cómo publicar una casa en arriendo
- dónde publicar mi departamento
- arrendar mi casa
- arrendar una pieza de mi casa
- publicar casa de veraneo
- arrendar cabaña por días

#### Oferta flexible

- publicar oficina por hora
- arrendar mi bodega
- publicar estacionamiento
- arrendar quincho
- publicar parcela para eventos
- arrendar local por días

### 8.3 Mapa intención → página

| Intención           | Landing                       | Contenido mínimo                                       | Conversión             |
| ------------------- | ----------------------------- | ------------------------------------------------------ | ---------------------- |
| General de demanda  | `/explorar`                   | Categorías con inventario real, búsqueda y explicación | Búsqueda o ficha       |
| Demanda residencial | `/explorar/inmuebles`         | Tipos, modalidad y resultados reales                   | Ficha residencial      |
| Demanda específica  | `/explorar/inmuebles/casas`   | Filtro preseleccionado y listado real                  | Ficha de casa          |
| Demanda flexible    | `/explorar/espacios/oficinas` | Uso, modalidad, atributos y resultados                 | Ficha de oficina       |
| Oferta general      | `/publicar`                   | Qué se puede publicar y pasos reales                   | Comenzar publicación   |
| Oferta residencial  | `/publicar/inmueble`          | Tipos, campos y límites                                | Comenzar inmueble      |
| Oferta específica   | `/publicar/casa-de-veraneo`   | Información requerida y reglas                         | Comenzar publicación   |
| Informacional       | `/guias/...`                  | Respuesta profunda, enlaces contextuales               | Ir a publicar/explorar |

### 8.4 Plantilla SEO para landing de categoría

1. `<title>` único.
2. H1 que combine categoría e intención, sin afirmar ubicación inexistente.
3. Introducción de 60–120 palabras útil y no repetida.
4. Resultados reales o estado vacío honesto.
5. Filtros propios de la categoría.
6. Explicación de modalidad por hora/día/mes según datos disponibles.
7. Enlaces a categorías relacionadas.
8. Bloque para oferta: “¿Tienes uno? Publícalo” si el flujo existe.
9. FAQ específica, no genérica duplicada.
10. Breadcrumbs y canonical.

### 8.5 Fórmulas de metadatos

#### Página general

- **Title:** `Arriendos de inmuebles y espacios | EspaciGo`
- **Description:** `Explora publicaciones de casas, departamentos y espacios flexibles según categoría, ubicación y modalidad disponible.`

#### Categoría con inventario

- **Title:** `{Categoría} en arriendo{ubicación opcional} | EspaciGo`
- **H1:** `{Categoría} en arriendo{ubicación opcional}`
- **Description:** `Revisa {categoría en minúscula} publicados{ubicación opcional}, sus características, modalidad y tarifa declarada.`

#### Oferta

- **Title:** `Publica tu {tipo} en arriendo | EspaciGo`
- **H1:** `Publica tu {tipo} en arriendo`
- **Description:** `Conoce qué información necesitas para publicar tu {tipo}: características, modalidad, tarifa y condiciones.`

No insertar automáticamente “Chile” o una comuna si la página no contiene resultados de ese alcance.

### 8.6 Canonical, facetas e indexación

Las combinaciones de filtros pueden crear un espacio prácticamente infinito de URLs y consumir recursos de rastreo. Google recomienda controlar la navegación facetada; si esas URLs no necesitan aparecer en buscadores, se puede impedir su rastreo, y si se necesitan, deben obedecer un orden consistente y responder correctamente ante combinaciones sin contenido.[1]

Política propuesta:

- Indexables: portada, páginas editoriales, categorías principales y combinaciones de ubicación seleccionadas manualmente.
- No indexables: orden, paginación con cursor, rango de precio, capacidad, modalidad temporal, atributos y búsquedas internas arbitrarias.
- Canonical: la versión estable de la categoría o landing aprobada; no usar canonical como sustituto de una arquitectura coherente. Google considera redirecciones y `rel="canonical"` señales fuertes, mientras que sitemap es una señal más débil.[2]
- No incluir en sitemap búsquedas internas, filtros, páginas privadas ni fichas retiradas.
- Evitar `noindex` como única respuesta a una explosión de URLs si Google igual debe rastrear cada combinación para descubrir la directiva.[1]

### 8.7 Datos estructurados

Prioridad inicial:

- `Organization` en información de marca.
- `BreadcrumbList` en categorías, guías y fichas.
- `ItemList` en listados públicos, solo con elementos visibles.
- Datos de cada ficha consistentes con el HTML y la API pública.

Google documenta `VacationRental` para información de propiedades vacacionales y exige elegibilidad e integración adicional; por lo tanto, no se debe añadir ese marcado a todas las casas, departamentos o cabañas por defecto.[3]

Reglas:

- No marcar fixtures sintéticos como oferta real.
- No usar `Product` solo porque la UI proviene de una plantilla de tienda.
- No inventar rating, disponibilidad, precio, dirección o anfitrión en JSON-LD.
- Activar un tipo especializado solo después de confirmar que la publicación y el sitio cumplen sus requisitos.

### 8.8 Contenidos prioritarios

#### P0 — Oferta

1. Cómo publicar una casa en arriendo.
2. Qué informar al publicar una habitación.
3. Cómo describir una casa de veraneo.
4. Cómo publicar una bodega o estacionamiento.
5. Cómo elegir entre tarifa por hora, día o mes.

#### P1 — Demanda

1. Cómo comparar una habitación privada y un inmueble completo.
2. Qué revisar antes de arrendar un espacio por hora.
3. Qué datos mirar en una publicación de bodega.
4. Cómo interpretar capacidad, reglas y espacios compartidos.

#### P2 — Local, solo con inventario

1. Guías por ciudad o región con publicaciones reales.
2. Comparaciones de categorías respaldadas por datos del catálogo.
3. Páginas de temporada para casas de veraneo o parcelas, sin afirmar disponibilidad futura.

### 8.9 Brief de contenido obligatorio

Cada pieza debe registrar:

- intención primaria;
- audiencia;
- consulta objetivo como hipótesis;
- URL canónica;
- CTA demanda u oferta;
- fuentes;
- claims revisados;
- responsable y fecha de actualización;
- vínculos internos;
- condición de indexación;
- evento de conversión esperado.

---

## 9. Campañas pagadas y orgánicas

### 9.1 Fases de activación

| Fase                                   | Condición                                                                     | Prioridad                                  | Qué no hacer                              |
| -------------------------------------- | ----------------------------------------------------------------------------- | ------------------------------------------ | ----------------------------------------- |
| G0 · Sin inventario público suficiente | Catálogo sintético o categorías vacías                                        | Captación de oferta, contenidos y medición | Campañas de demanda a páginas vacías      |
| G1 · Oferta inicial por micro mercado  | Al menos 10 publicaciones activas, completas y visibles en una categoría/zona | Prueba acotada de búsqueda de demanda      | Campaña nacional genérica                 |
| G2 · Señales de liquidez               | Intenciones cualificadas repetidas y tasa de ficha suficiente                 | Escalar demanda y retargeting consentido   | Expandir a zonas sin oferta               |
| G3 · Repetibilidad                     | Costos y conversión estables por cohorte                                      | Ampliar categorías y alianzas              | Mezclar resultados de segmentos distintos |

El umbral de 10 es una regla operativa inicial para evitar páginas vacías, no una afirmación de mercado; debe ajustarse con comportamiento real.

### 9.2 Distribución inicial del esfuerzo

#### Durante G0

- 50% captación de oferta.
- 25% contenido SEO/editorial para oferta.
- 15% alianzas locales y prospección.
- 10% analítica, creatividad y experimentos.

#### Durante G1–G2

- 35% captación de oferta.
- 35% búsqueda de demanda en zonas habilitadas.
- 15% contenidos y SEO.
- 10% retargeting con consentimiento y audiencia suficiente.
- 5% experimentos.

Los porcentajes distribuyen esfuerzo o presupuesto aprobado; no fijan un monto.

### 9.3 Google Search — oferta

#### Estructura

```text
Campaña: Publicar residencial
├── Casa / departamento
├── Habitación / pieza
├── Casa de veraneo / cabaña
└── Genérico inmueble

Campaña: Publicar espacio flexible
├── Oficina / sala
├── Bodega
├── Estacionamiento
├── Quincho / parcela
└── Local / stand
```

#### Concordancias iniciales

- Exacta y de frase para consultas de alta intención.
- Amplia solo cuando existan suficientes conversiones y una estrategia de puja controlada.
- Separar residencial y flexible para que la página y el anuncio sean específicos.

#### Negativas sugeridas

- venta, comprar, empleo, trabajo, corredora empleo, gratis, descargar, plantilla contrato, ley, subsidio, remate.
- Revisar semanalmente términos reales; no bloquear “sin comisión” si se quiere conocer esa demanda, pero no prometerlo en el anuncio.

#### RSA — titulares validados a máximo 30 caracteres

| Titular                     | Caracteres |
| --------------------------- | ---------: |
| Publica tu casa en arriendo |         27 |
| Arrienda tu casa de veraneo |         27 |
| Publica tu espacio en Chile |         27 |
| Pon tu inmueble en arriendo |         27 |
| Publica tu bodega u oficina |         27 |
| EspaciGo para anfitriones   |         25 |

#### RSA — descripciones validadas a máximo 90 caracteres

| Descripción                                                                               | Caracteres |
| ----------------------------------------------------------------------------------------- | ---------: |
| Publica las características, modalidad y tarifa de tu espacio cuando habilitemos tu zona. |         89 |
| ¿Tienes una casa de veraneo? Publícala y arriéndala en EspaciGo cuando abramos tu zona.   |         87 |

La redacción debe reflejar la marca, conectar titular y descripción, y apuntar a una landing coherente. Google recomienda mensajes claros y coherentes con la experiencia de destino, además de recursos variados en anuncios responsivos.[4][5]

### 9.4 Google Search — demanda

Activar solo para categorías/zonas que pasen G1.

#### Estructura

```text
Campaña: Demanda residencial — [zona]
├── Casas
├── Departamentos
├── Habitaciones
└── Temporal / veraneo

Campaña: Demanda flexible — [zona]
├── Oficinas / salas
├── Bodegas
├── Estacionamientos
└── Eventos
```

#### Titulares validados a máximo 30 caracteres

| Titular                    | Caracteres |
| -------------------------- | ---------: |
| Busca casas en arriendo    |         23 |
| Espacios para cada plan    |         23 |
| Oficinas por hora en Chile |         26 |
| Arriendo de bodegas y más  |         25 |
| Explora arriendos en Chile |         26 |
| Casas, espacios y parcelas |         26 |

#### Descripciones validadas a máximo 90 caracteres

| Descripción                                                                                | Caracteres |
| ------------------------------------------------------------------------------------------ | ---------: |
| Explora casas, departamentos y espacios flexibles publicados según categoría y ubicación.  |         89 |
| Busca por tipo de espacio, modalidad y ubicación. Revisa cada publicación antes de actuar. |         90 |

No usar un titular nacional si la campaña solo tiene oferta en una comuna; reemplazar “en Chile” por el alcance real.

### 9.5 Meta/Instagram — oferta

Objetivo: generar visitas cualificadas a landings de publicación o, si existe, comienzos de publicación medibles.

#### Audiencias

- Geografía donde se permita publicar.
- Intereses contextuales amplios relacionados con propiedad, turismo local, emprendimiento, oficinas o eventos, sujetos a las reglas vigentes de la plataforma.
- Retargeting de visitantes con consentimiento.
- No construir segmentación a partir de atributos sensibles ni datos que la persona no espera que se usen.

#### Creatividades

1. **Carrusel por activo:** casa, habitación, oficina, bodega, estacionamiento, quincho.
2. **Video corto:** “¿Qué espacio tienes?” → “Descríbelo” → “Elige modalidad” → “Publícalo cuando esté habilitado”.
3. **Checklist visual:** fotos, características, modalidad, tarifa, reglas.
4. **Caso real:** solo cuando exista autorización, datos verificables y resultado atribuible; no inventar testimonios.

#### Copy social sugerido

> ¿Tienes una casa de veraneo? Publícala y arriéndala en EspaciGo. Indica la modalidad, sus características y las reglas de uso para que las personas sepan qué estás ofreciendo.

CTA: `Más información` durante preapertura; `Publicar` cuando el flujo exista.

### 9.6 Orgánico social

Pilares editoriales:

- **Espacio de la semana:** únicamente publicación real y autorizada.
- **Cómo publicar mejor:** fotos, descripción, reglas, privacidad, modalidad.
- **Ideas de uso:** oficina por hora, bodega mensual, quincho por día, sin afirmar demanda garantizada.
- **Educación de categoría:** diferencia entre inmueble completo, habitación privada y compartida.
- **Cobertura local:** nuevas zonas solo cuando estén efectivamente disponibles.
- **Construcción pública del producto:** avances verificables sin anunciar fechas no confirmadas.

Cadencia inicial:

- 2 piezas educativas por semana.
- 1 pieza de categoría por semana.
- 1 actualización de producto o zona cuando exista un cambio real.
- Historias para preguntas frecuentes, sin usar encuestas como evidencia de demanda representativa.

### 9.7 Alianzas

Priorizar alianzas que agreguen oferta concentrada:

- administradores de oficinas y salas;
- operadores de bodegas;
- estacionamientos independientes;
- propietarios de quinchos o recintos;
- administradores de cabañas y casas de veraneo;
- comunidades locales de propietarios;
- instituciones o gremios solo con propuesta y uso de marca aprobados.

Propuesta inicial:

> Ayudamos a estructurar la publicación de tus espacios por categoría y modalidad. La campaña se activa cuando el catálogo público y el flujo correspondiente estén habilitados.

No ofrecer exclusividad, rentabilidad, tarifa preferente ni condiciones comerciales sin aprobación.

---

## 10. Funnel y lifecycle

### 10.1 Funnel de oferta

```text
Impresión o búsqueda
→ visita a landing de publicación
→ clic en comenzar
→ registro/acceso, si corresponde
→ inicio de borrador
→ datos mínimos completos
→ envío/publicación según reglas
→ publicación activa y visible
→ primera intención cualificada
```

#### Definiciones

- **Inicio de publicación:** se crea un borrador real o se entra al primer paso, según la arquitectura final.
- **Publicación completa:** contiene los campos mínimos exigidos para su categoría.
- **Publicación activa:** Go la considera visible públicamente.
- **Intención cualificada:** acción real disponible desde una ficha, por ejemplo iniciar una solicitud; no equivale a pago ni arriendo cerrado.

### 10.2 Funnel de demanda

```text
Impresión o resultado orgánico
→ landing/categoría
→ búsqueda o filtro
→ vista de listado
→ vista de ficha
→ clic en CTA disponible
→ autenticación contextual, si corresponde
→ inicio de solicitud real
```

### 10.3 Activación por correo o notificaciones

Solo implementar mensajes que tengan consentimiento, infraestructura y preferencia de canal:

#### Oferta

- borrador iniciado y no completado;
- publicación que requiere un dato;
- publicación activada;
- primera intención recibida;
- recomendación educativa basada en su categoría, sin prometer resultado.

#### Demanda

- retorno a una acción interrumpida;
- cambios de estado que Go exponga;
- alerta de nueva oferta solo si existe suscripción implementada;
- recordatorio de búsqueda solo con consentimiento.

No habilitar newsletter, alertas o formularios visuales que no recopilan o gestionan datos realmente.

---

## 11. Analítica y eventos

### 11.1 Principios

- Definir eventos desde acciones de negocio, no desde cada clic visual.
- Mantener nombres estables, parámetros documentados y sin PII.
- Distinguir fixtures, entornos y tráfico interno.
- No enviar dirección exacta, teléfono, email, contenido libre, tokens ni IDs sensibles a analítica.
- Deduplicar eventos SSR/cliente y reintentos.

Google Analytics recomienda eventos estándar para acciones comunes de comercio y generación de leads; se pueden adaptar los nombres solo si la semántica coincide con la acción real.[6]

### 11.2 Taxonomía propuesta

| Evento                          | Momento                          | Parámetros permitidos                                                                    |
| ------------------------------- | -------------------------------- | ---------------------------------------------------------------------------------------- |
| `view_home`                     | Portada cargada                  | `content_state`, `inventory_mode`                                                        |
| `select_user_intent`            | Selecciona arrendar/publicar     | `intent`                                                                                 |
| `view_item_list`                | Ve listado real                  | `category_code`, `region_code`, `result_count_bucket`                                    |
| `search`                        | Ejecuta búsqueda                 | `search_term_normalized` solo si se aprueba privacidad; preferir clasificación sin texto |
| `apply_filter`                  | Aplica filtros                   | `category_code`, `filter_codes`, `rate_unit`                                             |
| `view_item`                     | Abre ficha                       | `public_space_id`, `category_code`, `rate_unit`                                          |
| `select_item`                   | Selecciona tarjeta               | `public_space_id`, `list_context`                                                        |
| `start_publish`                 | Comienza publicación             | `category_code`, `source`                                                                |
| `save_publish_step`             | Guarda paso                      | `category_code`, `step_code`                                                             |
| `complete_publish_requirements` | Completa mínimos                 | `category_code`                                                                          |
| `publish_space`                 | Go confirma publicación visible  | `category_code`, `rate_unit`                                                             |
| `begin_rental_intent`           | Inicia acción privada disponible | `public_space_id`, `action_type`                                                         |
| `login_context_return`          | Retorna desde autenticación      | `action_type`, `success`                                                                 |
| `generate_lead`                 | Solo si existe lead real         | `lead_type`, `category_code`                                                             |

### 11.3 UTMs

Formato:

```text
utm_source=google|meta|instagram|partner|newsletter
utm_medium=cpc|paid_social|organic_social|referral|email
utm_campaign=cl_[supply|demand]_[residential|flex]_[category]_[zone]_[yyyymm]
utm_content=[format]_[message]_[variant]
utm_term=[keyword-if-applicable]
```

Ejemplo:

```text
utm_campaign=cl_supply_residential_casa-veraneo_valparaiso_202611
utm_content=video_casa-veraneo_v1
```

No incluir email, teléfono, nombre, dirección ni IDs privados en UTMs.

---

## 12. KPIs y tablero

### 12.1 North Star por etapa

- **G0–G1:** publicaciones activas, completas y visibles por categoría/zona.
- **G2:** porcentaje de publicaciones activas que recibe al menos una intención cualificada en 30 días.
- **G3:** intenciones cualificadas por publicación activa, segmentadas por categoría y zona.

No usar “reservas”, GMV o arriendos cerrados como North Star hasta que esas conversiones existan y se midan de punta a punta.

### 12.2 KPIs de oferta

| KPI                          | Fórmula                                            | Frecuencia             |
| ---------------------------- | -------------------------------------------------- | ---------------------- |
| CTR de captación             | clics / impresiones                                | Semanal                |
| Tasa landing → inicio        | `start_publish` / sesiones de landing              | Semanal                |
| Tasa de finalización         | publicaciones con mínimos / borradores iniciados   | Semanal                |
| Tasa de activación           | publicaciones visibles / publicaciones con mínimos | Semanal                |
| Costo por publicación activa | gasto de oferta / publicaciones activas atribuidas | Mensual                |
| Tiempo a publicación         | fecha visible − fecha de inicio                    | Semanal, mediana y P75 |
| Tiempo a primera intención   | primera intención − fecha visible                  | Mensual, mediana y P75 |

### 12.3 KPIs de demanda

| KPI                          | Fórmula                                            | Frecuencia |
| ---------------------------- | -------------------------------------------------- | ---------- |
| CTR de anuncio o SERP        | clics / impresiones                                | Semanal    |
| Búsqueda activa              | sesiones con búsqueda/filtro / sesiones de landing | Semanal    |
| Tasa listado → ficha         | sesiones con ficha / sesiones con listado          | Semanal    |
| Tasa ficha → intención       | `begin_rental_intent` / vistas de ficha            | Semanal    |
| Costo por intención          | gasto de demanda / intenciones cualificadas        | Mensual    |
| Cobertura de cero resultados | búsquedas sin resultados / búsquedas               | Semanal    |
| Retorno contextual exitoso   | retornos válidos / accesos contextuales            | Semanal    |

### 12.4 KPIs SEO

- Páginas indexables válidas enviadas a sitemap.
- Páginas indexadas con impresiones.
- Clics orgánicos no marca y marca.
- CTR por consulta y landing.
- Posición como diagnóstico, no como conversión.
- Fichas retiradas aún indexadas.
- URLs de facetas descubiertas/rastreadas.
- Conversiones de oferta y demanda asistidas por orgánico.

### 12.5 Guardrails

- Cero claims no aprobados en anuncios activos.
- Cero campañas de demanda hacia una categoría/zona que no pase el gate G1.
- Cero formularios de captación sin política, consentimiento y destino de datos.
- Cero PII en analítica o UTMs.
- Menos de 5% de eventos críticos duplicados o perdidos; objetivo inicial a validar técnicamente.
- 100% de anuncios con landing específica y estado real del inventario revisado antes de activación.

### 12.6 Metas

No fijar CPA, ROAS o conversión como promesa antes de tener baseline. Procedimiento:

1. Ejecutar 2–4 semanas con presupuesto controlado.
2. Excluir tráfico interno, bots y eventos inválidos.
3. Calcular mediana y rango por canal, categoría y zona.
4. Fijar metas del siguiente ciclo sobre la línea base, no sobre benchmarks genéricos.
5. Registrar cada meta en una tabla con versión, periodo, owner y criterio de éxito.

---

## 13. Calendario de 12 semanas

| Semana | Objetivo                        | Entregables                                                        | Gate de salida                                    |
| ------ | ------------------------------- | ------------------------------------------------------------------ | ------------------------------------------------- |
| 1      | Alinear verdad de producto      | Matriz de claims, rutas, categorías, estados y CTAs                | Producto/Backend confirman capacidades            |
| 2      | Diseñar medición                | Especificación de eventos, UTMs, dashboard y exclusión de fixtures | Eventos críticos comprobables                     |
| 3      | Publicar base de oferta         | `/publicar` y landings residenciales/flexibles con copy aprobado   | CTA funciona o queda explícitamente deshabilitado |
| 4      | Lanzar contenido P0             | 2 guías residenciales + 2 flexibles                                | Indexación, canonical y CTA revisados             |
| 5      | Probar captación de oferta      | Search + social en una zona/categoría                              | Tráfico y conversiones medibles                   |
| 6      | Mejorar onboarding              | Análisis de abandono y primera prueba CRO                          | Hipótesis y resultado documentados                |
| 7      | Auditar inventario              | Conteo real por categoría/zona y calidad de ficha                  | Selección de micro mercados G1                    |
| 8      | Activar demanda controlada      | Search para máximo 1–2 micro mercados aptos                        | Sin páginas vacías; intención medida              |
| 9      | Fortalecer páginas de categoría | Copy, enlaces, estados vacíos, FAQ y schema básico                 | QA SEO y accesibilidad                            |
| 10     | Ampliar contenido               | Guías basadas en consultas y preguntas reales                      | Cada pieza tiene conversión definida              |
| 11     | Retargeting o lifecycle         | Solo con consentimiento, audiencia e infraestructura               | Frecuencia y exclusiones configuradas             |
| 12     | Revisión trimestral             | Cohortes, costos, liquidez, SEO, backlog                           | Decisión: escalar, corregir o pausar              |

---

## 14. Backlog priorizado

### P0 — Antes de captar tráfico

- [ ] Aprobar posicionamiento y hero por estado de producto.
- [ ] Definir URL canónica del marketplace y evitar duplicación `/products` vs `/explorar`.
- [ ] Implementar matriz de claims y revisión de copy.
- [ ] Definir conversiones de oferta y demanda.
- [ ] Distinguir fixtures y datos reales en UI, analítica y SEO.
- [ ] Crear landing general de publicación.
- [ ] Crear landing de publicación residencial.
- [ ] Crear landing de espacios flexibles.
- [ ] Configurar canonical, robots y sitemap.
- [ ] Validar política de privacidad y consentimiento antes de leads o remarketing.

### P1 — Oferta y contenido

- [ ] Landings `publicar/casa`, `publicar/habitacion` y `publicar/casa-de-veraneo`.
- [ ] Landings para oficina, bodega, estacionamiento y eventos.
- [ ] Cuatro guías P0.
- [ ] Campaña Search de oferta.
- [ ] Creatividades sociales por tipo de espacio.
- [ ] Dashboard de activación y tiempo a publicación.

### P2 — Demanda condicionada

- [ ] Evaluar gate G1 por categoría/zona.
- [ ] Activar páginas SEO de categoría con inventario.
- [ ] Campañas Search de demanda por micro mercado.
- [ ] Mejorar filtros basados en datos reales.
- [ ] Medir intención cualificada por publicación.
- [ ] Añadir landings locales solo cuando aporten valor.

### P3 — Escala

- [ ] Lifecycle consentido.
- [ ] Alianzas de inventario.
- [ ] Automatización de alertas reales.
- [ ] Datos estructurados especializados si hay elegibilidad.
- [ ] Modelos de atribución y pruebas incrementales.

---

## 15. Experimentos

| ID     | Hipótesis                                      | Variante A            | Variante B                   | Métrica principal          | Guardrail                             |
| ------ | ---------------------------------------------- | --------------------- | ---------------------------- | -------------------------- | ------------------------------------- |
| EXP-01 | Separar intención mejora avance                | Dos CTAs en hero      | Selector “arrendar/publicar” | Inicio del funnel correcto | Rebote y errores de navegación        |
| EXP-02 | Copy específico mejora oferta                  | “Publicar mi espacio” | “Comenzar publicación”       | `start_publish`            | Calidad/completitud                   |
| EXP-03 | Casa de veraneo responde a motivación concreta | Copy general inmueble | Copy casa de veraneo         | Inicio residencial         | No sugerir ingresos garantizados      |
| EXP-04 | Mostrar requisitos reduce abandono tardío      | CTA directo           | Checklist previo             | Finalización               | No bajar inicio cualificado           |
| EXP-05 | Landing específica mejora Search               | `/publicar` general   | `/publicar/bodega`           | Publicación activa         | Volumen suficiente                    |
| EXP-06 | Orden por tarea mejora portada                 | Categorías primero    | Intención primero            | Ficha o inicio publicación | Accesibilidad y tiempo de interacción |

Reglas:

- Un experimento debe tener una sola hipótesis principal.
- Definir duración o tamaño mínimo antes de mirar resultados.
- No declarar ganador con diferencias pequeñas y muestra insuficiente.
- No experimentar con privacidad, consentimiento o claims de seguridad.

---

## 16. Límites y gobernanza

### 16.1 Matriz de claims

| Tema           | Permitido                                 | Condicional                                          | No usar sin implementación/evidencia |
| -------------- | ----------------------------------------- | ---------------------------------------------------- | ------------------------------------ |
| Inventario     | “Categorías disponibles en la plataforma” | “Casas publicadas en X”                              | “Miles de casas”                     |
| Precio         | “Tarifa declarada por hora/día/mes”       | “Desde $X” calculado desde oferta visible comparable | “Mejor precio”                       |
| Disponibilidad | “Revisa disponibilidad”                   | “Disponible” desde Go para intervalo                 | “Siempre disponible”                 |
| Publicación    | “Comienza una publicación”                | “Publica hoy” si el flujo termina en visible         | “Publicación garantizada”            |
| Identidad      | “Cuenta”                                  | Estado real expuesto por Go                          | “Usuario verificado”                 |
| Pago           | No promover                               | Solo capacidad productiva aprobada                   | “Pago seguro/protegido”              |
| Contrato       | Información general                       | Solo flujo productivo aprobado                       | “Contrato legal garantizado”         |
| Garantías      | Explicar si existen reglas públicas       | Solo política implementada                           | “Arriendo sin riesgo”                |
| Rentabilidad   | “Dale uso a tu espacio”                   | Datos propios con metodología                        | “Gana $X” o “ingreso garantizado”    |

### 16.2 Revisión antes de publicación

Cada landing, anuncio o correo debe pasar por:

1. **Producto:** la capacidad y el estado existen.
2. **Datos/Backend:** inventario, tarifa, modalidad y disponibilidad se obtienen correctamente.
3. **Privacidad/legal:** formulario, consentimiento, segmentación y claims.
4. **Contenido:** español chileno, claridad y consistencia.
5. **SEO:** canonical, indexación, metadatos y enlaces.
6. **Analítica:** evento y atribución.
7. **QA:** móvil, accesibilidad, estados vacíos y errores.

### 16.3 Criterios para pausar una campaña

- Landing sin resultados o caída.
- CTA no completa la acción prometida.
- Eventos de conversión rotos o duplicados.
- Claim cuestionado o capacidad retirada.
- Costo creciente con señales de baja calidad durante dos revisiones consecutivas.
- Inventario de la categoría/zona cae bajo el gate acordado.
- Quejas de privacidad o segmentación.

---

## 17. Definición de terminado por activo

### Landing

- [ ] Audiencia e intención únicas.
- [ ] H1, CTA y metadatos aprobados.
- [ ] Inventario o estado vacío real.
- [ ] Sin claims no soportados.
- [ ] Canonical y política de indexación.
- [ ] Eventos comprobados.
- [ ] Accesibilidad y móvil revisados.
- [ ] Responsable y fecha de actualización.

### Campaña

- [ ] Gate de inventario cumplido.
- [ ] Estructura por categoría/zona.
- [ ] Negativas iniciales.
- [ ] Anuncios y landing coherentes.
- [ ] UTMs válidas.
- [ ] Conversión primaria y guardrails.
- [ ] Presupuesto, duración y criterio de pausa.
- [ ] Revisión de políticas de la plataforma en la fecha de activación.

### Contenido

- [ ] Intención y consulta objetivo.
- [ ] Respuesta útil, no texto para rellenar.
- [ ] Fuentes y fecha.
- [ ] CTA adecuado.
- [ ] Enlaces internos.
- [ ] Claims revisados.
- [ ] Plan de actualización.

---

## 18. Decisiones recomendadas para la portada

1. Usar como idea central: **“Arrienda un espacio. Publica el tuyo.”**
2. Mostrar dos CTAs de igual jerarquía semántica, pero distinguir visualmente demanda y oferta.
3. Separar el catálogo en dos entradas:
   - `Inmuebles`;
   - `Espacios flexibles`.
4. Mostrar tipos específicos dentro de cada grupo y ocultar categorías sin disponibilidad real cuando la portada sea productiva.
5. Incluir la frase **“por hora, día o mes, según cada publicación”** como aclaración, no como promesa para todo el inventario.
6. Incluir el bloque solicitado:
   - **“¿Tienes una casa de veraneo? Publícala y arriéndala en EspaciGo.”**
7. Reemplazar el newsletter deshabilitado del prototipo por contenido útil; solo activar captación cuando exista almacenamiento, consentimiento y gestión de preferencias.
8. No etiquetar como sintéticas las publicaciones reales entregadas por Go; sí declarar que la categoría residencial está vacía y que el flujo de publicación no está habilitado. Mantener cualquier ensayo separado y no promocionarlo.
9. No mostrar ratings, cifras de usuarios, contadores de oferta, logos de medios o testimonios sin evidencia.
10. No usar “Reservar ahora”; preferir `Ver detalle`, `Revisar condiciones` o la acción exacta que el backend permita.

---

## 19. Próxima decisión de producto

Antes de convertir este plan en páginas y campañas, aprobar explícitamente:

1. La frase de posicionamiento.
2. El hero para modo demo y para modo productivo.
3. La ruta canónica del marketplace.
4. El CTA privado disponible desde la ficha.
5. Los estados exactos del funnel de publicación.
6. La definición de “intención cualificada”.
7. El gate G1 por categoría/zona.
8. El primer micro mercado de captación de oferta.
9. La política de leads, consentimiento y comunicaciones.
10. El owner operativo del tablero y del control de claims.

## Sources

[1] https://developers.google.com/crawling/docs/faceted-navigation — Managing crawling of faceted navigation URLs
[2] https://developers.google.com/search/docs/crawling-indexing/canonicalization — What is URL Canonicalization
[3] https://developers.google.com/search/docs/appearance/structured-data/vacation-rental — Vacation Rental structured data
[4] https://support.google.com/google-ads/answer/6167122 — Best practices for responsive search ads
[5] https://support.google.com/google-ads/answer/6167114 — Create messaging that reflects your brand
[6] https://support.google.com/analytics/answer/9267735?hl=es-PE — GA4 eventos recomendados
