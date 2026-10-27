# Plan de mejora de registro, acceso y verificación — EspaciGo

Fecha: 26 de octubre de 2026. Base inspeccionada: `715aca46d2e6b01d244ccccf8bc155bfa649885e`.
Estado: ejecución integral y publicación autorizadas por el usuario. La instrucción posterior es continuar hasta terminar, iterar entre implementación/revisión, aprobar internamente los diseños y publicar sin pedir aprobaciones visuales intermedias; el usuario revisará cuando esté montado. Se mantienen las puertas técnicas y la obligación de no inventar datos legales, evidencia de esquema o confirmaciones de entrega. Un bloqueo real de backend/identidad legal debe informarse y no se elude mediante cambios de infraestructura o SQL.

## 1. Objetivo y límites

Crear un flujo claro, moderno y breve: registro → registro exitoso → verificación del correo → acceso. Eliminar de la experiencia normal los campos de token, instrucciones técnicas y documentos legales extensos dentro del formulario. Usar el logo real de EspaciGo, Tailwind y la paleta Terracota + Crema ya aceptada.

La modernización visual solicitada se limita a las pantallas de identidad y sus estados; no autoriza rediseñar marketplace, workspace o navbar ni cambiar la sesión ya validada. Se conservan cookies BFF HttpOnly/Secure, Go como autoridad, navegación autenticada directa y logout a `/acceso`. No nuevos proveedores de acceso, JWT propio, refresh, cuentas de prueba nuevas, migraciones o cambios IAM por inferencia.

## 2. Diagnóstico comprobado en el código

| Hallazgo                                                                                            | Fuente                                                                                     | Consecuencia                                                                                               |
| --------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------- |
| Registro exitoso conserva el formulario y solo cambia un mensaje.                                   | `frontend/src/features/auth/ui/registration-screen.tsx`                                    | No hay cierre claro ni URL de resultado.                                                                   |
| Ambos documentos completos aparecen abiertos antes de los campos.                                   | `registration-screen.tsx`, `DocumentPanel`                                                 | La lectura legal desplaza y sobrecarga el registro.                                                        |
| Hay un aviso de documentos sintéticos añadido incondicionalmente por la UI.                         | `registration-screen.tsx`                                                                  | Lenguaje de desarrollo visible.                                                                            |
| Verificación y recuperación exponen entradas manuales de ID/token cuando no llega un enlace válido. | `frontend/src/features/auth/ui/action-screens.tsx`                                         | Se presentan herramientas técnicas como parte del acceso habitual.                                         |
| El enlace de correo ya incluye la credencial en el fragmento y la interfaz la captura.              | `internal/adapters/zohoauth/mailer.go`, `ui/use-action-credential.ts`                      | No es necesario pedir al usuario que copie tokens.                                                         |
| AuthShell usa `CircleIcon`; existe `/espacigo-mark.svg`.                                            | `ui/auth-shell.tsx`, `frontend/public/espacigo-mark.svg`                                   | Puede corregirse la identidad sin inventar otro logo.                                                      |
| Ya existen páginas legales y enlaces en el footer público.                                          | `frontend/src/app/footer.tsx`, rutas `/terminos-y-condiciones` y `/politica-de-privacidad` | Reutilizar y extender su acceso al grupo de autenticación, sin duplicar políticas.                         |
| El correo se genera en Go como texto plano y sus asuntos incluyen «(pruebas)».                      | `internal/adapters/zohoauth/mailer.go`                                                     | Una plantilla HTML y asuntos productivos requieren una tarea backend separada; no se arreglan desde React. |

### Dependencia real de documentos legales

No se trata solamente de una etiqueta visual: los JSON de `internal/identity/consentdocs/` contienen una razón social ficticia, un contacto declarado no operativo y cláusulas expresamente limitadas a evaluación. Go publica `synthetic: true` y el validador frontend exige actualmente ese valor. Además, Go calcula el hash del documento y lo contrasta con versiones publicadas; no se pueden editar textos y reutilizar IDs/hashes antiguos.

Se conservará la base actual para su revisión y se moverá su consulta fuera del formulario. No se presentará una identidad/contacto inventados como información real ni se falsificará el indicador del backend. Quitar el aviso técnico incondicional de la UI se separa de aprobar/publicar los documentos: antes de exponerlos como definitivos se necesitan responsable y contacto efectivos, confirmación del alcance y publicación versionada autorizada. El cambio de contrato para documentos productivos debe coordinar tipos, validadores, Go y publicación legal; no implica autorizar SQL o un despliegue completo ahora.

No copiar `contacto@chilepropiedades.cl`, datos personales del ejemplo ni inventar un correo de soporte EspaciGo. No se encontró un canal `mailto:` en el frontend inspeccionado. El soporte visible se habilitará cuando exista un canal confirmado.

## 3. Flujo propuesto

### A. `/registro`: crear cuenta

- Logo real con enlace al inicio, título «Crea tu cuenta en EspaciGo» y descripción breve.
- Una tarjeta de una columna: correo y contraseña con mostrar/ocultar; sin pedir perfil, teléfono, rol o datos que Go no acepte en el registro.
- Requisitos de contraseña compactos; no dos bloques legales abiertos ni textos sobre infraestructura.
- Dos aceptaciones independientes, inicialmente desmarcadas: «Acepto los Términos y condiciones» y «He leído y acepto la Política de privacidad», enlazadas a las páginas públicas. Abrir los documentos sin perder el formulario.
- CTA principal «Crear cuenta»; secundario discreto «¿Ya tienes cuenta? Inicia sesión».
- Mantener IDs de las versiones entregadas por Go. Si cambian o Go rechaza el consentimiento, desmarcar aceptaciones y pedir revisar las versiones nuevas. No inventar una API histórica o una versión estática.
- Si no se cargan documentos completos, explicar «No pudimos cargar los documentos necesarios. Inténtalo nuevamente», ofrecer reintento y bloquear solo el envío; conservar los datos no sensibles del formulario.

### B. `/registro-exitoso`: cuenta creada, verificar correo

Después de una respuesta confirmada de creación, limpiar la contraseña y reemplazar la ruta de registro con `/registro-exitoso` para evitar reenvíos al retroceder. No iniciar sesión automáticamente ni dar la cuenta por verificada.

**Copy propuesto para creación y envío aceptados por Go (201):**

> **Tu cuenta en EspaciGo fue creada con éxito**
>
> **Siguiente paso: verifica tu correo electrónico**
>
> Te enviamos un enlace de verificación a **[correo utilizado]**. Ábrelo y confirma tu correo para activar tu cuenta.
>
> El mensaje puede tardar unos minutos. Si no aparece en tu bandeja de entrada, revisa Spam o Correo no deseado. Si lo encuentras allí, márcalo como «No es spam».
>
> ¿No recibiste el mensaje? Puedes solicitar un nuevo enlace.

Acción principal «Reenviar enlace»; secundaria «Ir a iniciar sesión». Ayuda desplegable para correo no recibido. No usar bloques de advertencia en mayúsculas ni repetir dos veces toda la explicación. No prometer un plazo de entrega o recepción en la bandeja de entrada.

**Estado distinto para 202:** «Tu cuenta fue creada, pero no pudimos confirmar el envío del correo. Solicita un nuevo enlace para continuar». No mostrar «correo enviado» en ese caso. El handler actual distingue 201 de creación con envío aceptado y 202 de cuenta creada con un problema posterior de entrega.

El correo y el estado proceden únicamente del envío exitoso de esta persona, con un traspaso efímero en memoria del cliente. No correo, cuenta, token ni contraseña en query strings, SSR o almacenamiento persistente. Al recargar o visitar directamente sin contexto, mostrar instrucciones neutrales y permitir introducir un correo para solicitar instrucciones; no inventar una creación exitosa ni recuperar información privada. El registro no se reenvía automáticamente ante un timeout ambiguo.

La reemisión devuelve hoy 204 incluso para casos que no permiten confirmar existencia o entrega. Copy: «Si corresponde, recibirás un nuevo enlace para verificar tu correo». No afirmar «correo reenviado» ni inventar un contador de bloqueo. Deshabilitar mientras la solicitud está pendiente; respetar `Retry-After` únicamente cuando la operación lo exponga realmente.

### C. Correo de EspaciGo

Asunto propuesto: «Verifica tu correo y activa tu cuenta en EspaciGo».

- Marca EspaciGo, saludo breve, una explicación y CTA «Verificar mi correo».
- Vencimiento derivado del plazo real informado por Go, no una duración inventada.
- Aviso «Si no creaste esta cuenta, puedes ignorar este mensaje».
- HTML sencillo con alternativa de texto plano y logo compatible con clientes de correo; no depender únicamente de un SVG remoto.
- Remitente y contacto existentes verificados; no modificar credenciales SMTP, DNS, proveedor ni IAM dentro del trabajo de diseño.
- Mantener el enlace seguro del backend y no publicar el secreto en parámetros de consulta, imágenes o seguimiento. Sin píxeles ni analítica.

Esta fase modifica el mailer Go y necesita autorización explícita de alcance backend, revisión y el procedimiento de publicación correspondiente. No se enviarán correos a personas ni se crearán cuentas para probarla sin autorización adicional; la comprobación inicial se realizará con un capturador SMTP local.

### D. `/verificar-correo`: una confirmación clara, sin campos técnicos

El comportamiento que relató el usuario —abrir el enlace y confirmar— ya funciona y se conservará como base. El token viaja dentro del enlace: no hay que pedirlo, mostrarlo o copiarlo.

- Enlace válido: logo, «Confirma tu correo para activar tu cuenta» y un único botón «Verificar mi correo».
- Capturar y purgar inmediatamente el fragmento de la URL; conservar el secreto solo en memoria y enviarlo en el cuerpo de la solicitud existente.
- Mantener una confirmación voluntaria, no consumir automáticamente el enlace al cargar. Se simplifica la interfaz sin alterar la protección actual frente a aperturas automáticas de enlaces.
- Sin enlace, enlace incompleto, inválido, usado o vencido: explicación breve y acción «Solicitar un nuevo enlace», nunca formulario de ID/token. No distinguir estados secretos que Go agrupa como inválidos.
- No reintentar automáticamente una operación de un solo uso cuando la respuesta sea ambigua.
- Error de red: explicación recuperable y siguiente paso seguro, sin mensajes de implementación.

### E. `/correo-verificado`: resultado y continuación

Propuesta de nueva ruta de resultado tras confirmación positiva de Go:

> **Correo verificado**
>
> Tu cuenta de EspaciGo ya está lista. Inicia sesión para continuar.

Un CTA «Continuar»: a Mi cuenta si ya hay una sesión vigente confirmada; de lo contrario a `/acceso`. La verificación no emite ni sustituye una sesión. No guardar ni reutilizar la contraseña del registro para un auto-login. Visitas sin resultado confirmado no deben afirmar que se verificó una cuenta. Conservar el retorno interno permitido cuando realmente exista contexto; no inventar continuidad entre dispositivos o pestañas.

### F. `/acceso` y recuperación

- Mismo AuthShell renovado, logo, jerarquía visual y microcopy de marca.
- Correo, contraseña y «¿Olvidaste tu contraseña?» claramente ordenados; una sola acción principal.
- Mantener mostrar/ocultar, pegado, gestores de contraseñas y autocomplete correctos.
- Sesión confirmada: acceso directo al destino permitido; no volver a pedir credenciales ni romper el navbar validado.
- Errores inline junto al campo y resumen accesible; credenciales inválidas con mensaje genérico. Mostrar ayuda de verificación solo si el contrato realmente permite ese estado.
- Recuperación: correo → instrucciones → enlace → contraseña nueva → confirmación → acceso. Sin ID/token manuales ni pestañas técnicas «Tengo un enlace» como flujo principal.
- Logout mantiene exactamente `/acceso`; verificarlo como regresión obligatoria.

## 4. Diseño y rendimiento

- Mantener Tailwind v4 y los colores aceptados. Modernizar únicamente identidad: logo, tarjeta limpia, títulos cortos, espaciado consistente, iconos sobrios de estado y acciones con jerarquía inequívoca.
- No copiar el diseño del sitio de referencia ni recuperar la maqueta Angular rechazada. Reutilizar los componentes reales de la aplicación y presentar una propuesta visible en las rutas reales antes de extenderla a todo el flujo.
- Footer legal compacto compartido en acceso, registro, verificación, recuperación y resultados; mantener los enlaces globales existentes y agregar acceso discreto donde falte, sin colocar un footer comercial completo dentro del workspace.
- Mobile-first, navegación por teclado, foco visible, lectores de pantalla, estados anunciados y respeto de movimiento reducido. Evaluar escritorio 1365×900 y móvil 390×844 sin ocultar overflow globalmente.
- Lazy loading selectivo: conservar la división por rutas de Next; diferir componentes secundarios de ayuda y material no visible solo cuando el análisis del bundle lo justifique. Separar verificación/recuperación si comparten código innecesario.
- Logo, formulario inicial y captura/purga del fragmento son críticos: no cargarlos de forma diferida. No añadir spinners artificiales ni descargar todos los textos legales para renderizarlos dentro del registro.
- Skeletons con dimensiones estables, estado del botón durante solicitudes y prevención del doble envío. Medir bundle y tiempos antes/después, sin inventar mejoras o umbrales alcanzados.

## 5. Validaciones y seguridad a conservar

- Correo válido, obligatorio y compatible con la normalización real de Go; no «corregir» direcciones silenciosamente.
- Contraseña según la política actual inspeccionada: mínimo 8 puntos de código, máximo 72 bytes UTF-8, mayúscula, número y signo. No cambiar la política backend dentro de una reparación de UI.
- Validación contextual, no errores rojos al primer carácter; `aria-invalid`, `aria-describedby`, labels persistentes y foco en el primer error tras submit.
- Consentimientos obligatorios, vigentes y vinculados a los documentos realmente revisados. Legal no disponible/incompatible: no sustituir por IDs ficticios.
- Manejo explícito de 409, 422, 429 cuando se exponga, 503 y red/timeout. No duplicar altas o consumo de enlaces por reintentos automáticos.
- Sin tokens en DOM, logs, URLs de requests, almacenamiento, capturas o trazas. Pruebas con secretos de acción sin screenshots/video/traces.
- Purga de contraseñas y respuestas tardías al salir/cambiar de identidad. Preservar cookies, expiración y autoridad Go existentes.

## 6. Ejecución por fases y puertas de aprobación

| Fase                               | Entregable                                                                                                   | Condición para avanzar                                                                                                                    |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------- |
| 1. Diseño y contratos              | Inventario focalizado ya iniciado, estados/copy, propuesta visual de acceso/registro/éxito y decisión legal. | Aprobar propuesta y resolver qué documentos pueden publicarse como definitivos.                                                           |
| 2. Shell y acceso                  | Logo real, diseño de identidad, footer legal, validaciones y carga inicial.                                  | Comparación visible móvil/escritorio; sesión/navbar/logout intactos.                                                                      |
| 3. Registro y resultado            | Formulario compacto, consentimientos enlazados, `/registro-exitoso`, estados 201/202 y reemisión.            | RED→GREEN de creación, error, recarga, entrada directa y consentimiento cambiado.                                                         |
| 4. Verificación y recuperación     | Interfaz sin token manual, confirmación única, resultado y acceso.                                           | Enlaces válidos/inválidos/usados, purga de secretos y sin dobles consumos.                                                                |
| 5. Correo y documentos productivos | Plantilla de email de marca y, si corresponde, documentos versionados reales.                                | Autorización backend/legal explícita; sin cambios SQL/SMTP/IAM inferidos ni sustitución de hashes. Puede separarse de las fases frontend. |
| 6. Verificación y entrega          | Gates, revisión independiente, evidencia, publicación según alcance autorizado y contraste del despliegue.   | SHA → build → digest → revisión → tráfico → HTTP.                                                                                         |

Programación delegada en cortes pequeños, un escritor por archivo y un único propietario del build. Pruebas locales con datos sintéticos: esta condición de trabajo no es copy para clientes. No usar el navegador del usuario, crear cuentas reales, recuperar accesos o enviar correos reales sin permiso adicional. No desplegar mientras el plan esté pendiente de aprobación.

## 7. Aceptación final

- Registro correcto sale del formulario y llega a `/registro-exitoso`, sin contraseña o correo en la URL.
- 201 y 202 tienen mensajes honestos diferentes; un timeout no genera un alta duplicada.
- Ya no hay documentos extensos, texto técnico ni campos ID/token en el flujo ordinario.
- Marca real y mismo lenguaje visual en todo el ciclo; políticas accesibles globalmente y aceptaciones conservadas.
- Reenvío no revela cuentas ni promete entrega no confirmada; contacto solo si es operativo.
- La persona abre su correo, confirma una sola vez y entiende cómo continuar; sin auto-login inventado.
- Recarga/visita directa de pantallas de resultado no revela PII ni inventa estados de cuenta.
- Se mantienen nombre/ID, entrada directa, dos pestañas y logout a `/acceso` ya aceptados.
- Casos de teclado/Axe, móvil/escritorio, formularios, respuestas tardías y secretos aprobados; tipos/lint/formato/unitarias/build/E2E con skips informados.
- Diferenciar evidencia UI controlada, BFF real e integración nativa autorizada. No afirmar entrega SMTP a una bandeja real a partir de un mock.

## 8. Archivos iniciales de intervención

- `frontend/src/features/auth/ui/auth-shell.tsx`, `access-screen.tsx`, `registration-screen.tsx`, `action-screens.tsx`, `password-field.tsx`.
- Nuevas pantallas/rutas de resultado dentro del grupo `(auth)` y un footer legal compacto compartido.
- `frontend/src/features/auth/domain/consent-documents.ts`, adaptadores/tipos solo si se aprueba el cambio de contrato legal.
- `frontend/src/features/public-catalog/ui/legal-document.tsx` y `frontend/src/app/footer.tsx` para consulta legal coherente, sin duplicar contenido ni falsear metadatos.
- Mailer Go y documentos versionados: tarea separada, no autorizada por una implementación exclusivamente frontend.

## 9. Referencias técnicas

- Código y handlers locales citados en este documento: evidencia principal del comportamiento actual, sin comprobar nuevos registros de producción.
- Next.js, Lazy Loading: https://nextjs.org/docs/app/guides/lazy-loading — diferir componentes/librerías no críticos sin bloquear la interacción inicial.
- OWASP, Forgot Password Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html — mensajes de solicitud uniformes y manejo seguro de credenciales de acción. No sustituye el contrato de verificación existente de EspaciGo.
