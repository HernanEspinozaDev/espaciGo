# Registro UX — primer corte local: base visual y acceso

## Encargo y alcance

El usuario autorizó comenzar `plan_registro_acceso_verificacion_ux.md`. Base comprobada: `715aca46d2e6b01d244ccccf8bc155bfa649885e`, rama `main`. Antes de comenzar, el único archivo pendiente era el propio plan documental. Los puertos locales previstos estaban libres; no se accedió a la aplicación online ni al navegador del usuario.

Superficie de diseño: **formulario de configuración/acceso**. Priorizar legibilidad, marca, jerarquía de acción y ayuda contextual; no hero publicitario, tarjetas decorativas, fuentes nuevas ni cambios de paleta global.

## Entrega en curso

- Sustituir el círculo de AuthShell por `/espacigo-mark.svg`.
- Modernizar la composición del formulario compartido y la presentación del acceso, conservando componentes reales Next/Tailwind.
- Footer legal compacto con las rutas existentes, accesible en las pantallas que usan AuthShell.
- Preservar sin cambios login/checkSession/return_to, cookies, controles compartidos y logout a `/acceso`.
- Capturas locales antes/después de acceso vacío, escritorio y móvil, TDD browser RED→GREEN, tipos/lint/formato/build y revisión del padre.

Implementación delegada en una tarea pequeña con propiedad exclusiva de AuthShell, AccessScreen (solo presentación), footer nuevo si hace falta y una suite browser nueva. Un único propietario del build; sin commits/push ni despliegues en esta etapa.

## Límites explícitos

Este corte no implementa todavía `/registro-exitoso`, no modifica registro/verificación/recuperación ni su consentimiento, no publica documentos ficticios como reales y no modifica mailer Go, SQL, IAM, infraestructura, datos retenidos o AGENTS. Las siguientes fases requieren su contraste propio; aprobar el plan no es prueba de funciones implementadas.

La inspección local confirmó que el ejecutable de Chrome configurado históricamente por MCP (`/opt/google/chrome/chrome`) continúa ausente. No se instalará un navegador ni se cambiará a un perfil del usuario para ocultar ese límite; las pruebas del proyecto usarán Chromium propio en loopback. El contraste MCP se informará por separado de los resultados CLI.

## Resultados provisionales del primer contraste

El padre inspeccionó el diff y ambas capturas posteriores. El cambio de producto inicial afecta solo AuthShell y el footer nuevo; AccessScreen seguía intacto, sin cambios de hooks/sesión. Las capturas muestran logo real, tarjeta de una columna y enlaces legales, sin recorte u overflow horizontal visible en escritorio/móvil. La entrega reportó 13 escenarios aprobados y 229 unitarias; esos totales todavía no se atribuyen a una repetición del padre.

Se detectaron tres ajustes antes del cierre: el texto bajo el CTA todavía explica detalles de cookies/servicio en lugar de hablar a la persona; los enlaces legales requieren un área táctil de al menos 44px; y la nueva suite no debe crear una captura etiquetada BEFORE sobre una implementación AFTER cuando no existe baseline en CI. Se delegó un ajuste exclusivo de esas áreas con RED→GREEN y regeneración de capturas posteriores. Las capturas BEFORE existentes se conservan sin alteración.

## Verificación repetida por el padre

Tras los ajustes, el padre repitió tipos/lint/formato sin errores, **229/229 unitarias**, **14/14 pruebas focalizadas AuthShell** y la suite global: **83 E2E/Axe PASS / 45 SKIP / 0 FAIL**, de 128 escenarios. También ejecutó sobre el mismo build el recorrido HTTPS con BFF real: **11/11 PASS**, incluyendo navbar autenticado, acceso directo, nombre reactivo, dos pestañas y logout exactamente a `/acceso` con recarga e historial.

Se inspeccionaron las capturas finales de escritorio/móvil: logo real, tarjeta y footer completos, copy sin jerga, sin recortes visibles. El área táctil del footer de al menos 44px y la ausencia de overflow móvil se comprobaron en DOM por las pruebas. La suite ya no recrea BEFORE; la espera del logo usa `expect.poll` para confirmar su carga real.

Se comprobó con `git diff --exit-code` que no cambiaron session-controls, fachadas de autenticación, registro, verificación/recuperación, estilos globales/paleta, backend ni AGENTS. El fixture propio fue detenido; los runners cerraron sus procesos. Las pruebas focalizadas interceptan el gateway para estados UI y no son evidencia de registro nativo; el recorrido HTTPS se informó por separado.

Revisión independiente read-only `deleg_3e5018d4` aprobada: JSON válido con `passed: true`, sin hallazgos de seguridad, lógica o sugerencias. Confirmó el alcance exclusivamente visual y la invariancia de hooks/efectos/sesión. Con los gates y capturas contrastados por el padre, este primer corte queda técnicamente validado en local. La aprobación técnica no sustituye la aceptación visual del usuario ni completa las fases posteriores de registro y correo. Nada de este corte está publicado.
