# Plan UI/UX — marca, acceso, sesión y Mi cuenta

**Estado corregido tras revisión del usuario:** la maqueta original sigue **rechazada visualmente**. La corrección de paleta ya está implementada y verificada **sobre Next real**, manteniendo Tailwind 4 y el diseño de `main`: 12 escenarios, 922 elementos antes/después, cero diferencias geométricas y cero violaciones Axe; 59 unitarias aprobadas y suite E2E 64 PASS / 45 SKIP (sin integración nativa Go). Evidencia: `evidence/ui-ux-fase1-correccion-paleta-main.md`. Revisión independiente cerrada, sin bloqueantes para modo claro; fase correctiva cerrada técnicamente. El modo oscuro no está certificado. No confundir pruebas técnicas con aceptación visual del usuario. Solo cambian colores, incluidos ajustes de contraste rojo/verde que conservan su semántica; ningún componente o layout JSX se modificó. La persistencia HttpOnly/Secure conservó su aprobación durante ese corte. El cierre posterior de fase 2 se registra más abajo; push y despliegue se acreditan por separado.
**Base inspeccionada:** `main` `8a87e38d1b27c384d054ff4bf463c46bdc49c9d2`.
**Objetivo:** una experiencia continua y reconocible de EspaciGo: marca terracota, acceso con logo real, navegación consciente de sesión, nombre/ID visible, cierre de sesión accesible y retorno al sitio sin pedir credenciales mientras la sesión Go siga vigente.

## 1. Hallazgos comprobados

| Problema                                                         | Evidencia en el checkout actual                                                                                    | Consecuencia                                                                                               |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| El acceso no usa el logo real                                    | `frontend/src/features/auth/ui/auth-shell.tsx:32` usa `CircleIcon`; ya existe `frontend/public/espacigo-mark.svg`  | La entrada parece una plantilla distinta de EspaciGo                                                       |
| El panel sigue ofreciendo «Acceso»                               | `frontend/src/app/(workspace)/layout.tsx:29–31` contiene un enlace estático a `/acceso`                            | Una persona autenticada recibe una acción incoherente                                                      |
| La cabecera pública siempre ofrece ingresar                      | `frontend/src/app/(marketplace)/layout.tsx:39–45` no consume estado de sesión                                      | Volver al marketplace no presenta «Mi cuenta»                                                              |
| El nombre no forma parte de la identidad de la cabecera          | `AuthSnapshot` contiene `accountId`, pero no perfil; `AccountSnapshot` sí contiene `profile`                       | Falta coordinar sesión y nombre confirmado por Go                                                          |
| La implementación se desvió de las reglas de sesión documentadas | `auth-facade.ts:53–103` persiste `accessToken` y sesión en `sessionStorage`; la pantalla dice «solo en la memoria» | El copy no describe la realidad y Web Storage no es la solución propuesta para persistencia segura         |
| La restauración confía inicialmente en la sesión guardada        | Constructor de `AuthFacade`, antes de `checkSession()`                                                             | Hay que mostrar «comprobando» hasta validar con Go; no dar por confirmados roles guardados en el navegador |
| La duración no es indefinida                                     | `internal/identity/auth_service.go:725` crea la sesión con ocho horas; login expone `expires_at`                   | Volver al sitio debe respetar esa caducidad, no inventar refresh ni ampliar la duración desde Next         |
| Colores actuales fragmentados                                    | `globals.css` usa primarios neutros; `workspace-template.css` conserva colores de starter y acceso usa `orange-*`  | Hace falta una capa de tokens compartida, no cambiar colores componente por componente                     |

Las referencias son del SHA base. No se interpretan pantallas 200 como prueba de una sesión autenticada. No se usarán cuentas reales ni el navegador del usuario para diseñar o probar.

## 2. Marca recuperada: terracota cálido

Fuentes verificables: `/home/nandev/Documentos/projects/Proyectotitulo/espaciGo/frontend/src/styles/_tokens.scss:15–36` y las guías ya presentes en el checkout actual: `planning/frontend/referencias/guia_diseno_original.md:4–9,26,32–57` y `planning/frontend/diseno_scss_angular.md:3`. Las guías documentan que el usuario eligió **Terracota + Crema**, con dirección **Modern Warm Minimal**, y marcan cinco colores de marca como aprobados; los neutros y variantes complementarias figuraban como propuesta normativa y después aparecen en los tokens implementados.

Se recuperan **valores, roles de color y dirección visual**, no componentes Angular, hojas SCSS completas, reglas antiguas ni lógica de autenticación. La aprobación histórica de la marca no equivale a aprobación de todo este rediseño Next. Tampoco se trasladan las antiguas restricciones «Angular + SCSS / sin Tailwind» ni una taxonomía de producto antigua: se mantienen Next.js, Tailwind y el alcance actual de espacios flexibles y arriendos residenciales.

| Token propuesto Next  | Valor recuperado | Uso                                                         |
| --------------------- | ---------------- | ----------------------------------------------------------- |
| `brand.primary`       | `#A44731`        | Acción principal, selección y marca                         |
| `brand.primaryHover`  | `#843522`        | Hover del botón principal                                   |
| `brand.primaryActive` | `#702A1B`        | Estado presionado                                           |
| `brand.primarySoft`   | `#F5E3DB`        | Fondo de selección y superficies de marca                   |
| `brand.secondary`     | `#8A6A4D`        | Detalles secundarios cálidos                                |
| `brand.accent`        | `#D97745`        | Acento decorativo, no sustituto indiscriminado del primario |
| `surface.canvas`      | `#FBF7F2`        | Fondo general marfil cálido                                 |
| `surface.card`        | `#FFFFFF`        | Formularios y tarjetas                                      |
| `surface.subtle`      | `#F5EEE6`        | Bloques secundarios                                         |
| `text.primary`        | `#332720`        | Texto principal                                             |
| `text.muted`          | `#66574E`        | Ayuda y descripciones                                       |
| `border.default`      | `#DED5CC`        | Separadores                                                 |
| `border.control`      | `#8A796B`        | Bordes de controles cuando sea necesario para contraste     |
| `focus.ring`          | `#1D4ED8`        | Foco de teclado visible, no confundir foco con selección    |

Estados semánticos históricos: éxito `#15803D`, advertencia `#B45309`, error `#B91C1C`, información `#1D4ED8`. El terracota es marca, no el color universal de todos los estados.

**Contraste calculado mediante luminancia sRGB, no inspección visual:** blanco/primario **5,96:1**; blanco/hover **8,36:1**; texto principal/marfil **13,56:1**; texto secundario/marfil **6,48:1**. Blanco/acento `#D97745` da **3,15:1**: no usar esa combinación para texto normal de botones. Estos cálculos no sustituyen Axe ni la comprobación de estados reales.

### Dirección visual

**Corrección vinculante del usuario:** mantener el diseño Next de `main` tal como está y normalizar la paleta. Las propuestas siguientes de nuevos radios, espaciados, superficies y composición solo se conservan como historia; no autorizan cambiar geometría o reconstruir vistas. Los valores no cromáticos de `DESIGN.md` y sus exports no sustituyen los estilos actuales. Cualquier ajuste de accesibilidad ajeno a color debe justificarse y acotarse por separado. La comparación visual debe hacerse sobre las mismas vistas Next antes/después, no contra la maqueta rechazada.

- Conservar el código/composición de las plantillas Next ya integradas; no reconstruir otra aplicación.
- Una sola identidad de marca para marketplace, acceso y workspace; densidad distinta según contexto.
- Logo real y palabra EspaciGo mediante un componente `Brand` reutilizable con tamaños controlados.
- Fondo cálido, tarjetas blancas, bordes suaves y sombras discretas. Reservar terracota para acciones y selección.
- Mantener inicialmente la tipografía local existente: no añadir fuentes remotas ni tracking de terceros.
- Escala espacial sobre múltiplos de 4/8 px; controles con objetivo táctil de al menos 44 px; radios consistentes de 10–16 px en tarjetas, sin convertir todas las piezas en cápsulas.
- No recolorear indiscriminadamente fotografías, gráficos o estados de validación.
- Fijar los tokens en una especificación versionada antes de extenderlos a todas las features. Revisar los overrides `.saas-application` para que no anulen la marca de forma accidental.

## 3. Comportamiento acordado como objetivo de UX

### Cabecera pública

| Estado confirmado   | Identidad/acción                                                                                                            |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Comprobando sesión  | Indicador discreto o placeholder estable; no mostrar primero «Ingresar» y cambiarlo después por un perfil ajeno/no validado |
| Invitado            | **Ingresar a EspaciGo** → `/acceso`                                                                                         |
| Autenticado         | **Mi cuenta** → `/mi-cuenta`, directamente; no pasar por `/acceso`                                                          |
| Expirado o revocado | Volver al estado invitado; explicar el vencimiento si se intentó una acción privada                                         |

El logo siempre vuelve al inicio. Las áreas públicas continúan siendo públicas; no redirigir automáticamente todo el marketplace al panel por existir sesión.

### Cabecera del workspace / Mi cuenta

```text
[Logo EspaciGo → inicio]                 [Nombre o ID] [Cerrar sesión]
```

- Quitar el enlace «Acceso» de la experiencia autenticada.
- Mostrar `profile.displayName.trim()` cuando Go haya devuelto un nombre válido.
- Sin perfil, nombre vacío o perfil aún no configurado: mostrar el **ID de cuenta confirmado por Go**, no el correo ni un nombre inventado. En móvil permitir ajuste de línea; cualquier truncado visual debe conservar el ID completo accesible.
- Después de guardar el nombre correctamente, actualizar cabecera y saludo sin recargar, usando el perfil normalizado que devuelve Go.
- Un `404` de perfil inicial significa «perfil por completar», no fallo de autenticación.
- No consultar pagos, ARCO o fotos privadas solo para pintar el nombre en la cabecera. Usar una consulta mínima de identidad/perfil.
- No mostrar «Cerrar sesión» a invitados. Bloquear el doble envío mientras se procesa.
- Descartar respuestas tardías al cerrar sesión o cambiar de cuenta: nunca mostrar el nombre de la persona anterior.

### Acceso

- Sustituir el círculo por el logo real; marca visible con dimensiones y contraste correctos.
- En escritorio: formulario de anchura contenida y zona de marca opcional, sin fotos decorativas obligatorias. En móvil: una columna, marca y formulario visibles sin desplazamientos innecesarios.
- Jerarquía: marca → «Accede a EspaciGo» → breve explicación → correo/contraseña → recuperación → botón principal → registro secundario.
- Mantener autocompletado, control de mostrar contraseña, foco de error y mensajes seguros existentes.
- Con sesión confirmada: no obligar a iniciar sesión otra vez. Resolver el retorno interno válido; si no existe, entrar a **Mi cuenta**. El panel administrativo se mantiene accesible según capacidades de Go, sin perder un retorno explícito a una ruta administrativa permitida.
- El cierre de sesión estará en las cabeceras, por lo que ya no es necesario conservar `/acceso` como única pantalla para salir.
- Cambiar de cuenta será una acción explícita tras cerrar la sesión actual; no presentar simultáneamente «sesión activa» y un segundo formulario que pueda reemplazarla por accidente.
- Al modificar este comportamiento hay que actualizar los helpers E2E que hoy esperan el formulario de acceso incluso estando autenticados. No mantener una UX contradictoria únicamente por esos selectores antiguos.

### Mi cuenta

- Conservar y pulir el sidebar responsive ya implementado: General, Seguridad, Privacidad y Verificaciones; no añadir opciones sin capacidad real.
- Activo: terracota sobre fondo suave, icono y `aria-current`; no depender solo del color.
- Saludo con nombre o ID, estado de perfil y una acción clara para completarlo cuando falte nombre.
- Formulario de perfil con etiquetas, estados de guardado/error y confirmación accesible. Mantener datos sintéticos y límites existentes donde corresponda.
- No mostrar dashboards, métricas, fotos, pagos o verificaciones ficticias como datos reales.

## 4. Decisión técnica necesaria: sesión persistente segura

**Decisión aprobada por el usuario:** «si apruebo la persistencia, comienza con el plan de ejecucion». Se autoriza pasar del token expuesto al JavaScript cliente/Web Storage a una cookie **HttpOnly + Secure + SameSite**, gestionada por los route handlers de Next como BFF. Go sigue emitiendo, validando, autorizando, venciendo y revocando la sesión opaca. No incorporar NextAuth, JWT propio, refresh inventado, ORM ni una base de sesiones paralela.

Esto es un cambio deliberado respecto de la antigua regla «Bearer solo en memoria / sin cookies propias»; debe quedar documentado en una decisión de arquitectura aprobada. No se aplicará silenciosamente por considerarlo un detalle visual.

### Flujo propuesto

1. El navegador envía el login al route handler same-origin de Next.
2. Next llama al login Go, confirma `GET /auth/session` y recibe el token en servidor.
3. Next entrega una cookie opaca HttpOnly, sin token en JSON/HTML/props accesibles al cliente. La duración máxima deriva de `expires_at` de Go; no excederla ni renovarla ficticiamente.
4. El cliente conserva solo estado de presentación y datos mínimos de sesión confirmada. Las operaciones autenticadas usan el gateway same-origin; Next obtiene la credencial desde la cookie y la transmite a Go.
5. Al recargar, abrir otra pestaña o regresar al mismo sitio, consultar la sesión con respuesta no cacheable antes de pintar identidad privada. Debe funcionar en cualquier instancia Cloud Run, sin depender de memoria de un único proceso Next.
6. Al salir, solicitar revocación a Go, eliminar la cookie y purgar estados privados/blobs. Si no se confirma la revocación remota, explicar esa diferencia sin conservar una sesión local utilizable ni afirmar cierre global exitoso.

### Controles de seguridad y límites

- Cookie de host exacto, preferiblemente con prefijo `__Host-`, `Path=/`, sin `Domain`, `Secure` y `HttpOnly`; evaluar `SameSite=Lax` con los retornos y métodos reales.
- Cookie persistente con caducidad acotada por Go para permitir cerrar y reabrir el navegador. No confundirla con una cookie sin `Max-Age`/`Expires` ni prometer acceso indefinido.
- La implementación actual Go fija ocho horas. Persistir el transporte no permite volver autenticado días después si Go ya venció la sesión. Un plazo mayor requiere una decisión separada sobre backend/seguridad.
- Origin/CSRF para login, logout y toda mutación; SameSite por sí solo no sustituye el control. No aceptar destinos abiertos ni confiar ciegamente en headers forwarded.
- Respuestas autenticadas y de sesión sin caché compartida; nunca insertar el token o perfil de una persona en HTML público cacheable.
- Separar credencial de usuario de la identidad IAM Cloud Run de servicio. Conservar `X-Serverless-Authorization` y los audiences existentes; no introducir cookies crudas en solicitudes a Go.
- `401`/revocación expiran la UI y la cookie; tratar red/`503` como indisponibilidad, no como prueba automática de credenciales inválidas.
- Sin tokens en `localStorage`, `sessionStorage`, IndexedDB, URLs, logs, analítica, traces o capturas. HttpOnly reduce el acceso directo desde JavaScript a la cookie; no elimina todos los riesgos XSS o CSRF.
- Avisos entre pestañas sin credenciales, por ejemplo para sincronizar logout; cada pestaña revalida con Go. Comprobar también regreso desde historial/bfcache y recuperación de foco.
- Migración controlada: eliminar únicamente la clave legacy `espacigo_auth_session` y advertir que puede ser necesario iniciar sesión una vez. No usar `sessionStorage.clear()` ni invalidar sesiones de otras personas por inferencia.
- Cookie de aplicación e IAP son mecanismos distintos. No compartir cookies entre `espacigo.testingpage.store` y `espacigo-demo.testingpage.store` ni cambiar IAP/DNS para este alcance.
- Pruebas de los flags Secure con HTTPS local aislado cuando sea necesario; no relajar la configuración productiva para acomodar un ensayo HTTP.

## 5. Ejecución por fases y aceptación

### Fase 1 — marca y especificación visual

**Entrega:** inventario de tokens recuperados, especificación visual Next, estados de componentes y wireframes de acceso/cabeceras/cuenta.
**Aceptación:** terracota consistente, logo real, contraste documentado, layouts móvil/escritorio; no cambios de autenticación ni restauración Angular.

### Fase 2 — sesión persistente y autoridad Go

**Cierre técnico local (26 de octubre de 2026): completado.** 214 unitarias, 69 E2E/Axe aprobadas (45 omisiones explícitas), 14 pruebas HTTPS de BFF real y 7 de BFF→Go real→PostgreSQL nativo autorizado; tipos/lint/formato/build aprobados y revisiones independientes de servidor y fachadas sin bloqueos. Incluye invalidación de estado local privado por cambio de sesión y estabilidad del menú durante la comprobación inicial de invitado. Véase `evidence/ui-auth-fase2-sesion-bff.md` para alcance, secuencia histórica, limpieza y límites. No acredita aceptación visual de la paleta ni login de cuenta en producción. La fase 3 no se ha iniciado.

**Precondición cumplida:** decisión de cookie/BFF aprobada expresamente por el usuario; implementar después de cerrar la fase 1.
**Entrega:** route handlers de login/sesión/logout, migración de transporte de operaciones autenticadas y retirada de Web Storage; conservar allowlist de retorno, idempotencia, cancelación y blobs privados.
**Aceptación:** login confirmado, recarga y retorno con sesión válida, rechazo de sesión vencida/revocada, cero token accesible desde JS, protección CSRF, funcionamiento entre instancias y logout coherente entre pestañas. Sin ampliar TTL ni tocar SQL/IAM por inferencia.

### Fase 3 — identidad y navegación comunes

**Entrega:** componente de marca, controles de sesión compartidos, selector reactivo de nombre/ID y cabeceras pública/workspace.
**Aceptación:** «Ingresar» solo a invitados; «Mi cuenta» directo a autenticados; ningún «Acceso» en Mi cuenta; nombre actualizado sin reload; fallback ID; sin datos de una cuenta anterior.

### Fase 4 — rediseño de acceso y pulido de cuenta

**Entrega:** acceso unificado, estados de recuperación/error/carga, sidebar y perfil con los tokens aprobados.
**Aceptación:** flujo claro sin login redundante ni formularios simultáneos para otra cuenta; navegación por teclado, lector de pantalla, contraste y responsividad; conservar capacidades y funcionalidades reales.

### Fase 5 — regresión y entrega

**Entrega:** pruebas unitarias, contrato/transporte, Playwright y Axe locales; evidencia de integración con Go usando únicamente la cuenta sintética ya autorizada si procede; revisión independiente y publicación por el Cloud Build existente cuando se autorice el corte.
**Aceptación:** tipos/lint/formato/unitarias/build en verde; flujo completo local; después SHA→build→digest→revisión→tráfico verificados. Distinguir pruebas controladas de integración real y no afirmar pruebas de cuenta online que no se ejecutaron.

## 6. Matriz mínima de pruebas

- Invitado → acceso con logo → login confirmado → cuenta o retorno válido.
- Cuenta → inicio → CTA «Mi cuenta» → cuenta sin pasar por acceso.
- Recarga, pestaña nueva, cierre/reapertura dentro de vigencia; caducidad real y revocación.
- Sin perfil/404 → ID; guardar nombre → nueva identidad visible inmediatamente.
- Nombre largo, acentos, espacios, ID completo y vista móvil sin overflow.
- Logout desde cuenta y cabecera → invitado; atrás/bfcache no expone datos anteriores.
- Logout con fallo de red; carreras de carga/guardado de perfil; cambio de persona A→B.
- CSRF y retorno externo rechazados; cookies con flags correctos; token ausente de respuestas cliente, almacenamiento JS y logs.
- Diferencia entre `401`, `403` de autorización y `503`; restricciones/capacidades siempre confirmadas por Go.
- Móvil 390×844 y escritorio 1365×900, teclado, foco visible, Axe sin violaciones conocidas y movimiento reducido.
- Regresión de cuenta, anfitrión, reservas, administración y binarios privados tras cambiar el transporte.

## 7. Alcance de este documento

Es un plan en ejecución, no una implementación completa ni autorización de duración de sesión nueva. La persistencia fue aprobada expresamente y consta en `adr_ui_auth_001_persistencia_bff.md`; aún no se implementó al iniciar la fase 1. Los colores y la especificación visual no modifican por sí solos los archivos de UI o autenticación productiva. Mantener las decisiones en el repositorio y la skill del proyecto para que no dependan del modelo del chat.

Referencias de seguridad consultadas: fuente oficial OWASP `CheatSheetSeries/cheatsheets/Session_Management_Cheat_Sheet.md` y fuente MDN `content/files/en-us/web/http/reference/headers/set-cookie/index.md`. Se accedió a sus fuentes oficiales en GitHub porque el HTML OWASP devolvió 403. Estas fuentes sustentan el manejo de cookies, Web Storage y caducidad; no sustituyen el contrato específico de EspaciGo.
