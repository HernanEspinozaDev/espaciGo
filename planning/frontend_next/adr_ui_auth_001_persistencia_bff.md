# ADR UI-AUTH-001 — persistencia de sesión con transporte BFF

## Estado y aprobación

**Aceptada por el usuario para implementación por fases.** Mensaje de aprobación: «si apruebo la persistencia, comienza con el plan de ejecucion».

La aceptación decide la arquitectura objetivo; **no significa que la cookie o la migración de autenticación ya estén implementadas**. En la fase 1 se entregan marca, especificación y maqueta. La base productiva inspeccionada es `8a87e38d1b27c384d054ff4bf463c46bdc49c9d2`.

## Contexto

El usuario requiere volver al sitio con una sesión vigente, mostrar Mi cuenta en la cabecera pública y eliminar el acceso redundante del workspace. Las reglas históricas prohibían cookies propias y exigían Bearer en memoria. Sin embargo, el código actual `auth-facade.ts` ya guarda credenciales en `sessionStorage`, con copy incompatible con ese comportamiento. No se extenderá ese almacenamiento a `localStorage`.

## Decisión

- Go conserva emisión, validación, autorización, vencimiento y revocación de la sesión opaca.
- Next será únicamente BFF same-origin: recibe credenciales de login, confirma la sesión con Go y establece su transporte mediante cookie HttpOnly, Secure, host-only, `Path=/`, sin `Domain`, con política SameSite compatible y validada.
- El prefijo `__Host-` será la opción productiva preferida si el contrato y los ensayos confirman su viabilidad. No emitir variantes inseguras en producción para facilitar ensayos HTTP.
- El token no se entrega al JavaScript cliente por JSON, SSR props, HTML, URLs, Web Storage, IndexedDB, logs o trazas. Las operaciones protegidas atraviesan el gateway; la cookie no se reenvía cruda a Go.
- Persistencia significa continuidad **solo hasta el vencimiento Go**. La implementación inspeccionada crea sesiones de ocho horas; no se agrega renovación ni duración nueva desde Next.
- El BFF será independiente de una instancia Cloud Run particular: no usar un Map local como repositorio de sesiones. No crear ORM, base de sesiones paralela, NextAuth ni JWT propios.
- Proteger login/logout y mutaciones contra CSRF/Origin; no asumir que SameSite o HttpOnly eliminan todos los riesgos XSS/CSRF.
- Sesión y perfil confirmado no se servirán mediante caché compartida. El catálogo público no incorpora información personalizada en HTML cacheable.
- Conservar la autenticación IAM de servicio y su audience separada de la credencial de usuario; no alterar IAP del servicio privado ni compartir la cookie entre dominios.
- La migración elimina exclusivamente `espacigo_auth_session`. Puede exigir un nuevo login una vez; no borrar otros datos del navegador ni invalidar masivamente sesiones ajenas.

## Reglas de interacción

- Identidad desconocida: estado comprobando, sin confiar en roles restaurados desde el navegador.
- Invitado confirmado: Ingresar a EspaciGo.
- Autenticado confirmado: Mi cuenta directo; workspace con nombre del perfil o ID de cuenta y Cerrar sesión.
- Guardar perfil actualiza la identidad desde la respuesta Go, sin recarga. Perfil 404 se trata como no configurado.
- Logout elimina la continuidad local y purga datos/blobs privados, diferenciando cierre local de revocación remota confirmada.
- Abortar/descartar respuestas tardías al cambiar la generación de sesión. Ninguna cabecera puede mostrar el nombre de una persona anterior.
- No consultar pagos, exportaciones o fotos para obtener únicamente un nombre.

## Consecuencias y límites

Esta decisión sustituye específicamente la prohibición histórica de cookie de aplicación para el transporte de sesión. No autoriza cambios SQL, TTL backend, IAM, Terraform, creación de cuentas reales ni pruebas online autenticadas. No modificar `AGENTS.md`; la instrucción humana posterior y esta ADR documentan el nuevo alcance.

La API sigue siendo la autoridad. Ocultar enlaces no crea permisos; un fallo de red no demuestra que la credencial haya vencido. Las respuestas 401/403/503 se distinguirán conforme al contrato y operación reales.

## Gate de fase 2

Antes de declarar persistencia implementada: pruebas de login→confirmación→cookie, recarga/nueva pestaña/retorno, expiry/revocación, logout/carreras, CSRF, flags de cookie, ausencia de tokens en superficies JS, no caché compartida y regresión de todos los adaptadores protegidos. Probar la continuidad entre procesos/instancias sin inventar resultados de Cloud Run.

La fase 1 muestra esos estados como **simulación visual explícita**, sin cookies ni llamadas a la API.
