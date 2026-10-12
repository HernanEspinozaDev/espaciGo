# Guía de decisiones y ruta de implementación del frontend

**Proyecto:** Marketplace inmobiliario integral de arriendos  
**Fecha de consolidación:** 11 de octubre de 2026  
**Estado:** Documento base de decisiones arquitectónicas (ADR resumido) y hoja de ruta  
**Objetivo:** Estandarizar la construcción del frontend, facilitar su mantenimiento y preparar su despliegue independiente en Google Cloud Run.

---

## 1. Resumen ejecutivo

Se construirá el frontend del marketplace con **Angular 22 (rama elegida en la conversación), TypeScript, componentes standalone, Signals y SCSS**. El código se organizará **por funcionalidades (Feature-Based)**, con **arquitectura hexagonal ligera** en los módulos que presenten lógica de interfaz y coordinación compleja. La presentación utilizará **un Design System propio basado en tokens, componentes Angular compartidos, Angular CDK y SCSS encapsulado**.

El frontend abarcará **portal público indexable** y **paneles privados** para arrendatarios, propietarios/corredores y administradores. Usará **SSR o prerenderizado para páginas públicas**, y **renderizado cliente para áreas privadas**, según los requisitos de cada ruta. Consumirá **API REST del backend Go**, preferentemente con contrato **OpenAPI** y cliente TypeScript generado. Frontend y backend estarán en **contenedores Docker independientes** desplegables en **Cloud Run**; PostgreSQL residirá en **Cloud SQL** en producción.

**Principio rector:** Angular presenta y coordina interacciones; **Go conserva la autoridad sobre las reglas de negocio, seguridad, contratos, pagos, garantías, comisiones y cálculo/confirmación de split payments**.

> **Estado de decisiones:** Angular, SCSS y la arquitectura aquí descrita son la propuesta final consensuada en la conversación. Algunos aspectos operativos y de proveedores se mantienen expresamente pendientes de validación antes de implementarse.

## 2. Contexto y alcance del negocio

El producto es un **marketplace integral de arriendos inmobiliarios**. No será únicamente un portal de anuncios: busca cubrir el ciclo de vida completo del arriendo:

1. Registro, identidad y perfiles.
2. Publicación y descubrimiento de inmuebles, filtros y búsquedas geográficas.
3. Solicitudes de visita, contacto, postulaciones y documentación.
4. Evaluación y aprobación de postulaciones.
5. Generación, versionado y firma de contratos.
6. Gestión del arriendo activo, obligaciones periódicas y renovaciones.
7. Pagos, garantías, devoluciones, liquidaciones y comisiones.
8. Operaciones administrativas, auditoría y atención de disputas.

**Modelo de monetización definido:** la plataforma cobra **una comisión mediante split payments por transacción**. La distribución efectiva dependerá de la integración con un proveedor que soporte marketplace/split de pagos, sus reglas de liquidación y el marco legal aplicable. **Las garantías no deben tratarse automáticamente como un split de comisión**.

**Usuarios principales:** visitantes no autenticados; arrendatarios; propietarios y, cuando corresponda, corredores/administradores; personal de administración. Un usuario puede desempeñar más de un rol. La autorización es por recurso, no solamente por visibilidad de pantallas.

## 3. Registro consolidado de decisiones

| ID | Decisión | Justificación / alcance | Estado |
|---|---|---|---|
| FE-01 | **Angular 22 + TypeScript** | Framework con convenciones, DI, router y herramientas integradas; prioridad: estructura consistente para equipo empresarial. | Elegida; comprobar último parche y compatibilidades antes de iniciar |
| FE-02 | **Standalone Components** | Estilo moderno; evitar NgModules tradicionales como base de la aplicación. | Elegida |
| FE-03 | **Feature-Based Architecture** | Organizar por capacidades del negocio y limitar dependencias entre funcionalidades. | Elegida |
| FE-04 | **Hexagonal ligera y selectiva** | Puertos/adaptadores en flujos complejos, sin replicar capas artificiales en componentes simples. | Elegida |
| FE-05 | **Signals nativos** | Estado reactivo local, formularios, filtros y estados de interfaz. | Elegida |
| FE-06 | **NgRx SignalStore selectivo** | Estado compartido complejo dentro de una funcionalidad, si aparece esa necesidad. | Condicional; no instalar por defecto |
| FE-07 | **No adoptar NgRx Store tradicional inicialmente** | Evitar reducers, actions y effects globales sin necesidad demostrada. | Elegida |
| FE-08 | **HttpClient + RxJS** | Consumo HTTP y composición/cancelación de flujos asíncronos. | Elegida |
| FE-09 | **SCSS en lugar de Tailwind o Bootstrap** | Encapsulamiento Angular, control de convenciones y Design System propio. | Elegida |
| FE-10 | **Design tokens CSS + Sass, CDK y componentes UI propios** | Consistencia visual, accesibilidad y reutilización. | Elegida |
| FE-11 | **SSR/prerender para portal público; CSR para paneles privados** | SEO donde hay páginas indexables e interactividad donde hay sesiones. | Elegida como estrategia; concretar por ruta |
| FE-12 | **API Go separada; contrato OpenAPI** | Evitar duplicar reglas de negocio; cliente TypeScript generado y tipado. | Elegida |
| FE-13 | **Docker y Cloud Run separados para frontend y backend** | Escalado y despliegue independientes; Compose para integración local. | Elegida |
| FE-14 | **Monolito modular Go inicialmente** | Evitar microservicios prematuros; módulos de negocio claros. | Arquitectura de referencia backend |
| FE-15 | **Sin microfrontends al inicio** | Un frontend modular basta hasta existir equipos y ciclos de despliegue autónomos. | Elegida |
| FE-16 | **PostgreSQL en Cloud SQL y fotos/documentos en Cloud Storage** | Datos persistentes y blobs fuera de contenedores. | Arquitectura objetivo |

### Opciones analizadas y descartadas para la primera implementación

- **Next.js:** se evaluó seriamente por SSR, SEO y ecosistema React; finalmente se prefirió Angular por consistencia estructural, DI y convenciones empresariales. Su ventaja SEO no es absoluta frente a Angular correctamente configurado con SSR.
- **Nuxt y SvelteKit:** alternativas válidas con SSR, pero no seleccionadas.
- **React + Vite:** más libertad arquitectónica de la deseada para este proyecto.
- **NgModules como arquitectura principal:** se prefirió standalone; módulos existentes pueden interoperar si una dependencia los requiere.
- **NgRx Store/Redux desde el día uno:** exceso de complejidad sin una necesidad transversal concreta.
- **Tailwind CSS:** solución adecuada y rápida, pero se optó finalmente por SCSS con un sistema de diseño propio.
- **Bootstrap:** no seleccionado; evitar mezclar Bootstrap, Tailwind y SCSS como sistemas paralelos.
- **Angular Material completo:** no es base de diseño elegida; Angular CDK aporta primitivas/comportamientos accesibles sin imponer la estética Material. Puede evaluarse un componente puntual si resulta conveniente.

## 4. Stack frontend seleccionado

| Ámbito | Tecnología / patrón | Uso |
|---|---|---|
| Framework | **Angular 22** (último parche compatible) | Aplicación SPA + SSR híbrido |
| Lenguaje | **TypeScript**, modo `strict` | Tipado y mantenimiento |
| Componentes | **Standalone** | Unidades de interfaz y rutas |
| Navegación | **Angular Router**, lazy loading | Portal público y paneles |
| Reactividad | **Angular Signals**, `computed()` | Estado local y derivado |
| Estado modular avanzado | **NgRx SignalStore**, opcional | Flujos complejos con múltiples vistas |
| Asincronía | **Angular HttpClient + RxJS** | Comunicación API, operadores y cancelación |
| Estilos | **SCSS**, CSS custom properties | Temas, estilos por componente y tokens |
| Comportamientos accesibles | **Angular CDK** | Overlays, accesibilidad y primitivas UI |
| Renderizado | **Angular SSR + prerender/CSR por ruta** | SEO y aplicaciones privadas |
| API | **OpenAPI + cliente TypeScript generado** | Contrato seguro con Go |
| Calidad | Angular CLI, ESLint, Stylelint, pruebas, CI | Convenciones automatizadas |
| Documentación UI | Storybook, recomendable | Catálogo y revisión de componentes |
| Infraestructura | Docker, Docker Compose, Cloud Run | Entornos local y producción |

**No confundir conceptos:** NgModules organizan declaraciones/proveedores en el enfoque tradicional; Signals gestionan reactividad; NgRx gestiona estado. No son opciones mutuamente excluyentes. **Signals + SignalStore** pueden coexistir. **RxJS** continúa siendo útil incluso utilizando Signals.

## 5. Arquitectura frontend: Feature-Based + hexagonal ligera

### 5.1. Principios

1. **Organización por negocio:** `properties`, `search`, `applications`, `visits`, `contracts`, `leases`, `payments`, `deposits`, `commissions`, `messaging`.
2. **UI desacoplada del transporte:** no realizar llamadas HTTP dispersas en componentes visuales.
3. **Dependencias hacia el núcleo:** los casos de uso de interfaz dependen de interfaces/puertos, no de adaptadores concretos.
4. **Interfaces solo donde aportan valor:** una vista informativa simple no requiere seis capas.
5. **La fuente de verdad empresarial es Go:** Angular puede validar entradas para UX, pero Go vuelve a validar, autorizar y procesar toda operación sensible.
6. **No reproducir la lógica financiera del servidor en el navegador:** montos y splits definitivos solo provienen del backend.
7. **La DI de Angular facilita los adaptadores:** como interfaces TypeScript se eliminan al compilar, utilizar `InjectionToken` o clases abstractas cuando se necesite resolver dependencias en tiempo de ejecución.

### 5.2. Estructura de referencia

```text
frontend/
├── src/
│   ├── app/
│   │   ├── core/
│   │   │   ├── auth/
│   │   │   ├── config/
│   │   │   ├── http/
│   │   │   └── layout/
│   │   ├── features/
│   │   │   ├── properties/
│   │   │   ├── search/
│   │   │   ├── applications/
│   │   │   ├── visits/
│   │   │   ├── contracts/
│   │   │   ├── leases/
│   │   │   ├── payments/
│   │   │   ├── deposits/
│   │   │   ├── commissions/
│   │   │   └── messaging/
│   │   ├── shared/
│   │   │   ├── ui/
│   │   │   ├── pipes/
│   │   │   └── utils/
│   │   ├── app.config.ts
│   │   └── app.routes.ts
│   ├── generated/
│   │   └── api/               # Código generado: no editar manualmente
│   ├── styles/
│   │   ├── abstracts/
│   │   ├── tokens/
│   │   ├── base/
│   │   ├── themes/
│   │   └── styles.scss
│   └── main.ts
├── Dockerfile
├── compose.yaml                # O en raíz del repositorio de integración
├── angular.json
└── package.json
```

**Ejemplo de módulo complejo** (`features/payments/`):

```text
payments/
├── domain/
│   ├── models/
│   └── ports/
├── application/
│   └── use-cases/
├── infrastructure/
│   ├── repositories/
│   └── mappers/
├── presentation/
│   ├── pages/
│   ├── components/
│   └── state/
└── payments.routes.ts
```

**Ejemplo de módulo simple:** `features/search/` puede comenzar con `pages/`, `components/`, `data-access/` y `search.routes.ts`. Introducir puertos y casos de uso solo cuando el diseño lo justifique.

### 5.3. Límites de dependencia

```text
Componente Angular / Signals
          ↓
Caso de uso de interfaz (si hace falta)
          ↓
Puerto (contrato TypeScript)
          ↑ implementado por
Adaptador HTTP → cliente OpenAPI → API Go
```

El diagrama anterior indica **flujo de ejecución desde la UI** y **dependencia de código del caso de uso hacia el puerto**: el adaptador implementa el puerto y depende de los detalles HTTP. La infraestructura no debe contaminar el dominio.

## 6. Sistema de diseño SCSS

### 6.1. Elección

Se descarta Tailwind/Bootstrap como base. El sistema se construirá con **SCSS + CSS custom properties + componentes Angular reutilizables + Angular CDK**.

Capas de estilo:

1. **Tokens:** colores semánticos, tipografía, espaciado, radios, sombras, densidades, z-index.
2. **Herramientas Sass:** mixins, funciones y mapas de breakpoints.
3. **Globales mínimos:** normalización, fuentes, temas y estilos base.
4. **Componentes UI:** botón, input, select, modal, alerta, badge, tabla, paginación, skeleton.
5. **Componentes de negocio:** tarjeta de propiedad, ficha de arriendo, estado de postulación, resumen de pago, contrato.

### 6.2. Convenciones obligatorias

- Usar **variables CSS** para tokens que deben cambiar en ejecución (tema claro/oscuro), y variables/mixins Sass para composición en compilación.
- No introducir colores, breakpoints o espaciados arbitrarios en módulos sin justificar y revisar el token correspondiente.
- Mantener estilos de funcionalidades en componentes Angular (encapsulamiento predeterminado), no en hojas globales gigantes.
- Evitar selectores globales profundos, `!important` sistemático y `::ng-deep` indiscriminado.
- BEM puede usarse en componentes complejos, pero no es obligatorio donde el encapsulamiento ya resuelve colisiones.
- Documentar variantes permitidas de componentes (`primary`, `secondary`, `danger`, tamaños, estados disabled/loading).
- Definir foco visible, contraste, navegación por teclado, errores de formulario, estados vacíos y vistas móviles.
- Aplicar **Stylelint**, revisión de componentes y, cuando sea posible, pruebas visuales.

### 6.3. Estructura SCSS

```text
src/styles/
├── abstracts/
│   ├── _breakpoints.scss
│   ├── _mixins.scss
│   └── _functions.scss
├── tokens/
│   ├── _colors.scss
│   ├── _spacing.scss
│   ├── _typography.scss
│   └── _radius.scss
├── base/
│   ├── _reset.scss
│   └── _global.scss
├── themes/
│   ├── _light.scss
│   └── _dark.scss
└── styles.scss
```

**Ejemplo de token:**

```scss
:root {
  --color-primary: #2563eb;
  --color-text: #0f172a;
  --color-background: #ffffff;
  --space-4: 1rem;
  --radius-card: 0.75rem;
}

[data-theme='dark'] {
  --color-text: #f8fafc;
  --color-background: #0f172a;
}
```

Los colores anteriores son **ejemplos técnicos**, no una identidad visual de marca aprobada. La paleta, fuentes, logo y diseño final del marketplace siguen pendientes.

## 7. Estrategia de renderizado y SEO

El marketplace combina dos tipos de experiencia:

| Área | Estrategia inicial | Objetivo |
|---|---|---|
| Portada y landing públicas | SSR o prerender | Rendimiento e indexación |
| Categorías, comunas y resultados seleccionados | SSR selectivo | Páginas indexables útiles |
| Fichas públicas de inmuebles | SSR | Información HTML accesible a buscadores |
| Inicio de sesión | CSR o SSR según UX | Acceso y autenticación |
| Panel de arrendatario | CSR | Flujos privados interactivos |
| Panel de propietario/corredor | CSR | Administración de propiedades, cobros y contratos |
| Administración interna | CSR | Moderación, auditoría y operaciones |

Obligatorio en el portal público:

- Títulos y descripciones únicas.
- Canonicals coherentes y sitemap actualizado.
- URLs legibles para publicaciones y ubicación.
- Datos estructurados apropiados cuando cumplan las directrices aplicables.
- Manejo de propiedades expiradas/retiradas para evitar resultados indexados obsoletos.
- Optimización de fotografías, carga diferida y Core Web Vitals.
- Evitar indexar información privada, contratos, pagos o documentos.

**Nota:** Angular con SSR puede lograr un SEO excelente; no se elige Next.js exclusivamente por esa razón.

## 8. Integración con backend Go y responsabilidades

### El frontend puede

- Capturar datos, mostrar errores y estados de avance.
- Mostrar filtros, listados, perfiles y paneles.
- Mostrar estimaciones de precio o comisión **identificadas como estimaciones**.
- Solicitar al backend crear una postulación, contrato o intención de pago.
- Consultar estados y comprobantes autorizados.
- Gestionar estado visual local mediante Signals/SignalStore.

### Solo Go debe

- Validar permisos de cada operación y recurso.
- Calcular/importar montos definitivos y reglas del contrato.
- Determinar porcentajes, beneficiarios y montos de **split payments**.
- Comunicarse con proveedores de pago desde un entorno confiable.
- Confirmar pagos mediante API/webhook verificado; asegurar idempotencia.
- Mantener conciliación, ledger, liquidaciones y auditoría financiera.
- Gestionar reglas de garantías, devoluciones y disputas.
- Preservar contratos, versiones y evidencia de firma.

### Contrato API

- Documento **OpenAPI** versionado como contrato técnico.
- Generar cliente y tipos TypeScript con una herramienta elegida durante el setup.
- Evitar duplicar manualmente DTOs y rutas.
- Definir convenciones uniformes para errores, paginación, fechas, moneda (CLP/UF), autenticación y versionado.
- Automatizar prueba de compatibilidad entre cambios Go y el cliente frontend.

## 9. Flujo de pagos y split payments

**Decisión comercial explícita:** la comisión se cobra por split payment en cada transacción soportada.

Flujo conceptual:

1. El usuario inicia el pago en Angular.
2. Angular solicita una intención de pago al backend Go.
3. Go valida autorización, obligación, comisión y destinatarios.
4. Go crea la operación con el proveedor de marketplace/split payments.
5. El usuario completa la operación en el mecanismo autorizado por el proveedor.
6. El proveedor informa el resultado; Go lo verifica y lo procesa idempotentemente.
7. Go concilia, registra movimientos contables y expone el estado confirmado.
8. Angular presenta el recibo y los estados resultantes.

**Precauciones:**

- No confiar en un retorno del navegador como comprobante definitivo.
- Separar renta, comisión, garantía, devolución y liquidación.
- Evitar dobles cobros/reembolsos con claves de idempotencia.
- Precisar cómo operarán devoluciones, pagos parciales, contracargos y liquidaciones.
- Verificar que el proveedor permita realmente la distribución requerida para Chile.
- Validar tratamiento legal y contractual de depósitos de garantía; split payment no equivale a custodia regulada.

## 10. Infraestructura y entorno local

```text
Navegador
   ↓
Angular (Docker → Cloud Run)
   ↓ HTTPS / API
Go (Docker → Cloud Run)
   ├── PostgreSQL (local Docker / Cloud SQL producción)
   ├── Cloud Storage (fotos y documentos)
   └── Proveedor de pagos (split payments)
```

Configuración local de referencia:

```text
Docker Compose
├── frontend-angular     :4200 (desarrollo) / puerto configurado (producción)
├── backend-go           :8080 (ejemplo)
└── postgres             :5432 (ejemplo)
```

Requisitos para aproximarse a Cloud Run:

- Contenedores reproducibles y separados.
- Escuchar en `0.0.0.0` y aceptar variable `PORT` en ejecución del servicio de producción.
- Manejar señales de terminación, límites de memoria/CPU y ausencia de disco persistente.
- Configuración por variables de entorno y secretos fuera del repositorio.
- Distinguir endpoint API accesible desde navegador y comunicación interna entre servicios.
- HTTPS, CORS (si dominios diferentes) y política de cookies/sesiones coherente.
- Pruebas locales de carga, simultaneidad, latencias, fallos y reintentos.

**No afirmar equivalencia perfecta:** Docker Compose no reproduce autoscaling, IAM ni la topología real de Cloud Run; es necesario probar además en un entorno Google Cloud de staging.

## 11. Calidad, pruebas y gobernanza del código

Aplicar desde el inicio:

- `strict` TypeScript y normas para evitar `any` injustificado.
- Angular ESLint y reglas de límites entre módulos.
- Stylelint para SCSS y convenciones de tokens.
- Unit tests para casos de uso de interfaz, servicios y funciones puras.
- Pruebas de integración de adaptadores HTTP con respuestas controladas.
- Pruebas end-to-end de los flujos principales (búsqueda, postulación, firma, inicio de pago, paneles).
- Accesibilidad (teclado, semántica HTML, etiquetas, contraste, foco y errores).
- Storybook o catálogo equivalente para componentes compartidos.
- CI con lint, typecheck, build, pruebas y validación del contrato OpenAPI.
- Convención de nombres, política de imports y documentación de decisiones arquitectónicas (ADR).

**Regla clave:** la arquitectura no se garantiza por las carpetas; se garantiza mediante **límites verificables**, pruebas y revisión de PR.

## 12. Ruta de implementación

### Etapa 0 — Preparación y contratos

- [ ] Revisar estado actual del frontend de pruebas en HTML + TypeScript.
- [ ] Inventariar endpoints existentes en Go y elaborar OpenAPI.
- [ ] Confirmar versión/parche de Angular, Node.js y TypeScript compatibles.
- [ ] Definir repositorio(s), ramas, convenciones de PR y CI.
- [ ] Definir rutas públicas, privadas, roles y autorizaciones por recurso.

**Salida:** inventario, contrato API inicial y decisiones técnicas fijadas.

### Etapa 1 — Bootstrap Angular

- [ ] Crear aplicación Angular standalone con SCSS.
- [ ] Incorporar Angular Router y plan de SSR por rutas.
- [ ] Crear estructura `core`, `features`, `shared`, `generated`, `styles`.
- [ ] Configurar `strict`, ESLint, Stylelint y pruebas básicas.
- [ ] Crear cliente TypeScript desde OpenAPI e interceptor de errores/autorización cuando aplique.

**Salida:** frontend base ejecutable y conectado a una API de prueba.

### Etapa 2 — Design System

- [ ] Definir tokens de color, tipografía, espaciado, radios, breakpoints y elevación.
- [ ] Definir temas claro/oscuro si se confirma su necesidad.
- [ ] Implementar componentes UI: botón, input, select, badge, modal, table, alert, skeleton, paginación.
- [ ] Crear componentes inmobiliarios reutilizables: PropertyCard, PriceDisplay, LocationFilter.
- [ ] Añadir documentación de variantes, accesibilidad y pruebas visuales.

**Salida:** lenguaje visual uniforme y biblioteca interna de componentes.

### Etapa 3 — Portal público y SEO

- [ ] Landing pública, listados, categorías/ubicaciones y ficha de propiedad.
- [ ] Filtros de búsqueda y sincronización con URL cuando sea conveniente.
- [ ] Integración con catálogo Go.
- [ ] Metadatos, canonicals, sitemap y estado de publicaciones.
- [ ] Validación de SSR con HTML realmente indexable.

**Salida:** catálogo público navegable, optimizado y medible.

### Etapa 4 — Identidad y paneles

- [ ] Login, recuperación de acceso, sesión y permisos.
- [ ] Paneles de arrendatario, propietario/corredor y administración.
- [ ] Flujos de propiedades, visitas y postulaciones.
- [ ] Signals para estados locales; evaluar SignalStore por flujo complejo.

**Salida:** operativa autenticada completa para búsquedas y postulaciones.

### Etapa 5 — Contratos y arriendos

- [ ] Proceso de aceptación y generación de contrato.
- [ ] Visualización de versiones y estados de firma.
- [ ] Calendario y obligaciones del arriendo.
- [ ] Controles de privacidad para documentos.

**Salida:** gestión del ciclo contractual integrada con Go.

### Etapa 6 — Finanzas y split payments

- [ ] Confirmar proveedor, onboarding de destinatarios, reglas y capacidades en Chile.
- [ ] Integrar creación de pagos, comprobantes y estados.
- [ ] Mostrar desglose de renta/comisión/garantía sin decidirlo en el navegador.
- [ ] Tratar errores, reintentos, pagos pendientes y devoluciones.
- [ ] Ejecutar pruebas de extremo a extremo en sandbox del proveedor.

**Salida:** frontend de transacciones respaldado por autoridad financiera de Go.

### Etapa 7 — Docker, Cloud Run y endurecimiento

- [ ] Dockerfile optimizado para SSR Angular en producción.
- [ ] Compose local frontend + Go + PostgreSQL.
- [ ] Pipeline de build y despliegue independiente.
- [ ] Staging Cloud Run, HTTPS, dominios, secretos y telemetría.
- [ ] Pruebas de carga, seguridad, accesibilidad y rendimiento.
- [ ] Plan de rollback, monitoreo y criterios de salida a producción.

**Salida:** plataforma lista para una puesta en producción controlada.

## 13. Pendientes de decisión (no inventar acuerdos)

Estos puntos **no se resolvieron definitivamente** durante la conversación:

1. **Marca y apariencia final:** nombre comercial, logotipo, colores, tipografía, tono y diseños definitivos.
2. **Proveedor de split payments:** compatibilidad real en Chile, tarifas, cuentas receptoras, onboarding/KYC, reversos, contracargos y liquidaciones.
3. **Mecanismo de firma de contratos:** proveedor, grado de firma, eventual intervención notarial, evidencias y requisitos jurídicos aplicables.
4. **Custodia/devolución de garantías:** quién recibe y mantiene los fondos, cuándo se liberan y cómo se resuelven disputas.
5. **Política de sesiones:** cookies, identidad, renovación de sesiones, MFA, CSRF y dominios.
6. **Motor de búsqueda:** comenzar con PostgreSQL/PostGIS y evaluar otros motores cuando las métricas lo justifiquen.
7. **Herramientas concretas de cliente OpenAPI, E2E, pruebas visuales y CI.**
8. **Estrategia exacta de SSR:** determinar ruta por ruta qué se prerenderiza, qué se renderiza dinámicamente y qué permanece CSR.
9. **Topología de repositorios:** mono-repositorio o repositorios separados, con despliegues independientes en cualquier caso.
10. **MVP y secuencia contractual/legal:** acordar qué flujos financieros se habilitan primero, sin exponer procesos incompletos a usuarios reales.

## 14. Criterios de aceptación para considerar listo el frontend base

- [ ] Proyecto Angular se compila en modo estricto y ejecuta tests/lint sin errores.
- [ ] Estructura Feature-Based creada, con límites de importación verificables.
- [ ] SCSS centralizado en tokens globales y estilos encapsulados por componentes.
- [ ] Componentes base accesibles y reutilizables disponibles.
- [ ] Cliente OpenAPI generado e integrado con al menos una API real de Go.
- [ ] Página de propiedad pública entrega HTML adecuado para indexación mediante SSR.
- [ ] Panel privado exige sesión y obtiene permisos verificados en el backend.
- [ ] Imágenes y documentos usan mecanismo correcto de acceso (público o autorizado).
- [ ] Angular y Go se ejecutan en contenedores separados en entorno de integración.
- [ ] La lógica de dinero/split payments no está duplicada en el frontend.
- [ ] Existe pipeline CI y un entorno staging para pruebas previas al despliegue.

## 15. Decisión final

**Adoptar Angular 22 + TypeScript + Standalone Components + Signals + SCSS + Angular CDK + Design System propio**, con arquitectura **Feature-Based** y **hexagonal ligera** en las funcionalidades que la requieran; **NgRx SignalStore solo bajo demanda**. Usar **Angular SSR híbrido** para el catálogo público y **CSR** para paneles privados. Integrar exclusivamente mediante **API Go documentada en OpenAPI**, ejecutando frontend y backend en **Docker/Cloud Run por separado**.

La prioridad es **consistencia arquitectónica, aislamiento de dominios, accesibilidad, SEO público, mantenibilidad y evolución gradual**, sin introducir complejidad de microfrontends, NgRx Store global o microservicios prematuramente.
