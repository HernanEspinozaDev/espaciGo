# Invariantes de identidad, cuenta y sesión (M01)

**Estado:** contrato de diseño revisado para AUTH-ARCH-01. Las decisiones de producto autorizadas están en [decisiones M01](decisiones_m01_identidad_sesion.md), ratificadas por el usuario el 2026-10-04. Este documento no implementa reglas, no altera ES1/ES2 y distingue requisitos fuente, decisiones autorizadas y brechas aún abiertas.

## 1. Identidad y registro

- Validar primero el formato del correo conforme RQF-002. Conservar el valor original y derivar `correo_normalizado` eliminando espacios externos y aplicando Unicode `casefold` independiente del idioma a toda la dirección. Aplicar la misma transformación en registro, login y recuperación. No añadir NFC/NFKC, normalización de puntos, reglas `+alias` ni reglas de proveedores.
- La identidad no distingue mayúsculas. PostgreSQL impone `UNIQUE(correo_normalizado)` para evitar duplicados también ante altas concurrentes. No usar una consulta previa como sustituto de la constraint.
- CU-01 registra la cuenta y la aceptación versionada según sus pasos; RQF-213 pide persistir una preferencia de uso no excluyente. La preferencia no es un rol de autorización. El modelo físico de esa preferencia sigue como brecha DB02-09: no afirmar que se persiste ni agregar una columna/tabla por inferencia.
- Mantener los estados del Anexo B: `correo_pendiente`, `activo`, `bloqueado`, `baja_solicitada`, `desidentificado`. No conceder permisos de publicación/KYC a partir de una preferencia de uso.

## 2. Duplicados y respuestas de registro/recuperación

- Para un correo duplicado, conservar CU-01 A1: rechazar el registro e informar que el correo ya existe, recomendando iniciar sesión o recuperar la contraseña (RQF-007).
- **Conflicto explícito:** revelar el duplicado permite enumerar cuentas; el control de amenaza anterior recomendaba respuestas equivalentes. En esta revisión se preserva CU-01 A1 por decisión expresa del usuario. No se cambia ES1 ni se propone ocultar el duplicado; una reconciliación futura debe pasar por cambio de requisito trazable y revisión.
- Para recuperación de contraseña, la respuesta pública es genérica y no revela si la cuenta existe. Esto difiere de CU-05 A2, que informa que no se puede continuar cuando el correo no está registrado. Mantener esa discrepancia visible sin reescribir el caso de uso.

## 3. Contraseña, roles y términos

- Aplicar la política de contraseña indicada en ES1 RQF-003–006 y no registrar ni almacenar la contraseña en claro; RNF-013 exige bcrypt con costo mínimo 12.
- El login conserva el contador de RQF-014, bloqueo tras 5 fallos consecutivos de RQF-015, alerta de seguridad al bloquear por RQF-016 y liberación a los 30 minutos de RQF-017. Esos límites son del login, no de los tokens.
- Los roles de autorización se modelan como roles coexistentes conforme ES2 `rol_usuario`. La preferencia de RQF-213 no concede ni sustituye un rol.
- La aceptación queda asociada a una versión identificable de términos y al acto/canal según ES1 RQF-186–188 y ES2 `version_terminos`/`aceptacion_terminos`; una versión publicada no se sobrescribe.

## 4. Verificación de correo y tokens

- CU-02/RQF-008–010 requiere token de verificación y cuenta no autenticable antes de verificar. TTL ratificado: 24 horas. Un token es aleatorio, de un solo uso, almacenado mediante hash y nunca aparece en logs. Reemitirlo invalida el token activo anterior del mismo usuario y propósito.
- Recuperación RQF-019–022/CU-05: TTL de 15 minutos, fijado por RQF-021/CU-05. Tras validar y consumir el token, actualizar la credencial; una solicitud existente o inexistente recibe la misma respuesta genérica. Los tokens expirados, consumidos, inválidos o invalidados no actualizan la cuenta.
- Máximo 5 intentos fallidos por token y 3 emisiones/reenvíos por cuenta y propósito en ventana móvil de una hora. Añadir limitación por IP en Backend. Estos límites no bloquean la cuenta ni el login; no confundirlos con el umbral de login de RQF-015/017.
- No incluir flujo de cambio de correo: no aparece como caso/RQF en ES1. Aunque ES2 B.2 enumera `cambiar_correo` entre los propósitos de `token_accion`, esa enumeración no agrega por sí sola un flujo; si un requisito posterior lo incorpora, la confirmación tendrá TTL de 15 minutos.

## 5. Sesiones y vencimiento

- RQF-018 crea una sesión temporal; RNF-016 obliga a vencer tras 30 minutos de inactividad y, como máximo, a las 8 horas desde su creación. Persistir `ultima_actividad_en` y la fecha absoluta (`expira_en = creada_en + 8 horas`).
- El Backend, usando su propio reloj, rechaza la sesión si `now >= ultima_actividad_en + 30 minutos` o `now >= expira_en`. Solo solicitudes autenticadas de operaciones de usuario actualizan `ultima_actividad_en`.
- Health checks, polling automático, keepalives y renovación de tokens no cuentan como actividad. Una renovación nunca extiende el vencimiento absoluto. CU-06/RQF-023 invalida la sesión al cerrar; la revocación también debe verificarse al autorizar.

## 6. Matriz de flujos éxito/fallo

| Flujo | Éxito | Fallo y respuesta/estado |
| --- | --- | --- |
| CU-01 registro | Formato válido, correo único y términos aceptados → cuenta según estado de verificación. | Formato/política/aceptación inválidos → no registrar. Duplicado → conservar CU-01 A1 y su mensaje explícito. La persistencia de preferencia RQF-213 sigue abierta. |
| CU-02 verificación | Token vigente y no consumido → habilitar cuenta. | Token expirado, inválido, consumido o reemplazado → no habilitar; reenvío cuenta para límites de token. |
| CU-03 login/CU-04 bloqueo | Cuenta habilitada y credencial correcta → emitir sesión. | Credencial errónea incrementa el contador; el quinto fallo activa bloqueo y alerta de seguridad (RQF-015/016); RQF-017 libera a los 30 min. No emitir sesión para cuenta no verificada/bloqueada. |
| CU-05 recuperación | Token de 15 min vigente → cambio de hash y consumo único. | Respuesta pública equivalente para cuenta existente/no existente; token inválido/vencido/reutilizado → sin cambio. La discrepancia con CU-05 A2 queda documentada. |
| CU-06 cierre | Revocar la sesión actual. | Una sesión revocada/vencida no se reactiva. |
| CU-50 cambio de contraseña | Validar sesión y contraseña actual; aplicar política ES1 y restricciones RQF-216/217; RQF-218 requiere notificar. | Credencial actual incorrecta, nueva inválida/igual o reutilizada → no actualizar. Persistencia de historial y evento específico de notificación siguen abiertos en DB02-09; no se inventan en este contrato. |

## 7. Amenazas, límites y brechas abiertas

- **Enumeración:** preservar la excepción explícita CU-01 A1; recuperación es genérica por autorización expresa. No generalizar una respuesta uniforme a otros casos sin revisar ES1.
- **Fuerza bruta:** conservar el bloqueo funcional del login según RQF-014/015/017; los límites de token e IP operan aparte y no bloquean el login.
- **Reuso/filtración de tokens:** hash en almacenamiento, un solo uso, invalidación al reemitir, TTL según propósito y ningún token en logs.
- **Vencimiento de sesión:** validación con reloj del Backend y dos límites (inactividad/absoluto); actividad solo de operaciones autenticadas del usuario.
- **DB02-09:** RQF-213 (preferencia no modelada) afecta persistencia de registro; RQF-217 (historial) afecta soportar CU-50; RQF-218 (evento) requiere contrato de notificación M09/M11. Los tres siguen abiertos y trazados en `decisiones_m01_identidad_sesion.md` y `revision_diccionario_datos.md`; no se declaran resueltos.

## Referencias

- ES1 `B_requerimientos_funcionales.md`: RQF-001–023, RQF-186–188 y RQF-213–218.
- ES1 `C_requerimientos_no_funcionales.md`: RNF-013, RNF-015 y RNF-016.
- ES1 `D_casos_de_uso.md`: CU-01–06 y CU-50, incluidos CU-01 A1 y CU-05 A2.
- ES2 `B_diccionario_datos.md`: B.1–B.2 (`usuario`, `rol_usuario`, `sesion`, `token_accion`, `version_terminos`, `aceptacion_terminos`).
- `revision_diccionario_datos.md`: crosswalk RQF-213–218 y hallazgo DB02-09.
- Decisiones ratificadas: `decisiones_m01_identidad_sesion.md`.
