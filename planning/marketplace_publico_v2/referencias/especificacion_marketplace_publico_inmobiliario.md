# Especificación funcional y UX/UI: Marketplace inmobiliario público

**Proyecto:** Marketplace integral de arriendos inmobiliarios  
**Documento:** Requerimientos y guía de implementación del catálogo público  
**Estado:** Decisión de producto aprobada para implementación  
**Fecha:** 13 de octubre de 2026  
**Tecnologías existentes/seleccionadas:** Angular + TypeScript + SCSS, backend Go, PostgreSQL, Docker, Google Cloud Run.  
**Identidad visual:** Terracota + Crema (aplicar los tokens y componentes definidos en la guía UX/UI y SCSS).

---

## 1. Decisión principal

**El catálogo de propiedades inmobiliarias debe ser público y navegable sin iniciar sesión ni crear una cuenta.**

Cualquier visitante puede entrar al sitio, explorar categorías, buscar por ubicación, aplicar filtros, ver los resultados y abrir la ficha completa de una propiedad publicada. **El registro e inicio de sesión se solicitan solamente cuando el usuario intenta realizar una acción que requiere una identidad y una relación privada con la plataforma**, como enviar una postulación, solicitar formalmente una visita, iniciar una operación de arriendo, firmar un contrato o pagar.

Esto reemplaza el flujo actual **landing → categorías → inicio de sesión → publicaciones**, que introduce una barrera innecesaria para descubrir inmuebles.

**Nuevo flujo:** **landing → búsqueda/categorías → resultados públicos → ficha pública → acción de interés → autenticación (solo si es necesaria) → continuación de la acción**.

### Declaración breve para el equipo

> Implementar el marketplace bajo un modelo **public-first**, similar a una tienda o marketplace de productos: el inventario publicado debe ser visible para visitantes anónimos. Incorporar una **Product Listing Page (PLP)** adaptada a inmuebles, una **Property Detail Page (PDP)** pública y un sistema de **búsqueda facetada (faceted search)** con filtros persistentes en la URL. La autenticación debe aplicarse a operaciones privadas, no a la exploración del catálogo.

---

## 2. Terminología de diseño

| Término | Significado para este producto |
|---|---|
| **Public-first marketplace** | La experiencia de descubrimiento es pública; el registro ocurre cuando aporta valor. |
| **PLP — Listing / Search Results Page** | Página de resultados que muestra múltiples propiedades en tarjetas, con filtros, ordenamiento y paginación. |
| **PDP — Property Detail Page** | Ficha pública de una propiedad con fotografías, precio, características, ubicación aproximada y condiciones visibles. |
| **Faceted search / navegación facetada** | Búsqueda por múltiples dimensiones: tipo, ubicación, precio, dormitorios, baños, superficie, etc. |
| **Filter sidebar / panel lateral de filtros** | Bloque con filtros a la izquierda en escritorio; en móvil se convierte en un panel desplegable. |
| **Progressive registration / autenticación diferida** | Pedir registro o inicio de sesión al iniciar una acción protegida, sin bloquear la exploración. |
| **Search and discovery** | Experiencia de exploración, búsqueda, filtros, categorías y recomendaciones. |
| **SSR / prerenderizado** | Generación de HTML indexable para las páginas públicas cuando corresponda. |

Los términos *PLP* y *PDP* provienen del comercio electrónico, pero se utilizan aquí para facilitar la comunicación del patrón visual; los contenidos son inmobiliarios, no productos físicos.

---

## 3. Objetivos y alcance

### Objetivos

1. Permitir explorar todas las propiedades **publicadas, vigentes y aptas para difusión pública** sin cuenta.
2. Dar visibilidad al inventario mediante buscadores, enlaces compartidos y navegación por categorías.
3. Facilitar una búsqueda rápida y comprensible, especialmente desde dispositivos móviles.
4. Convertir visitantes interesados en usuarios registrados **en el momento de postular o gestionar un arriendo**.
5. Mantener protegidas las operaciones y los datos privados, aunque el catálogo sea público.
6. No romper ni duplicar la lógica empresarial ya implementada en Go.

### Fuera de alcance de esta etapa

- Rediseñar el backend completo o migrarlo de lenguaje.
- Exigir inicio de sesión para consultar el catálogo.
- Publicar documentos privados, datos personales sensibles o la dirección exacta sin autorización.
- Implementar de entrada un motor de búsqueda dedicado si PostgreSQL cubre los requisitos.
- Implementar pagos o firmas desde componentes públicos sin autorización del servidor.

---

## 4. Experiencia de navegación y mapa de páginas

### 4.1 Flujo de visitante anónimo

```text
Inicio público (landing)
  ├── Buscador principal: comuna, región o ubicación
  ├── Categorías: departamento, casa, habitación, etc.
  ├── Propiedades destacadas (públicas)
  └── Ver todos los arriendos
          ↓
Catálogo público /arriendos
  ├── Panel de filtros + chips activos
  ├── Ordenamiento + contador de resultados
  ├── Tarjetas de propiedades
  └── Paginación / cargar más
          ↓
Ficha pública /arriendos/:slug
  ├── Galería y descripción
  ├── Precio y condiciones públicas
  ├── Ubicación aproximada
  ├── Características y disponibilidad
  └── CTA: Solicitar visita / Postular / Iniciar arriendo
          ↓
Si no inició sesión: registro o login contextual
          ↓
Regresar a la propiedad y continuar la acción solicitada
          ↓
Proceso autenticado: visita, postulación, contrato o pago
```

### 4.2 Rutas propuestas

| Ruta ilustrativa | Visibilidad | Función |
|---|---|---|
| `/` | Pública | Landing con búsqueda directa y propiedades destacadas. |
| `/arriendos` | Pública | Catálogo completo con filtros y resultados. |
| `/arriendos?tipo=departamento&comuna=providencia` | Pública | Búsqueda filtrada, enlazable y compartible. |
| `/arriendos/departamento-en-providencia-12345` | Pública | Ficha del inmueble, URL legible y estable. |
| `/ingresar?returnUrl=...` | Pública | Autenticación preservando el destino o acción segura. |
| `/registro?returnUrl=...` | Pública | Registro contextual. |
| `/mi-cuenta/postulaciones` | Privada | Postulaciones del usuario. |
| `/mi-cuenta/contratos` | Privada | Contratos propios. |
| `/mi-cuenta/pagos` | Privada | Cobros y pagos propios. |
| `/propietario/propiedades` | Privada con rol | Gestión de publicaciones del propietario. |
| `/administracion` | Privada con rol | Moderación y administración. |

**Nota:** la estructura es propuesta, no obligación de renombrar endpoints actuales. Las URLs concretas se ajustarán al enrutado existente y al modelo de inmuebles. Evitar identificadores sensibles en rutas públicas.

---

## 5. Diseño de la PLP: página de resultados inmobiliarios

### 5.1 Estructura en escritorio

```text
┌──────────────────────────────────────────────────────────────────────────────┐
│ Logo | Arriendos | Categorías | Cómo funciona | Publicar | Ingresar         │
├──────────────────────────────────────────────────────────────────────────────┤
│ [ ¿Dónde quieres arrendar?                                  ] [ Buscar ]    │
├───────────────────┬──────────────────────────────────────────────────────────┤
│ FILTROS           │ Arriendos en Santiago        128 resultados            │
│                   │ [Comuna: Santiago ×] [2+ dormitorios ×] [Limpiar]      │
│ Ubicación         │ Ordenar por: [Más recientes ▼]  Vista: [Cuadrícula]    │
│ Tipo de inmueble  │                                                          │
│ Precio mensual    │ [Foto] Tarjeta A       [Foto] Tarjeta B                │
│ Dormitorios       │ Precio, zona, atributos, disponibilidad                │
│ Baños             │                                                          │
│ Superficie        │ [Foto] Tarjeta C       [Foto] Tarjeta D                │
│ Admite mascotas   │                                                          │
│ Amoblado          │ [Paginación / siguientes resultados]                    │
│ Disponibilidad    │                                                          │
└───────────────────┴──────────────────────────────────────────────────────────┘
```

El **panel de filtros debe ubicarse a la izquierda en escritorio** como patrón predeterminado. Puede ser fijo al desplazar dentro de límites razonables, sin tapar contenido. El contenido principal muestra tarjetas y encabezado de resultados.

### 5.2 Estructura móvil

- Encabezado compacto, búsqueda prominente y control **«Filtros»**.
- Filtros en **drawer / bottom sheet / diálogo de pantalla completa** accesible: no usar barra lateral permanente.
- Mostrar número de filtros activos y acción **«Ver N propiedades»**.
- Tarjetas en una columna; navegación y acciones fáciles de tocar.
- Mantener filtros al abrir una ficha y volver a los resultados.
- No obligar al usuario a desplazarse por un panel enorme antes de ver inmuebles.

### 5.3 Componentes principales

- `MarketplaceSearchBar`
- `PropertyFiltersPanel`
- `ActiveFilterChips`
- `PropertySortSelect`
- `PropertyResultsSummary`
- `PropertyCard`
- `PropertyGrid`
- `Pagination`
- `EmptyResultsState`
- `FilterMobileDialog`

Nombres referenciales de componentes Angular. Los componentes visuales siguen el **Design System Terracota + Crema** y la arquitectura **Feature-Based + Hexagonal ligera** acordada.

---

## 6. Taxonomía y filtros

### 6.1 Filtros prioritarios para MVP público

| Filtro | Control UX | Observaciones |
|---|---|---|
| Ubicación | Autocompletado por región/comuna/sector | No exigir dirección exacta. |
| Tipo | Casillas o selección múltiple | Casa, departamento, habitación, etc.; catálogo administrable. |
| Precio mensual | Mínimo y máximo | CLP; si se admite UF, explicitar unidad y conversión. |
| Dormitorios | Selección por cantidad mínima o rango | Incluir estudio/0 cuando aplique. |
| Baños | Selección por cantidad mínima | Opciones simples. |
| Superficie | Rango m² | Distinguir útil y total según los datos disponibles. |
| Amoblado | Sí / No / Indistinto | Solo si el atributo existe en los datos. |
| Mascotas | Permitidas / Indistinto | Evitar prometer reglas no verificadas. |
| Disponibilidad | Fecha o condición | Mostrar únicamente información confiable y actualizada. |

### 6.2 Filtros posteriores según datos reales

Estacionamiento, bodega, gastos comunes, accesibilidad, ascensor, terraza, piso, duración de arriendo, cercanía a transporte, gastos incluidos y equipamiento. **No mostrar filtros vacíos o sin respaldo en el modelo de datos.**

### 6.3 Reglas de comportamiento

1. Los filtros se combinan de forma predecible (**AND** entre dimensiones; **OR** dentro de selecciones múltiples de una misma dimensión, si se admite).
2. Mostrar filtros activos como chips removibles y botón **«Limpiar filtros»**.
3. Conservar filtros en la URL (`query parameters`) para compartir y volver.
4. Validar tipos, límites y valores permitidos tanto en frontend como en Go.
5. Si no hay resultados, explicar el estado y ofrecer quitar o relajar filtros.
6. Mostrar carga, error y estado vacío distintos; no confundir «sin resultados» con error de servidor.
7. Ordenar con criterio explícito: recomendados (si existe ranking definido), recientes, menor precio y mayor precio.
8. Paginación estable (cursor o página según API); controlar tamaños máximos y evitar descargar todo el catálogo.
9. Mostrar precio, unidad monetaria y periodicidad claramente: por ejemplo, **$550.000 CLP/mes**.
10. Solo mostrar recuentos por faceta si el backend puede proporcionarlos de manera fiable.

---

## 7. Tarjetas de propiedad y ficha pública (PDP)

### Tarjeta de catálogo

Debe incluir:

- Fotografía principal optimizada, con alternativa textual adecuada.
- Tipo de inmueble, título breve y ubicación general.
- Precio de arriendo mensual con moneda y periodicidad.
- Dormitorios, baños y superficie (cuando correspondan).
- Etiquetas verificables, por ejemplo «Disponible» o «Publicación verificada» **solo cuando el backend las respalde**.
- Enlace a la ficha completa.

No mostrar datos internos como IDs privados, identidad documental del propietario, evaluaciones internas o información confidencial.

### Ficha pública

Debe incluir: galería, descripción, precio, gastos y condiciones que correspondan, características, reglas públicas, ubicación aproximada o mapa con precisión apropiada, disponibilidad confiable, identificación pública permitida del anunciante y CTA principal.

**CTA recomendado:** «Solicitar visita» o «Postular a esta propiedad». El botón «Arrendar ahora» solo debe usarse si el negocio realmente permite completar la contratación en ese flujo y los requisitos de postulación, firma y pago están resueltos.

La **dirección exacta** y los detalles privados de contacto se publican únicamente con autorización y conforme a políticas de privacidad; no deben exponerse automáticamente solo por existir en base de datos.

---

## 8. Autenticación diferida y permisos

### Acceso anónimo permitido

- Consultar landing y categorías.
- Buscar propiedades y combinar filtros.
- Ver fotos, precios, descripciones y ficha pública.
- Compartir enlaces y navegar entre resultados.
- Consultar información pública de funcionamiento y condiciones generales.

### Acciones protegidas

- Guardar favoritos persistentes en una cuenta (opcional: favoritos temporales locales sin cuenta).
- Enviar mensajes privados o datos de contacto mediante la plataforma.
- Solicitar formalmente visitas y enviar postulaciones.
- Subir antecedentes y consultar su evaluación.
- Suscribir o firmar contratos.
- Ejecutar pagos, consultar pagos personales y gestionar garantías.
- Crear, editar o administrar propiedades.

### Regla crítica de experiencia

Cuando una persona no autenticada pulse «Postular», **no enviarla a una pantalla genérica que haga perder el contexto**. Mostrar una explicación breve, ofrecer ingresar o registrarse y, después de autenticarse, devolverla a la misma propiedad y al paso iniciado.

**Seguridad:** validar el destino (`returnUrl`) contra rutas internas permitidas para evitar *open redirects*. Después del login, volver a comprobar en Go autorización, estado de la propiedad y disponibilidad. El hecho de que Angular oculte una acción **no constituye un control de seguridad**.

---

## 9. Requerimientos de backend Go y API

El catálogo público requiere endpoints o rutas equivalentes accesibles **sin token de sesión**, con proyecciones de datos públicas deliberadamente limitadas.

| Operación ilustrativa | Acceso | Resultado |
|---|---|---|
| `GET /api/public/properties` | Público | Listado filtrado y paginado. |
| `GET /api/public/properties/{slug}` | Público | Ficha pública de inmueble publicado. |
| `GET /api/public/property-types` | Público | Taxonomía disponible. |
| `GET /api/public/locations/suggest?q=...` | Público | Sugerencias de ubicaciones. |
| `POST /api/applications` | Autenticado | Crear postulación; autorización y validación de negocio. |
| `POST /api/visits` | Autenticado | Solicitar visita. |

**Los nombres son propuestas**, no instrucciones para duplicar endpoints si ya existen rutas reutilizables. El contrato OpenAPI debe distinguir DTO públicos de DTO privados. No se recomienda devolver un DTO completo del administrador y ocultar campos solo con Angular.

### Reglas servidor

- Filtrar por estado de publicación: solo registros **publicados**, no retirados, no borradores ni suspendidos.
- Validar que el inmueble puede mostrarse públicamente y su disponibilidad real.
- Aplicar límites, paginación, ordenamiento permitido e índices SQL para filtros frecuentes.
- Considerar PostgreSQL + PostGIS para filtros geográficos cuando corresponda.
- Limitar abuso mediante rate limiting, protección contra scraping abusivo y monitoreo, sin bloquear injustificadamente rastreadores legítimos.
- Nunca exigir token para endpoints que solo entregan el catálogo público.
- Mantener todas las operaciones financieras y contractuales autorizadas en Go; los split payments no se procesan desde el cliente.

---

## 10. Angular: arquitectura y renderizado

Mantener las decisiones existentes:

- Angular + TypeScript `strict`.
- Componentes standalone y rutas diferidas cuando corresponda.
- Arquitectura Feature-Based con puertos/adaptadores solo donde aporten valor.
- Signals para estado local y NgRx SignalStore solo cuando sea útil.
- SCSS encapsulado por componente y Design System propio.
- SSR o prerenderizado para páginas públicas indexables; CSR o renderizado híbrido para zonas privadas.

### Estructura sugerida

```text
frontend/src/app/
├── core/
│   ├── auth/
│   └── api/
├── features/
│   ├── marketplace/
│   │   ├── pages/
│   │   │   └── listing-page/
│   │   ├── components/
│   │   │   ├── search-bar/
│   │   │   ├── filters-panel/
│   │   │   ├── property-card/
│   │   │   └── property-grid/
│   │   ├── application/
│   │   ├── ports/
│   │   └── adapters/
│   ├── property-detail/
│   ├── applications/
│   ├── contracts/
│   └── payments/
├── shared/
│   └── ui/
└── app.routes.ts
```

El módulo de marketplace público no debe importar servicios de autenticación como requisito para **leer propiedades**. Puede utilizar autenticación únicamente para activar flujos privados.

### SEO técnico

- Entregar HTML con contenido principal en SSR para páginas públicas relevantes.
- Títulos y descripciones únicos, URLs amigables, canonical y sitemap con fichas válidas.
- Metadatos sociales para enlaces compartidos e imágenes adecuadas.
- No indexar paneles privados, login ni URLs con datos sensibles.
- Evitar indexar indiscriminadamente cada combinación de filtros; definir reglas de canonical/noindex para búsquedas facetadas y páginas de escaso valor.
- Cuando un inmueble ya no existe o deja de ser público, devolver respuesta y estrategia SEO acordes al estado (por ejemplo, 404/410 o redirección justificada); no exponer borradores.
- Optimizar imágenes, tamaño de JavaScript y rendimiento móvil.

---

## 11. UX/UI y diseño visual aprobados

**Identidad:** Terracota + Crema, estilo cálido, moderno, minimalista y profesional. Aplicar la guía de diseño previa como referencia principal.

| Rol visual | Color aprobado |
|---|---|
| Principal | `#A44731` |
| Secundario | `#8A6A4D` |
| Acento | `#D97745` |
| Fondo crema | `#FBF7F2` |
| Texto principal | `#332720` |
| Éxito | `#15803D` |
| Advertencia | `#B45309` |
| Error | `#B91C1C` |
| Información | `#1D4ED8` |

### Estilo de la experiencia pública

- Buscador visible inmediatamente, incluso en la landing.
- Fotografía de propiedades como protagonista; evitar saturación decorativa.
- Fondo crema suave y superficies claras para tarjetas y formularios.
- Jerarquía tipográfica fuerte para precio, zona y características.
- Etiquetas y estados con significado estable; nunca comunicar un estado solo con color.
- Filtros sobrios y compactos, con secciones desplegables si es necesario.
- Botones y campos definidos en el Design System; evitar estilos únicos por pantalla.
- Contraste WCAG 2.2 AA, foco visible, navegación con teclado, controles etiquetados.
- Cargas con skeletons o indicadores adecuados, y mensajes comprensibles ante errores.

### Arquitectura SCSS

Usar **Design Tokens (variables CSS)** para semántica visual, **Sass Modules (`@use`)** para mixins y funciones y **SCSS de componente encapsulado** para estilos locales. Mantener mínimos los estilos globales; no crear hojas CSS gigantes por página ni duplicar botones, tarjetas o formularios.

```text
frontend/src/styles/
├── tokens/
│   ├── _colors.scss
│   ├── _spacing.scss
│   └── _typography.scss
├── abstracts/
│   └── _breakpoints.scss
├── themes/
│   ├── _light.scss
│   └── _dark.scss
└── styles.scss
```

La guía UX/UI y SCSS previamente aprobada define el detalle de los tokens, componentes y reglas de calidad. Este documento define **cómo aplicarlos al catálogo público**.

---

## 12. Migración desde el flujo actual

No reemplazar a ciegas la landing ni eliminar el mockup hasta verificar el nuevo flujo.

1. **Auditar el estado actual:** identificar dónde se fuerza el login (guard de Angular/mockup, ruta, middleware Go o endpoint protegido).
2. **Separar lectura pública de operaciones privadas** en la API o ajustar las rutas existentes sin exponer campos sensibles.
3. **Crear PLP pública** con filtros y tarjetas conectadas al backend real.
4. **Crear PDP pública** para cada inmueble publicado.
5. **Modificar landing y categorías** para que enlacen directamente a catálogo público filtrado.
6. **Implementar autenticación contextual** para acciones protegidas y retorno al inmueble.
7. **Agregar SEO y SSR** según configuración Angular/Cloud Run.
8. **Probar seguridad, accesibilidad y experiencia móvil**, y luego sustituir el mockup anterior.

Si el backend actual tiene endpoints de listados protegidos por diseño, su modificación debe hacerse de manera controlada y con nuevos DTO públicos, evitando eliminar autorizaciones de operaciones sensibles.

---

## 13. Historias de usuario y criterios de aceptación

### HU-PUB-001 — Explorar propiedades sin cuenta

**Como** visitante anónimo, **quiero** visualizar propiedades disponibles **para** decidir si una me interesa antes de registrarme.

**Criterios de aceptación:**
- La landing permite llegar al catálogo sin login.
- Las categorías llevan a resultados públicos sin login.
- Las tarjetas muestran propiedades publicadas con información pública suficiente.
- Abrir una propiedad no redirige a ingresar.
- Un enlace directo a una ficha funciona en una sesión anónima o ventana privada.

### HU-PUB-002 — Buscar y filtrar

**Como** visitante, **quiero** filtrar por ubicación, tipo, precio y características **para** encontrar arriendos adecuados.

**Criterios de aceptación:**
- En escritorio, los filtros están en panel lateral.
- En móvil, pueden abrirse y cerrarse sin perder el contexto.
- Los filtros actualizan resultados y URL de forma coherente.
- Existen estados de carga, error y cero resultados.
- Se pueden limpiar filtros y ordenar resultados.

### HU-PUB-003 — Consultar ficha pública

**Como** visitante, **quiero** ver fotografías, precio, características y condiciones públicas **para** evaluar la propiedad.

**Criterios de aceptación:**
- No se exige sesión.
- La URL es compartible y accesible directamente.
- No se exponen datos privados, documentos confidenciales o direcciones exactas no autorizadas.
- Una propiedad no publicada no aparece en resultados y no expone ficha pública.

### HU-PUB-004 — Registrarme cuando decido actuar

**Como** visitante interesado, **quiero** registrarme solo al postular o solicitar un servicio privado **para** no perder tiempo antes de encontrar una propiedad.

**Criterios de aceptación:**
- La plataforma solicita autenticación al comenzar una acción protegida.
- Explica por qué necesita la cuenta.
- Tras el login/registro, regresa al inmueble y continúa el proceso cuando sea válido.
- Si la propiedad dejó de estar disponible, lo explica y evita generar operaciones inválidas.

### HU-PUB-005 — Indexación de catálogo

**Como** responsable de la plataforma, **quiero** que Google y otros buscadores puedan descubrir fichas públicas **para** captar visitantes de manera orgánica.

**Criterios de aceptación:**
- Las fichas entregan HTML y metadatos públicos adecuados.
- Sitemap incluye únicamente URLs válidas e indexables.
- No se indexan datos personales, rutas privadas ni variaciones de filtros sin valor SEO.

---

## 14. Matriz mínima de pruebas

| Prueba | Resultado esperado |
|---|---|
| Abrir `/arriendos` sin cookies | Muestra listado público; no redirige a login. |
| Abrir una ficha desde un enlace compartido | Muestra detalle público directamente. |
| Buscar con filtros | Resultados correctos y URL actualizada. |
| Usar filtros en móvil | Panel accesible, resultados legibles. |
| Abrir ficha de borrador/suspendida | No expone información ni documentos privados. |
| Presionar «Postular» anónimo | Muestra autenticación contextual. |
| Completar login | Retorna a la acción cuando corresponda y sigue autorizado. |
| Consultar endpoint público | Devuelve solo campos permitidos y lista paginada. |
| Intentar operar pagos sin sesión | Go rechaza la solicitud. |
| Rastreador visita ficha pública | Recibe HTML con contenido y metadatos apropiados. |
| Cambiar/eliminar filtros | Se actualizan UI, URL y resultados. |
| Cero resultados/error de API | Se muestran estados diferenciados. |

---

## 15. Orden de trabajo para el equipo

| Fase | Entregable | Prioridad |
|---|---|---|
| 1 | Auditoría de guards, endpoints y DTO actuales | Crítica |
| 2 | Lectura pública segura desde Go + contrato OpenAPI | Crítica |
| 3 | PLP pública, tarjetas y filtros MVP | Crítica |
| 4 | PDP pública y enlaces desde landing/categorías | Crítica |
| 5 | Login/registro contextual con retorno al flujo | Alta |
| 6 | SEO, SSR, sitemap, canonical y metadatos | Alta |
| 7 | Accesibilidad, pruebas móviles, rendimiento y carga | Alta |
| 8 | Filtros avanzados, mapa y mejoras de conversión | Evolutiva |

**Definición de terminado:** una persona sin cuenta puede descubrir, buscar, filtrar y evaluar inmuebles publicados; cuando decide postular o realizar una operación privada, puede identificarse y continuar sin perder el inmueble seleccionado. Las API públicas no exponen datos privados y las páginas públicas son compatibles con la estrategia de indexación SEO.

---

## 16. Instrucción lista para el equipo o un agente de desarrollo

> **Objetivo:** corregir la arquitectura de navegación del marketplace inmobiliario para convertirlo en una experiencia **public-first**. Actualmente el flujo landing → categorías exige iniciar sesión para ver las publicaciones; esto debe cambiarse. Implementar un catálogo público de arriendos (PLP) con filtros laterales en escritorio y drawer de filtros en móvil, búsqueda, ordenamiento, paginación, chips de filtros activos y tarjetas de propiedades. Implementar fichas públicas (PDP) accesibles mediante URLs compartibles. Solo requerir autenticación para acciones privadas (postular, solicitar visitas formales, enviar mensajes privados, firmar contratos, pagos, garantías, administración). Tras autenticación, devolver al usuario a la propiedad y a la acción iniciada. Mantener Angular con arquitectura Feature-Based + Hexagonal ligera, TypeScript estricto, Signals y SCSS con Design System Terracota + Crema; Go sigue siendo la autoridad sobre publicación, acceso, contratos y finanzas. Auditar los guards y endpoints actuales antes de modificar; crear DTO públicos limitados, reutilizar OpenAPI y no romper el backend existente. Implementar SSR/SEO para fichas públicas, con controles de indexación para filtros. Entregar cambios por etapas, pruebas automatizadas y criterios de aceptación de este documento. **No exigir login para consultar el catálogo o las fichas públicas.**

---

## 17. Dependencias y decisiones pendientes

Este documento fija el **modelo de acceso público**, no inventa características ya implementadas. Antes de ejecutar, validar:

- Estructura real de URLs, endpoints, permisos y contratos existentes.
- Campos disponibles para cada filtro y estados de publicación reales.
- Regla de visibilidad de dirección exacta e identidad de anunciantes.
- Moneda principal y tratamiento de UF/CLP.
- Funcionalidad precisa de visitas, postulación, reservas y contratación.
- Configuración concreta de Angular SSR y despliegue en Cloud Run.

**Referencias internas recomendadas:** guía de arquitectura frontend y guía UX/UI + SCSS aprobadas previamente. Este documento es un complemento específico para la experiencia pública del catálogo inmobiliario, no las reemplaza.
