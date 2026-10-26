# Corrección del flujo visible de sesión y navbar

## Instrucción del usuario — 26 de octubre de 2026

El usuario revisó y **aprobó los colores**, pero rechazó como incompleto el flujo visible entregado: el navbar autenticado seguía mostrando «Acceso» y el sitio público no ofrecía identidad y retorno directo a Mi cuenta. La validación técnica anterior de cookies/BFF no acredita este recorrido de usuario. No se modifica la paleta aprobada ni se introduce un rediseño.

## Criterios vinculantes del nuevo corte

1. Mientras Go confirma sesión, mostrar un estado discreto de comprobación, no una identidad anterior ni un CTA de invitado que parpadee.
2. En el navbar de Mi cuenta/workspace autenticado: únicamente nombre de perfil confirmado por Go —o ID de cuenta si no hay perfil— y **Cerrar sesión**, además de la marca existente. Sin «Acceso», segundo login, avatar/menú nuevo o acciones extra.
3. En el navbar público autenticado: nombre o ID y **Mi cuenta**, enlazado directamente a `/mi-cuenta`. Volver al inicio, recargar o abrir otra pestaña no debe pedir credenciales mientras Go confirme válida la sesión.
4. Invitado o sesión invalidada: mostrar el acceso correspondiente sin conservar nombre, ID ni datos de la cuenta anterior.
5. Obtener el nombre solo mediante `GET /profile`; no cargar pagos, privacidad o fotos para poblar una cabecera. Un 404 de perfil significa sin configurar, no logout. Guardar perfil actualiza la identidad visible desde la respuesta Go sin recarga.
6. Reutilizar AuthFacade, cookies BFF y logout existentes. No nuevo token, sesión paralela, Web Storage de identidad, JWT, refresh, ampliación de vencimiento o cambios de backend/SQL/IAM.
7. Probar el **DOM y la navegación completa** sobre HTTPS y BFF real con upstream sintético local: login → cuenta → navbar sin Acceso → inicio público → Mi cuenta sin segundo login → recarga → logout. Cubrir nombre/ID, actualización de perfil, permisos/cuenta distintos y respuestas tardías.
8. Mantener Tailwind y composición/colores actuales; verificar escritorio y móvil, contraste y overflow.

## Diagnóstico inicial comprobado

Base local limpia `main` `ae73ede2d194174f462101fe5dda6b4082d74bbc`.

- `frontend/src/app/(workspace)/layout.tsx` renderiza un enlace estático «Acceso», sin observar AuthFacade.
- `frontend/src/app/(marketplace)/layout.tsx` renderiza «Ingresar a EspaciGo» de forma incondicional.
- La infraestructura de sesión aprobada ya existe; el sitio público no la consumía en su cabecera. La implementación debe comprobar restauración real y no asumir que corregir los textos prueba las cookies.

## Ejecución

En curso, con propiedad separada de archivos para identidad compartida, controles/layouts y regresión HTTPS real. Un único propietario del build cuando terminen los escritores. No se considera aceptado o publicado el arreglo hasta verificar el flujo pedido y la entrega remota.

### Primer contraste del padre

Se leyeron el store de identidad y los controles reales. Typecheck, lint y formato pasaron tras corregir formato del runner; las unitarias y el build se ejecutaron correctamente. Sobre el build nuevo, el runner HTTPS sin interceptar el BFF obtuvo **6 PASS / 2 FAIL**: ya pasan login con navbar autenticado sin Acceso, clic directo a Mi cuenta sin segundo login, nombre reactivo al guardar, logout, purga entre pestañas y auditoría de secretos. Los dos fallos leen el navbar mientras todavía se está confirmando sesión; se encargó esperar explícitamente los controles autenticados y añadir verificación móvil/escritorio.

La lectura detectó además que el snapshot SSR del store se publica como invitado en vez de comprobando. Se delegó su corrección y las carreras de revalidación/actor con regresiones reales. No se acepta parpadeo del CTA invitado ni restauración de nombre de otro actor.

Regresión global provisional: **65 PASS / 4 FAIL / 45 SKIP**. Los cuatro fallos intentan rellenar el formulario de login cuando una sesión ya activa redirige correctamente; se encargó actualizar solo esos helpers/asserts legacy conservando retorno seguro, casos 401 y pruebas de dominio. No se revierte la navegación pedida para hacer pasar selectores antiguos.

### Verificación HTTPS del padre sobre el build corregido

Tras volver a ejecutar tipos, lint, formato, unitarias y build, el padre ejecutó `tooling/header-session-browser.mjs`: **11 PASS / 0 FAIL**, exit 0, sin interceptar endpoints del BFF. Verificados en DOM real: login a cuenta sin Acceso, regreso público con Mi cuenta, entrada directa sin segundo login, recarga con perfil 404/ID, actualización del nombre tras guardar, logout, dos pestañas, comprobación retardada sin CTA invitado, nombres e IDs largos en 1365×900 y 390×844 sin overflow y exactamente dos controles autenticados; auditoría sin secretos.

También se verificaron los nuevos RED/GREEN del store para SSR checking, revalidación síncrona que no revive A y caché residual de actor anterior. La revisión independiente `deleg_812c8c3c` devolvió JSON válido con `passed: true`, sin hallazgos de seguridad, lógica o sugerencias, y confirmó 229 unitarias aprobadas. Queda contrastar la entrega de helpers legacy y repetir la regresión global; el arreglo aún no se ha publicado.

### Regresión global contrastada

El padre ejecutó la suite global sobre el build nuevo: **229 unitarias PASS / 0 FAIL; 69 E2E/Axe PASS / 0 FAIL / 45 SKIP**, de 114 escenarios. Se repitió también la suite HTTPS de seguridad/transporte BFF: **14 PASS / 0 FAIL**, además de las 11 comprobaciones del flujo visible. Tipos/lint/formato/build en verde y revisión independiente aprobada. La paleta y los archivos de estilos aprobados no tienen diferencias contra la base.

Los 45 skips históricos no constituyen prueba de integración nativa de todos los dominios. Las comprobaciones de este corte son locales y sintéticas. Una lectura final de los helpers detectó un probe contra un global de test inexistente (no está expuesto en producción); se está sustituyendo por observación del navbar real, sin alterar el código productivo ni los asserts de dominio. Tras ese ajuste se publicará y se verificará el despliegue por el flujo existente.

### Cierre local del corte corregido

Después de eliminar de los helpers la consulta al global inexistente y sustituirla por los controles DOM reales, el padre releyó los cambios y repitió los gates globales: **229 unitarias, 69 E2E/Axe aprobadas y 45 omisiones explícitas**; tipos/lint/formato sin errores. También repitió el recorrido HTTPS de navbar: **11/11 PASS** sobre el último build. La suite HTTPS previa del BFF permanece en **14/14 PASS**. Revisión independiente aprobada, sin bloqueos.

Se detuvo el fixture público propio y los runners cerraron sus servidores HTTP/TLS, procesos Next y Chromium. No cambian paleta/CSS, backend, SQL, IAM ni los archivos AGENTS. La paridad read-only con la API activa confirmó 447 archivos backend idénticos; la identidad Git/GitHub verificada es `HernanEspinozaDev`.

**Corrección validada localmente.** Se publica siguiendo el push directo a `main` ya autorizado y el trigger web regional existente. Este documento no afirma anticipadamente despliegue o una prueba con cuenta de producción: verificar SHA remoto → Cloud Build → digest → revisión → tráfico y conservar el reporte de release en `.local/deployments/`. No se han pedido ni usado credenciales reales. Los colores quedan expresamente aceptados por el usuario y sin modificaciones en este corte.

Pruebas exclusivamente locales y sintéticas; sin cuentas nuevas, recuperación, credenciales reales, navegador del usuario, modificación de `AGENTS.md` ni datos retenidos.

## Corrección posterior: destino explícito después de cerrar sesión

El usuario confirmó casi todo el flujo publicado en `bfa4fff7103525ac2d9f476f96516557a060c36d`, pero indicó que **Cerrar sesión debe terminar en `/acceso`**, no permanecer en `/mi-cuenta`. El escenario previo comprobaba la purga del navbar, no el destino. La lectura del padre confirmó que `handleLogout` esperaba el cierre pero no ejecutaba navegación.

La entrega acotada `deleg_30983ee9` incorporó `useRouter` y `router.replace("/acceso")` en `finally`, después de `await logout()`. No cambia la revocación ni la limpieza existentes: también se navega cuando la limpieza local ya ocurrió y la confirmación remota falla, sin afirmar éxito remoto. No modifica colores, geometría, fachadas, backend, SQL ni IAM.

El agente reportó RED previo: 10/11 escenarios HTTPS y 3/4 contratos de controles, fallando precisamente la navegación ausente. Después del build corregido, el padre inspeccionó el diff completo y ejecutó **229/229 unitarias, 69 E2E/Axe PASS / 45 SKIP / 0 FAIL y 11/11 HTTPS sobre Next/BFF real**. Tipos/lint/formato en verde. La prueba ahora exige ruta `/acceso`, formulario visible, permanencia tras recarga y ausencia de identidad privada al retroceder; se espera el DOM invitado real, sin sleeps arbitrarios. Se preservan el acceso directo sin segundo login y las dos pestañas.

La paridad read-only vuelve a confirmar 447 archivos backend idénticos a la API activa. La publicación sigue el trigger web existente y requiere verificar nuevamente SHA → build → digest → revisión → tráfico → HTTP antes de declarar entrega. El reporte exacto de esa publicación queda en `.local/deployments/`, sin utilizar cuentas reales.
