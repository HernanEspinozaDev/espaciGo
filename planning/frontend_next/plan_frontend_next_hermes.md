# Plan maestro — Frontend Next.js de EspaciGo

## 1. Identificación

- **Fecha de línea base:** 2026-10-24.
- **Rama exclusiva:** `frontend-next-hermes`.
- **Base:** `origin/main` en `99a1ee6ab2764b50a218b7042519c40e43684080`.
- **Cuenta autorizada para esta rama:** `HernanEspinozaDev`.
- **Flujo autorizado por el usuario:** commits y pushes directos a `frontend-next-hermes`; no se exige la separación histórica entre cuenta de desarrollo y cuenta revisora para este trabajo.
- **Límite de publicación:** no fusionar a `main`, crear PR, desplegar ni probar la aplicación online hasta autorización posterior expresa.
- **Entorno:** desarrollo, navegación y pruebas exclusivamente locales, con datos sintéticos.

### Decisión humana vigente — Fase 2R (2026-10-24)

La Fase 2 anterior y su demo fueron rechazadas por el usuario: su UI simplificada no cumplió la instrucción de integrar las vistas reales de las plantillas. La evidencia `fase-2-sistema-visual.md` queda marcada como rechazada y no aceptada. Esta decisión reemplaza, para la Fase 2R, toda regla previa que limitara las plantillas a inspiración o referencia visual: el código y las vistas reales de los commits congelados indicados abajo deben copiarse/adaptarse dentro de la aplicación Next.js única. No habilita Fase 3 ni publicación fuera de `frontend-next-hermes`.

Este documento mantiene el plan maestro y sus fases. La decisión de Fase 2R y su alcance vigente se registran arriba; su implementación, resultados y límites se documentan de forma incremental en `planning/frontend_next/evidence/fase-2r-integracion-plantillas.md`. Completar esta fase visual no autoriza ni inicia Fase 3.

## 2. Objetivo

Construir un único frontend Next.js para EspaciGo que combine dos lenguajes visuales:

1. **Marketplace público**, integrado desde las vistas y componentes reales de Your Next Store.
2. **Aplicación SaaS privada**, integrada desde las vistas y componentes reales de Next.js SaaS Starter.

Las plantillas son las bases de código y vistas reales para la integración visual de la Fase 2R; no son autoridades de dominio. Se integran selectivamente en la única aplicación bajo `frontend/`, sin aplicaciones independientes ni historiales Git anidados, y se sustituyen sus accesos de plataforma por fixtures tipados locales.

El resultado debe consumir exclusivamente la API Go existente, conservar las rutas y capacidades funcionales implementadas, respetar seguridad, privacidad, idempotencia y exactitud monetaria, y no introducir una segunda base de datos, autenticación paralela ni pagos Stripe.

## 3. Referencias congeladas

### 3.1 Your Next Store

- Repositorio: `https://github.com/yournextstore/yournextstore.git`.
- Commit inspeccionado: `1de63c614f4c57c1862b1b14710f2133784749c1`.
- Licencia: MIT.
- Uso previsto: marketplace, navegación pública, buscador, filtros, grillas, tarjetas, ficha, galería, estados de carga/error, SEO y patrones de rendimiento.
- No se conservarán Commerce Kit, YNS API, carrito, checkout, scripts de publicación, telemetría, chat AI, Bun ni integraciones propias de la plataforma YNS.

### 3.2 Next.js SaaS Starter

- Repositorio: `https://github.com/nextjs/saas-starter.git`.
- Commit inspeccionado: `6e33e58b1e553a41fe22e6b941a7229a002de361`.
- Licencia: MIT.
- Uso previsto: shell de dashboard, navegación lateral, encabezados, formularios, tarjetas, tablas, actividad, seguridad y patrones de administración.
- No se conservarán Drizzle, su esquema PostgreSQL, JWT propios, cookies de sesión, bcrypt, equipos SaaS, Stripe, billing, webhooks ni route handlers que dupliquen Go.

### 3.3 Versiones iniciales

Versiones estables verificadas al crear este plan:

- Next.js: `16.4.3`.
- pnpm: `12.15.0`.
- React: versión estable compatible fijada por el scaffold y los peer dependencies de Next.js.
- Node.js local observado: `26.12.1`.

La instalación inicial fijará `packageManager: pnpm@12.15.0`, lockfile `pnpm-lock.yaml` y una versión explícita de Node compatible. No se usarán dependencias canary.

## 4. Decisión arquitectónica

Se construirá **una sola aplicación Next.js** bajo `frontend/`, usando App Router y route groups:

```text
frontend/
├── app/
│   ├── (public)/          marketplace y contenido público
│   ├── (auth)/            acceso, registro y recuperación
│   ├── (account)/         cuenta, privacidad y verificaciones
│   ├── (host)/            gestión de espacios
│   ├── (bookings)/        reservas, operación y disputas
│   ├── (admin)/           administración operativa
│   ├── (global-admin)/    administración global
│   ├── api/backend/       gateway same-origin hacia Go
│   ├── layout.tsx
│   ├── error.tsx
│   ├── not-found.tsx
│   └── globals.css
├── components/
│   ├── brand/
│   ├── marketplace/
│   ├── dashboard/
│   ├── forms/
│   ├── feedback/
│   └── ui/
├── features/
│   └── <feature>/{application,domain,infrastructure,ui}/
├── lib/
│   ├── api/
│   ├── auth/
│   ├── idempotency/
│   ├── money/
│   ├── time/
│   ├── files/
│   └── runtime-config/
├── generated/api/
├── public/
├── tests/
└── tooling/
```

No se mantendrán dos aplicaciones Next independientes porque eso duplicaría autenticación, SDK, tokens, estilos, rutas, build, Docker y pruebas. Los dos orígenes visuales se integrarán mediante dos layouts coordinados sobre un mismo sistema de diseño.

## 5. Política para las plantillas

Los repositorios completos congelados se conservan únicamente como fuentes ignoradas en:

```text
.local/frontend-template-sources/yournextstore/
.local/frontend-template-sources/nextjs-saas-starter/
```

`.local/` ya está excluido de Git. La Fase 2R integra vistas reales: Your Next Store es la base del marketplace y Next.js SaaS Starter aporta las vistas del workspace. Antes de copiar código se registrarán:

- URL;
- commit exacto;
- licencia;
- archivo o componente de origen;
- adaptación realizada.

Se añadirá un `THIRD_PARTY_NOTICES.md` cuando se incorpore código reconocible. No se copiarán los directorios `.git`, lockfiles, secretos, configuraciones de despliegue ni AGENTS de las plantillas.

## 6. Activos heredados permitidos

Del frontend Angular eliminado solo se rescatarán:

- `frontend/public/espacigo-mark.svg` desde el commit base;
- favicon generado o servido a partir de esa misma marca.

No se restaurarán componentes, estilos, páginas, lógica Angular, SDK generado ni dependencias del frontend anterior.

La marca se validará para:

- favicon;
- icono Apple/PWA si se decide habilitar manifest;
- logotipo del encabezado público;
- logotipo compacto del dashboard;
- metadatos Open Graph locales.

## 7. Contrato con Go

### 7.1 Fuente de verdad

- API: `planning/openapi.yaml`.
- Base funcional: `/api/v1`.
- Go conserva autoridad sobre sesión, roles, capacidades, propiedad, elegibilidad, precios, disponibilidad, estados y transiciones.
- El frontend no tendrá acceso directo a PostgreSQL.
- No se introducirán Server Actions que creen reglas o persistencia paralela.

### 7.2 SDK

Se generará un cliente TypeScript desde `planning/openapi.yaml`. Los archivos generados:

- vivirán en `frontend/generated/api/`;
- no se editarán manualmente;
- se regenerarán de forma determinista;
- tendrán un gate que falle si la regeneración deja diferencias.

Las páginas y componentes no importarán directamente transporte HTTP ni tipos generados. El flujo será:

```text
UI → caso de uso/fachada → puerto → adaptador → cliente generado → gateway → Go
```

### 7.3 Gateway same-origin

`app/api/backend/[...path]/route.ts` será un gateway limitado hacia Go:

- allowlist de paths y métodos basada en OpenAPI;
- límites de URL y body;
- eliminación de hop-by-hop headers;
- propagación segura de `Authorization`, `Content-Type`, `Accept`, `Idempotency-Key` y `X-Request-ID`;
- sin registrar Bearer, cuerpos sensibles o secretos;
- sin cachear peticiones privadas;
- soporte separado para JSON, streaming limitado y binarios;
- URL interna de Go disponible solo en servidor.

El navegador usará el gateway same-origin. Los Server Components públicos podrán leer Go desde el servidor usando la URL interna y sin credenciales privadas.

### 7.4 Sesión

La sesión existente utiliza Bearer opaco. Se mantendrán estas reglas:

- token exclusivamente en memoria;
- prohibido `localStorage`, `sessionStorage`, IndexedDB, cookies propias, URL, HTML SSR y logs;
- no inventar refresh token;
- confirmar login con `/auth/session`;
- polling de sesión no se considera actividad;
- expirar únicamente la generación de sesión que originó un `401` tardío;
- logout limpia el estado local aunque falle la revocación remota;
- `restricted_mode`, roles y capacidades provienen del servidor.

Las áreas privadas serán principalmente CSR después de hidratar el shell. No se enviará Bearer desde SSR ni se compartirá sesión entre solicitudes.

### 7.5 Idempotencia y concurrencia

Se implementará un registro de intentos por operación:

- una clave por intención del usuario;
- misma clave y mismo payload durante reintentos;
- doble clic bloqueado sin perder la clave;
- timeout ambiguo produce estado pendiente/conciliación, no una segunda operación;
- aislamiento por sesión, recurso y revisión;
- descarte de respuestas tardías que ya no correspondan al contexto visible.

### 7.6 Dinero, fechas y archivos

- Montos CLP exactos, sin cálculos con `float`.
- Campos `int64` potencialmente fuera del rango seguro se preservarán como string o mediante parser exacto.
- Go decide cotizaciones, vencimientos y disponibilidad.
- Fechas RFC 3339 y zonas IANA; pruebas específicas para `America/Santiago` y DST.
- Descargas privadas mediante Blob URL temporal y revocación al cambiar sesión/recurso o desmontar.
- Fotos públicas solo desde endpoints públicos autorizados.

## 8. Mapa de rutas objetivo

### 8.1 Públicas

- `/`
- `/categorias`
- `/como-funciona`
- `/explorar`
- `/espacios/[id]`
- `/terminos-y-condiciones`
- `/politica-de-privacidad`

### 8.2 Identidad

- `/acceso`
- `/registro`
- `/verificar-correo`
- `/recuperar-acceso`

### 8.3 Cuenta

- `/mi-cuenta`
- `/mi-cuenta/privacidad`
- `/mi-cuenta/seguridad`
- `/mi-cuenta/verificaciones`

### 8.4 Anfitrión

- `/panel-anfitrion`, alias de `/panel/espacios`
- `/panel/espacios`
- `/panel/espacios/[spaceId]/editar`
- `/panel/espacios/[spaceId]/tarifas`
- `/panel/espacios/[spaceId]/calendario`
- `/panel/espacios/[spaceId]/galeria`

### 8.5 Reservas

- `/reservas`
- `/reservas/[id]`
- `/reservas/[id]/disputas`

### 8.6 Administración

- `/administracion`
- `/administracion/disputas`
- `/administracion/global`

No se crearán rutas visibles para capacidades aspiracionales sin contrato Go implementado.

## 9. Sistema visual

Se integrarán los árboles de vistas reales seleccionados de ambas plantillas en una sola aplicación. Se conservarán por separado sus composiciones y densidades; solo se compartirán los tokens y utilidades compatibles. No se reemplazarán con una UI simplificada nueva.

### Marketplace

Tomará de Your Next Store:

- encabezado público;
- hero y buscador;
- navegación de categorías;
- tarjetas responsivas;
- filtros y orden;
- skeletons;
- paginación/cursor;
- ficha con galería;
- secciones informativas y SEO.

Los conceptos se traducirán así:

- producto → espacio;
- variante → modalidad/tarifa publicada;
- categoría de tienda → categoría de espacio;
- stock → disponibilidad consultada;
- precio de producto → tarifa o estimación proporcionada por Go;
- carrito/checkout → cotización y flujo autenticado de reserva, sin carrito comercial.

### SaaS privado

Tomará de Next.js SaaS Starter:

- shell lateral;
- navegación contextual;
- encabezado de página;
- cards métricas;
- tablas y filtros;
- formularios;
- timeline/actividad;
- menús responsivos;
- patrones de cuenta y seguridad.

Se eliminarán los conceptos de team SaaS, suscripción y Stripe. La navegación se generará desde sesión, roles, capacidades y `restricted_mode`.

### Tokens

Se definirán tokens para:

- marca;
- fondos y superficies;
- texto y bordes;
- estados semánticos;
- foco;
- spacing;
- radios;
- elevación;
- tipografía;
- densidad pública y densidad administrativa.

No se copiarán paletas completas sin contrastarlas con WCAG AA. El marketplace podrá ser más editorial y el SaaS más denso, conservando marca y componentes base compartidos.

## 10. Calidad y herramientas

Scripts mínimos previstos:

```text
pnpm dev
pnpm build
pnpm start
pnpm lint
pnpm format:check
pnpm typecheck
pnpm test
pnpm test:e2e
pnpm test:a11y
pnpm api:generate
pnpm api:check
pnpm check
```

Herramientas previstas:

- TypeScript estricto;
- ESLint con configuración Next;
- Prettier o Biome, eligiendo una sola autoridad de formato;
- Vitest;
- React Testing Library;
- Playwright;
- axe-core;
- validación de OpenAPI generado;
- auditoría de dependencias pnpm.

No se aceptará una configuración donde lint escriba cambios durante CI. Los comandos de comprobación serán reproducibles y no mutantes.

## 11. Pruebas obligatorias

### Unitarias

- sesión y generaciones;
- guards/políticas de navegación;
- error envelope;
- dinero exacto;
- fechas y DST;
- cursor opaco;
- idempotencia;
- revocación de Blob URLs;
- mappers request/response;
- categorías y perfiles dinámicos.

### Componentes

- loading, empty, error, success, denied, restricted y offline;
- formularios con teclado;
- tablas responsivas;
- filtros públicos;
- galería;
- diálogos sensibles;
- navegación lateral móvil.

### Integración

- gateway hacia Go local;
- JSON estricto;
- binarios;
- CORS/same-origin;
- `X-Request-ID` y `Retry-After`;
- expiración y revocación de sesión;
- respuestas fuera de orden;
- 401/403/404/409/422/428/429.

### E2E local

Cobertura por actor:

- anónimo;
- arrendatario;
- anfitrión;
- administrador;
- administrador global;
- tercero sin acceso;
- cuenta bloqueada;
- cuenta restringida.

Viewports mínimos:

- `390 × 844`;
- `1365 × 900`.

Los ensayos usarán únicamente URL y puerto locales comprobados, stack aislado y cuentas sintéticas autorizadas por la política local aplicable.

## 12. Accesibilidad, SEO y rendimiento

### Accesibilidad

- WCAG 2.2 AA como objetivo verificable, no como certificación.
- AXE sin violaciones conocidas en rutas principales.
- foco visible;
- skip link;
- landmarks;
- labels persistentes;
- errores vinculados;
- navegación completa por teclado;
- touch targets adecuados;
- movimiento reducido;
- no depender solo del color.

### SEO público

- metadata por ruta;
- canonical confiable;
- Open Graph;
- Twitter cards si procede;
- JSON-LD adaptado a espacios y organización sin inventar datos legales;
- sitemap/robots únicamente cuando el alcance local esté validado;
- páginas privadas `noindex` y `no-store`.

### Rendimiento

- Server Components por defecto en público;
- Client Components solo donde haya interacción o sesión;
- una sola imagen LCP prioritaria;
- tamaños explícitos y `sizes` correctos;
- skeletons estables;
- sin waterfalls evitables;
- presupuestos de bundle definidos tras la línea base;
- auditorías Lighthouse locales como tendencia, no como prueba única.

## 13. Integración local y Docker

El frontend nuevo sustituirá el servicio Angular de `compose.yaml` en una fase posterior. La adaptación debe:

- usar una imagen multi-stage con pnpm y output standalone;
- ejecutar como usuario no privilegiado;
- exponer únicamente el puerto configurado;
- conectarse a la red API, nunca a la red de datos;
- no recibir secretos de DB, SMTP, HMAC o cifrado;
- tener healthcheck propio;
- publicar solo configuración no sensible;
- mantener backend y PostgreSQL sin cambios funcionales.

Hasta que el servicio Next esté disponible, las pruebas frontend usarán un stack local específico y puertos nuevos. No se tocarán volúmenes persistentes ni stacks retenidos.

## 14. Fases de ejecución

### Fase 0 — Gobierno y línea base

Entregables:

- rama `frontend-next-hermes`;
- este plan;
- registro de commits y licencias de plantillas;
- instrucción explícita de trabajo local y push directo autorizado.

Gate:

- rama remota verificada;
- `main` sin modificaciones producidas por este trabajo;
- plan revisable desde GitHub.

### Fase 1 — Scaffold Next.js limpio

Entregables:

- reemplazo controlado de `frontend/` por scaffold Next.js estable;
- pnpm y Node fijados;
- TypeScript estricto;
- lint, formato, tests y build mínimos;
- Dockerfile inicial;
- plantilla clonada solo bajo `.local/`.

Gate:

- `pnpm install --frozen-lockfile`;
- lint, typecheck, test y build verdes;
- aplicación local muestra shell neutro con marca EspaciGo.

### Fase 2 — Sistema de diseño y dos shells

El resultado previo de Fase 2 fue rechazado por el usuario y no constituye un gate aceptado. Fase 2R lo reemplaza: integrar el código y las vistas reales de las dos fuentes congeladas, adaptar solo imports, routing, stack fijado, marca EspaciGo y límites de dominio; preservar layouts, CSS, jerarquía, navegación y densidad. La política anterior de “referencia/inspiración solamente” ya no aplica a este corte.

Entregables:

- tokens;
- componentes base;
- shell público;
- shell SaaS;
- navegación móvil/escritorio;
- estados UI comunes;
- marca y favicon.

Gate:

- Story/demo local de componentes;
- contraste y teclado verificados;
- ambos viewports sin overflow.

### Fase 3 — Contrato API, gateway y sesión

Entregables:

- generación OpenAPI;
- gateway allowlisted;
- runtime config;
- cliente de errores;
- sesión Bearer en memoria;
- roles, capacidades y restricted mode;
- idempotencia, exact JSON, tiempo y archivos.

Gate:

- pruebas contractuales y de seguridad;
- ningún token en storage/cookies/logs;
- respuestas tardías aisladas;
- binarios y errores tratados correctamente.

Registro final del corte local del 2026-10-24; implementación publicada en `74f166de60a1acded0d8cb8ed14b3cf07667ccf5`: tras autorización humana expresa se recuperó una cuenta sintética existente mediante el flujo normal de Go (solicitud 202, finalización 204); no se creó cuenta. El login anterior a la recuperación había fallado por discrepancia de credencial, no por un defecto demostrado del frontend. Se corrigió el same-origin del gateway para comparar Origin con Host saneado, se rechazó el origen divergente (403), se ignoraron `x-forwarded-*` y se trató correctamente el stream vacío del logout bodyless. Login, sesión autenticada, roles/capabilities/`restricted_mode`, privacidad del Bearer, Axe anónimo/autenticado en 1280×900 y 390×844, logout 204 y sesión posterior 401 pasaron en E2E contra API nativa y PostgreSQL nativo. `pnpm install --frozen-lockfile`, generación/check OpenAPI, lint, typecheck, unitarias, formato, build, `pnpm check` y E2E nativo pasaron; la solicitud a términos sigue omitida por 503 `terms_unavailable`. La matriz conserva 50 operaciones directas, 100 por familia/prefijo y 29 no verificadas; 179 montajes quedan condicionales dentro de `if prototypeEnabled`, no son 179 rutas exactas confirmadas. Una sesión creada por la primera corrida autenticada no pudo revocarse porque el gateway rechazó su logout antes de Go; su Bearer no se guardó y la sesión puede expirar naturalmente dentro del TTL normal de hasta 8 h. La corrida final sí revocó su propio token. Fase 3 queda verificada y publicada directamente en `frontend-next-hermes`, sin Fase 4. Evidencia: `planning/frontend_next/evidence/fase-3-contrato-gateway-sesion.md`.

### Fase 4 — Marketplace público

Entregables:

- portada;
- categorías;
- cómo funciona;
- explorar;
- filtros tipados;
- cursor y orden;
- ficha;
- galería pública;
- disponibilidad/cotización informativa;
- términos y privacidad.

Gate:

- SSR/RSC público contra Go local;
- proyección minimizada;
- SEO y accesibilidad;
- navegación anónima completa.

Estado del corte local (2026-10-25): implementación frontend y pruebas Fase 4 pasaron contra el API Go compatible histórico `5995a5fcdbd428220cd8927499d1d33f84466980`, usando la misma base nativa sin modificarla. La validación con el API Go de HEAD no es completa: categorías/ubicaciones responden 200, pero el listado responde 500 por la relación `public.space_media` ausente (SQLSTATE `42P01`) en el esquema existente. Esta limitación de compatibilidad backend/esquema queda abierta, sin cambios a Go o PostgreSQL, y no se atribuye a un fallo del frontend. La respuesta legal compatible devuelve metadatos sintéticos sin contenido; la UI no muestra texto legal sustituto. Evidencia de niveles, resultados, pruebas y límites: `planning/frontend_next/evidence/fase-4-marketplace-publico.md`. Fase 5 no iniciada.

### Fase 5 — Identidad y onboarding

Entregables:

- acceso;
- registro con versiones de documentos;
- verificación de correo;
- recuperación;
- retorno contextual;
- bloqueo y rate-limit UX.

Gate:

- tokens de acción tratados de forma segura;
- login confirmado por sesión;
- no enumeración de cuenta;
- 401/429 y expiración cubiertos.

### Fase 6 — Cuenta y verificaciones

Entregables:

- perfil;
- foto;
- cuenta de cobro sintética;
- privacidad;
- exportaciones;
- seguridad;
- elegibilidad y KYC/KYB sintético.

Gate:

- descargas privadas y Blob cleanup;
- avisos sintéticos visibles;
- autorización por recurso y estados completos.

### Fase 7 — Panel anfitrión

Entregables:

- lista y creación de borrador;
- editor por perfil versionado;
- publicación/contenido;
- tarifas e historial;
- calendario, zona y horario semanal;
- galería y portada.

Gate:

- sin lista manual autoritativa de categorías;
- `attribute_schema_version` preservado;
- importes/fechas validados por Go;
- conflictos e idempotencia cubiertos.

### Fase 8 — Reservas y operación

Entregables:

- bandeja y detalle;
- cotización/reserva;
- pago fake;
- decisión anfitrión;
- cancelación/devolución;
- contrato;
- check-in/out/recepción;
- conversación;
- reseñas;
- disputas, daños, garantía y liquidación fake.

Gate:

- snapshots históricos respetados;
- timeouts ambiguos conciliados;
- etiquetas de ensayo permanentes;
- todos los actores y transiciones cubiertos.

### Fase 9 — Administración operativa y global

Entregables:

- KYC;
- privacidad;
- reseñas/reportes;
- avisos;
- cuentas/roles;
- publicaciones;
- reservas;
- auditoría;
- reportes;
- cuentas y publicaciones globales;
- KPI y exportaciones.

Gate:

- navegación por capacidades;
- recent authentication y `428`;
- `expected_updated_at` y `409`;
- limpieza de datos privados tras revocación/403.

### Fase 10 — Integración Docker y QA integral

Entregables:

- servicio Next en Compose;
- scripts locales actualizados;
- Playwright integral;
- accessibility suite;
- pruebas de red/secretos;
- documentación de arranque.

Gate:

- stack aislado reproducible;
- ningún acceso del frontend a red de datos;
- healthchecks verdes;
- suite completa sobre Go local real.

### Fase 11 — Consolidación y preparación de revisión

Entregables:

- eliminación de código de plantilla no utilizado;
- inventario de atribuciones;
- documentación final;
- matriz ruta/capacidad/prueba;
- evidencia local por fase;
- lista explícita de límites pendientes.

Gate:

- árbol limpio;
- gates verdes;
- sin secretos ni datos reales;
- sin push a `main`, PR, merge o despliegue hasta nueva autorización.

## 15. Estrategia de commits y seguimiento

Cada fase tendrá commits pequeños y coherentes, por ejemplo:

```text
chore(frontend): scaffold Next.js with pnpm
feat(frontend): add shared design tokens and shells
feat(frontend): generate Go API client
feat(frontend): add in-memory identity session
feat(frontend): build public marketplace search
```

Después de cada corte se registrará en este documento o en `planning/frontend_next/evidence/`:

- HEAD completo;
- rutas afectadas;
- pruebas ejecutadas;
- resultado;
- limitaciones;
- siguiente corte.

No se afirmará integración completa basándose solo en mocks o screenshots.

## 16. Riesgos conocidos

1. **Las plantillas no son compatibles entre sí de forma directa.** Fase 2R conserva sus vistas en shells separados dentro de una aplicación y adapta imports, tokens y puertos tipados, sin duplicar runtimes.
2. **Your Next Store usa Bun, Commerce Kit y reglas específicas de plataforma.** Deben eliminarse antes de considerar código reusable.
3. **SaaS Starter implementa su propia DB, JWT y Stripe.** Todo eso contradice la autoridad de Go y debe descartarse.
4. **Next.js 16 evoluciona rápidamente.** Cada uso de caché, proxy, Server Components y routing se verificará contra la documentación instalada de la versión fijada.
5. **OpenAPI y handlers tienen pequeñas inconsistencias históricas.** El cliente tendrá fallback seguro y pruebas contractuales.
6. **Bearer en memoria limita SSR privado.** Las páginas privadas se diseñarán como shell + CSR autenticado, sin inventar cookies.
7. **Hay una diferencia histórica entre ocho categorías y la categoría `inmuebles` de V67.** El catálogo del servidor será la autoridad.
8. **Operaciones financieras y legales son sintéticas.** La UI debe evitar lenguaje de capacidad real.
9. **El frontend anterior tenía gran amplitud funcional.** La aceptación requiere todas las rutas y pestañas, no solo marketplace y dashboard iniciales.
10. **La rama parte de un commit que todavía contiene Angular.** La sustitución se realizará en un commit explícito, sin restaurar Angular en el checkout principal del usuario.

## 17. Definición de terminado

El nuevo frontend se considerará técnicamente listo para revisión local cuando:

- todas las rutas del mapa estén implementadas o marcadas explícitamente como fuera de alcance por falta de contrato;
- consuma Go mediante OpenAPI y gateway;
- no duplique autenticación, DB, pagos o reglas;
- use pnpm y Next.js estable fijados;
- build, lint, typecheck, unitarias, integración y E2E estén verdes;
- AXE y teclado cubran las rutas principales;
- marketplace y SaaS sean visualmente coherentes;
- roles, capacidades y restricted mode funcionen;
- dinero, tiempo, idempotencia y archivos respeten el contrato;
- Docker local esté aislado;
- no haya secretos, datos reales o claims productivos falsos;
- el usuario haya realizado su revisión local/online reservada y autorice el siguiente paso.

La fusión a `main` es una decisión posterior y no forma parte de este plan inicial.
