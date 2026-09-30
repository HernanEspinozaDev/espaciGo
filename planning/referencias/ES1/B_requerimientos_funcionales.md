# Catálogo Consolidado de Requerimientos Funcionales (RQF)

## Sistema: EspaciGo — Marketplace SaaS B2B2C de Espacios Comerciales

El catálogo reúne los **236 requerimientos funcionales** de EspaciGo (RQF-001 a RQF-236), redactados con la estructura «El sistema debe…» y distribuidos en los 11 módulos descritos en el Anexo A. Cada requerimiento tiene un identificador único y estable: los complementarios se incorporaron al final del catálogo, sin renumerar los anteriores, de modo que todas las referencias del informe, de los módulos, de los casos de uso y de las historias de usuario siguen siendo válidas.

*Tabla. Requerimientos funcionales base (RQF-001 a RQF-185)*

| ID | Descripción del Requerimiento Funcional |
| :--- | :--- |
| RQF-001 | El sistema debe permitir registrar una cuenta de usuario. |
| RQF-002 | El sistema debe validar que el correo electrónico ingresado posea un formato válido. |
| RQF-003 | El sistema debe validar que la contraseña posea un mínimo de 8 caracteres. |
| RQF-004 | El sistema debe validar que la contraseña posea al menos una letra mayúscula. |
| RQF-005 | El sistema debe validar que la contraseña posea al menos un número. |
| RQF-006 | El sistema debe validar que la contraseña posea al menos un carácter especial. |
| RQF-007 | El sistema debe rechazar el registro si el correo electrónico ya existe en la plataforma. |
| RQF-008 | El sistema debe enviar un correo electrónico con un token de verificación tras el registro. |
| RQF-009 | El sistema debe cambiar el estado de la cuenta a "Verificado" al confirmar el correo electrónico. |
| RQF-010 | El sistema debe bloquear el inicio de sesión para cuentas en estado "No Verificado". |
| RQF-011 | El sistema debe permitir iniciar sesión. |
| RQF-012 | El sistema debe validar las credenciales ingresadas durante el inicio de sesión. |
| RQF-013 | El sistema debe rechazar el inicio de sesión si las credenciales son incorrectas. |
| RQF-014 | El sistema debe incrementar un contador de intentos fallidos al rechazar un inicio de sesión. |
| RQF-015 | El sistema debe bloquear la cuenta de usuario al alcanzar 5 intentos fallidos consecutivos. |
| RQF-016 | El sistema debe enviar un correo de alerta de seguridad al bloquear una cuenta. |
| RQF-017 | El sistema debe liberar automáticamente una cuenta bloqueada tras 30 minutos. |
| RQF-018 | El sistema debe generar una sesión temporal tras un inicio de sesión exitoso. |
| RQF-019 | El sistema debe permitir solicitar la recuperación de contraseña. |
| RQF-020 | El sistema debe enviar un enlace único de recuperación al correo electrónico. |
| RQF-021 | El sistema debe expirar el enlace de recuperación tras 15 minutos de su emisión. |
| RQF-022 | El sistema debe permitir actualizar la contraseña utilizando el enlace de recuperación. |
| RQF-023 | El sistema debe permitir cerrar la sesión activa. |
| RQF-024 | El sistema debe permitir consultar los datos del perfil de usuario. |
| RQF-025 | El sistema debe permitir modificar el nombre de perfil del usuario. |
| RQF-026 | El sistema debe validar que el nombre de perfil contenga únicamente caracteres alfabéticos y espacios. |
| RQF-027 | El sistema debe permitir modificar el número telefónico del usuario. |
| RQF-028 | El sistema debe validar que el número telefónico contenga exactamente 9 dígitos. |
| RQF-029 | El sistema debe permitir cargar una fotografía de perfil. |
| RQF-030 | El sistema debe validar que la fotografía de perfil no supere los 2MB de tamaño. |
| RQF-031 | El sistema debe permitir eliminar la fotografía de perfil. |
| RQF-032 | El sistema debe permitir registrar una cuenta bancaria asociada al perfil. |
| RQF-033 | El sistema debe validar que el RUT de la cuenta bancaria coincida con el RUT del usuario. |
| RQF-034 | El sistema debe permitir solicitar la eliminación de la cuenta de usuario. |
| RQF-035 | El sistema debe rechazar la eliminación de la cuenta si existen reservas activas. |
| RQF-036 | El sistema debe rechazar la eliminación de la cuenta si existen pagos pendientes. |
| RQF-037 | El sistema debe rechazar la eliminación de la cuenta si existen disputas abiertas. |
| RQF-038 | El sistema debe permitir solicitar la validación de identidad KYC (Persona Natural). |
| RQF-039 | El sistema debe permitir cargar una fotografía del anverso de la Cédula de Identidad. |
| RQF-040 | El sistema debe permitir cargar una fotografía del reverso de la Cédula de Identidad. |
| RQF-041 | El sistema debe validar que las imágenes de la cédula posean un formato compatible. |
| RQF-042 | El sistema debe extraer el RUT desde la fotografía del anverso. |
| RQF-043 | El sistema debe extraer el número de serie desde la fotografía. |
| RQF-044 | El sistema debe validar la vigencia de la cédula ante el organismo correspondiente. |
| RQF-045 | El sistema debe cambiar el estado del perfil a "Verificado _KYC" tras una validación exitosa. |
| RQF-046 | El sistema debe rechazar la solicitud KYC si el documento se encuentra vencido. |
| RQF-047 | El sistema debe notificar al usuario el resultado de su validación KYC. |
| RQF-048 | El sistema debe permitir solicitar la validación de identidad KYB (Empresa). |
| RQF-049 | El sistema debe permitir ingresar el RUT de la empresa. |
| RQF-050 | El sistema debe validar el dígito verificador del RUT ingresado. |
| RQF-051 | El sistema debe consultar el inicio de actividades de la empresa. |
| RQF-052 | El sistema debe rechazar la validación KYB si la empresa no posee inicio de actividades. |
| RQF-053 | El sistema debe cambiar el estado del perfil a "Verificado_KYB" tras una validación exitosa. |
| RQF-054 | El sistema debe permitir al administrador consultar el listado de validaciones fallidas. |
| RQF-055 | El sistema debe permitir al administrador aprobar manualmente una validación de identidad. |
| RQF-056 | El sistema debe permitir al administrador rechazar manualmente una validación de identidad. |
| RQF-057 | El sistema debe exigir al administrador ingresar un motivo al rechazar una validación. |
| RQF-058 | El sistema debe permitir al usuario modificar los antecedentes rechazados de una validación. |
| RQF-059 | El sistema debe permitir al usuario solicitar un nuevo intento de validación. |
| RQF-060 | El sistema debe permitir acceder al panel de creación de publicaciones. |
| RQF-061 | El sistema debe bloquear el panel de creación de publicaciones a usuarios no verificados. |
| RQF-062 | El sistema debe permitir ingresar un título para la publicación. |
| RQF-063 | El sistema debe validar que el título no exceda los 70 caracteres. |
| RQF-064 | El sistema debe permitir ingresar la descripción de la publicación. |
| RQF-065 | El sistema debe validar que la descripción contenga al menos 100 caracteres. |
| RQF-066 | El sistema debe permitir ingresar la superficie total del inmueble en metros cuadrados. |
| RQF-067 | El sistema debe validar que la superficie ingresada sea un número mayor a cero. |
| RQF-068 | El sistema debe permitir seleccionar el tipo de inmueble. |
| RQF-069 | El sistema debe permitir establecer la capacidad máxima de personas. |
| RQF-070 | El sistema debe permitir agregar reglas de uso a la publicación. |
| RQF-071 | El sistema debe permitir configurar la modalidad tarifaria (por hora, día o mes). |
| RQF-072 | El sistema debe permitir ingresar el precio base del arriendo. |
| RQF-073 | El sistema debe validar que el precio base ingresado sea mayor a \$5.000 CLP. |
| RQF-074 | El sistema debe permitir cargar fotografías a la galería de la publicación. |
| RQF-075 | El sistema debe validar que la galería no exceda las 10 fotografías. |
| RQF-076 | El sistema debe rechazar la carga de fotografías que superen los 5MB. |
| RQF-077 | El sistema debe permitir seleccionar una fotografía como portada. |
| RQF-078 | El sistema debe permitir eliminar una fotografía de la galería. |
| RQF-079 | El sistema debe permitir ingresar la dirección física completa del inmueble. |
| RQF-080 | El sistema debe registrar la ubicación geográfica a partir de la dirección ingresada. |
| RQF-081 | El sistema debe asignar el estado "Borrador" automáticamente durante la creación. |
| RQF-082 | El sistema debe cambiar el estado a "Activa" al confirmar la creación. |
| RQF-083 | El sistema debe generar un calendario de disponibilidad para la publicación. |
| RQF-084 | El sistema debe permitir al arrendador bloquear fechas en el calendario. |
| RQF-085 | El sistema debe permitir al arrendador desbloquear fechas manualmente. |
| RQF-086 | El sistema debe permitir modificar el título de una publicación existente. |
| RQF-087 | El sistema debe permitir modificar el precio base de una publicación existente. |
| RQF-088 | El sistema debe permitir cambiar el estado de una publicación activa a "Oculta". |
| RQF-089 | El sistema debe ocultar en los resultados públicos las publicaciones en estado "Oculta". |
| RQF-090 | El sistema debe permitir cambiar el estado de una publicación "Oculta" a "Activa". |
| RQF-091 | El sistema debe permitir eliminar permanentemente una publicación. |
| RQF-092 | El sistema debe rechazar la eliminación de una publicación si posee reservas futuras pendientes. |
| RQF-093 | El sistema debe permitir al usuario consultar su listado de publicaciones. |
| RQF-094 | El sistema debe permitir buscar publicaciones ingresando texto en una barra de búsqueda. |
| RQF-095 | El sistema debe mostrar un mapa interactivo con las ubicaciones de los espacios. |
| RQF-096 | El sistema debe mostrar una cuadrícula de tarjetas con el resumen de las publicaciones. |
| RQF-097 | El sistema debe permitir buscar a través de filtros de rango de precio. |
| RQF-098 | El sistema debe permitir buscar a través de filtros de tipo de inmueble. |
| RQF-099 | El sistema debe permitir buscar a través de filtros de superficie mínima. |
| RQF-100 | El sistema debe permitir buscar a través de filtros de rango de fechas. |
| RQF-101 | El sistema debe excluir de los resultados las publicaciones sin disponibilidad en las fechas consultadas. |
| RQF-102 | El sistema debe permitir limpiar todos los filtros de búsqueda aplicados. |
| RQF-103 | El sistema debe permitir consultar los detalles completos de una publicación. |
| RQF-104 | El sistema debe calcular el costo total multiplicando el precio base por el tiempo seleccionado. |
| RQF-105 | El sistema debe calcular el monto de la comisión de servicio sobre el costo total. |
| RQF-106 | El sistema debe calcular el monto de la garantía exigida sobre el costo total. |
| RQF-107 | El sistema debe mostrar el desglose de cobro (Estadía + Comisión + Garantía). |
| RQF-108 | El sistema debe permitir seleccionar las fechas/horas de reserva en la publicación. |
| RQF-109 | El sistema debe validar que la fecha de inicio seleccionada sea posterior a la fecha actual. |
| RQF-110 | El sistema debe validar que la fecha de fin sea posterior a la fecha de inicio. |
| RQF-111 | El sistema debe validar la disponibilidad del espacio para el intervalo solicitado. |
| RQF-112 | El sistema debe rechazar la reserva si existe superposición temporal con otra reserva o bloqueo. |
| RQF-113 | El sistema debe registrar la reserva en estado "Pendiente de Pago" al confirmar disponibilidad. |
| RQF-114 | El sistema debe permitir al usuario seleccionar un método de pago mediante la pasarela integrada. |
| RQF-115 | El sistema debe iniciar el flujo de cobro comunicándose con la pasarela de pagos. |
| RQF-116 | El sistema debe cambiar el estado de la reserva a "Pagada" al recibir confirmación exitosa. |
| RQF-117 | El sistema debe mantener retenidos los fondos de la reserva hasta que se cumplan las condiciones de liberación. |
| RQF-118 | El sistema debe ejecutar una pre-autorización por el monto de la garantía. |
| RQF-119 | El sistema debe cambiar el estado a "Cancelada_Por_Pago" si la pasarela rechaza la transacción. |
| RQF-120 | El sistema debe cancelar automáticamente las reservas "Pendientes de Pago" tras 15 minutos sin completarse. |
| RQF-121 | El sistema debe notificar al arrendatario cuando el pago sea procesado. |
| RQF-122 | El sistema debe notificar al arrendador sobre la recepción de una nueva solicitud pagada. |
| RQF-123 | El sistema debe permitir al arrendador consultar las solicitudes de reserva entrantes. |
| RQF-124 | El sistema debe permitir al arrendador aprobar una solicitud de reserva. |
| RQF-125 | El sistema debe cambiar el estado de la reserva a "Aprobada_Host" tras la aprobación. |
| RQF-126 | El sistema debe permitir al arrendador rechazar una solicitud de reserva. |
| RQF-127 | El sistema debe exigir al arrendador seleccionar un motivo al rechazar una solicitud. |
| RQF-128 | El sistema debe ejecutar el reembolso total al arrendatario tras el rechazo del arrendador. |
| RQF-129 | El sistema debe cancelar las solicitudes no respondidas por el arrendador en un plazo de 24 horas. |
| RQF-130 | El sistema debe iniciar la generación del contrato tras la aprobación de la reserva. |
| RQF-131 | El sistema debe consultar los datos legales de las partes involucradas y del inmueble. |
| RQF-132 | El sistema debe generar un documento de contrato incorporando los datos extraídos. |
| RQF-133 | El sistema debe enviar el documento al proveedor de firma electrónica externa. |
| RQF-134 | El sistema debe enviar el enlace de firma correspondiente al arrendatario. |
| RQF-135 | El sistema debe enviar el enlace de firma correspondiente al arrendador. |
| RQF-136 | El sistema debe cambiar el estado a "Firma_Parcial" tras recibir la confirmación de la primera firma. |
| RQF-137 | El sistema debe almacenar el contrato final asociado a la reserva tras la confirmación de todas las firmas. |
| RQF-138 | El sistema debe cambiar el estado a "Lista_Para_Checkin" tras el almacenamiento del contrato. |
| RQF-139 | El sistema debe notificar a las partes cuando el contrato esté completamente firmado. |
| RQF-140 | El sistema debe cancelar las reservas que alcancen su fecha de inicio sin poseer el contrato firmado. |
| RQF-141 | El sistema debe reembolsar el pago al arrendatario en caso de cancelación por falta de firmas. |
| RQF-142 | El sistema debe permitir a las partes descargar el contrato firmado. |
| RQF-143 | El sistema debe habilitar la opción de "Check-in" únicamente el día de inicio de la reserva. |
| RQF-144 | El sistema debe bloquear el Check-in fuera de la fecha de inicio de la reserva. |
| RQF-145 | El sistema debe permitir al arrendatario cargar fotografías en el formulario de Check-in. |
| RQF-146 | El sistema debe permitir al arrendatario ingresar comentarios de texto en el formulario de Check-in. |
| RQF-147 | El sistema debe registrar la fecha y ubicación al procesar el Check-in. |
| RQF-148 | El sistema debe cambiar el estado de la reserva a "En_Curso" tras procesar el Check-in. |
| RQF-149 | El sistema debe notificar al arrendador cuando el arrendatario procese el Check-in. |
| RQF-150 | El sistema debe permitir al arrendatario registrar el Check-out de la reserva. |
| RQF-151 | El sistema debe permitir cargar fotografías en el formulario de Check-out. |
| RQF-152 | El sistema debe cambiar el estado de la reserva a "Finalizada" al registrar el Check-out. |
| RQF-153 | El sistema debe permitir al arrendatario registrar una calificación numérica del espacio. |
| RQF-154 | El sistema debe permitir al arrendatario registrar una reseña de texto del espacio. |
| RQF-155 | El sistema debe mostrar las reseñas y calificaciones en la vista de detalle de la publicación. |
| RQF-156 | El sistema debe permitir al arrendatario enviar mensajes en el chat de la reserva. |
| RQF-157 | El sistema debe permitir al arrendador enviar mensajes en el chat de la reserva. |
| RQF-158 | El sistema debe permitir a las partes consultar el historial de mensajes de la reserva. |
| RQF-159 | El sistema debe habilitar el registro de reclamos durante las 24 horas posteriores al término de la reserva. |
| RQF-160 | El sistema debe permitir al arrendador registrar un reclamo adjuntando texto descriptivo. |
| RQF-161 | El sistema debe exigir al arrendador cargar fotografías de evidencia en el reclamo. |
| RQF-162 | El sistema debe bloquear la liberación de fondos de una reserva al detectar un reclamo registrado. |
| RQF-163 | El sistema debe cambiar el estado de la reserva a "En_Disputa" tras el registro de un reclamo. |
| RQF-164 | El sistema debe notificar al arrendatario sobre la apertura de una disputa. |
| RQF-165 | El sistema debe permitir al arrendatario ingresar texto y fotografías de defensa. |
| RQF-166 | El sistema debe permitir al administrador consultar el listado de disputas abiertas. |
| RQF-167 | El sistema debe permitir al administrador consultar la evidencia fotográfica vinculada a la disputa. |
| RQF-168 | El sistema debe permitir al administrador registrar un fallo a favor del arrendatario. |
| RQF-169 | El sistema debe permitir al administrador registrar un fallo a favor del arrendador. |
| RQF-170 | El sistema debe exigir al administrador ingresar el monto a deducir de la garantía al fallar a favor del arrendador. |
| RQF-171 | El sistema debe cambiar el estado de la disputa a "Resuelta" tras el fallo del administrador. |
| RQF-172 | El sistema debe iniciar la liquidación (Payout) si transcurren las 24 horas de gracia sin reclamos. |
| RQF-173 | El sistema debe transferir el monto de la estadía (menos comisiones) a la cuenta bancaria del arrendador. |
| RQF-174 | El sistema debe ejecutar el cobro de la pre-autorización de garantía cuando el fallo exija compensación. |
| RQF-175 | El sistema debe liberar el saldo de garantía restante al arrendatario tras aplicar posibles deducciones. |
| RQF-176 | El sistema debe generar la boleta electrónica por la comisión cobrada. |
| RQF-177 | El sistema debe cambiar el estado de la reserva a "Cerrada" tras la ejecución exitosa de la liquidación. |
| RQF-178 | El sistema debe permitir al administrador bloquear el acceso a una cuenta de usuario. |
| RQF-179 | El sistema debe permitir al administrador desbloquear una cuenta de usuario. |
| RQF-180 | El sistema debe permitir al administrador consultar el motivo de bloqueo de una cuenta. |
| RQF-181 | El sistema debe permitir al administrador ocultar una reseña reportada. |
| RQF-182 | El sistema debe exigir al administrador registrar un motivo al ocultar una reseña. |
| RQF-183 | El sistema debe permitir al administrador generar reportes de gestión de la plataforma. |
| RQF-184 | El sistema debe permitir al administrador consultar el historial de auditoría de un usuario filtrando por RUT. |
| RQF-185 | El sistema debe permitir al administrador exportar los registros de auditoría consultados. |

## Requerimientos funcionales complementarios (RQF-186 a RQF-212)

> Los 27 requerimientos complementarios (RQF-186 a RQF-212) completan el catálogo base en los aspectos que no estaban cubiertos: aceptación de términos y condiciones, operaciones faltantes sobre entidades ya existentes, notificaciones y evidencias de los flujos alternativos. Se incorporan con identificadores nuevos al final del catálogo, sin renumerar ni modificar ningún requerimiento anterior.

| ID | Descripción del Requerimiento Funcional |
| :--- | :--- |
| RQF-186 | El sistema debe exigir la aceptación de los términos y condiciones durante el registro de la cuenta. |
| RQF-187 | El sistema debe registrar la fecha y la versión de los términos y condiciones aceptados. |
| RQF-188 | El sistema debe permitir reenviar el correo de verificación de la cuenta. |
| RQF-189 | El sistema debe permitir consultar la cuenta bancaria registrada en el perfil. |
| RQF-190 | El sistema debe permitir modificar la cuenta bancaria asociada al perfil. |
| RQF-191 | El sistema debe permitir eliminar la cuenta bancaria asociada al perfil. |
| RQF-192 | El sistema debe permitir capturar una fotografía tipo selfie durante la validación KYC. |
| RQF-193 | El sistema debe permitir consultar el estado de la validación de identidad. |
| RQF-194 | El sistema debe notificar al usuario el resultado de su validación KYB. |
| RQF-195 | El sistema debe permitir modificar la descripción de una publicación existente. |
| RQF-196 | El sistema debe permitir modificar las reglas de uso de una publicación existente. |
| RQF-197 | El sistema debe permitir modificar la capacidad máxima de una publicación existente. |
| RQF-198 | El sistema debe permitir modificar la modalidad tarifaria de una publicación existente. |
| RQF-199 | El sistema debe permitir consultar el historial de reservas asociadas al usuario. |
| RQF-200 | El sistema debe liberar las fechas bloqueadas al cancelar una reserva. |
| RQF-201 | El sistema debe notificar al arrendatario cuando el pago sea rechazado. |
| RQF-202 | El sistema debe registrar el rechazo de firma notificado por el proveedor de firma electrónica. |
| RQF-203 | El sistema debe exigir la carga de al menos una fotografía para procesar el Check-in. |
| RQF-204 | El sistema debe exigir la carga de al menos una fotografía para registrar el Check-out. |
| RQF-205 | El sistema debe permitir al arrendador confirmar la recepción del espacio tras el Check-out. |
| RQF-206 | El sistema debe registrar la fecha y la ubicación al confirmar la recepción del espacio. |
| RQF-207 | El sistema debe permitir reportar una reseña publicada. |
| RQF-208 | El sistema debe liberar la totalidad de la garantía al arrendatario cuando la reserva finalice sin reclamos. |
| RQF-209 | El sistema debe permitir al administrador consultar la confirmación de recepción vinculada a la disputa. |
| RQF-210 | El sistema debe notificar el resultado de la disputa a las partes. |
| RQF-211 | El sistema debe enviar la boleta electrónica al correo del arrendador. |
| RQF-212 | El sistema debe permitir al administrador buscar una cuenta de usuario. |

### Origen de los requerimientos complementarios (RQF-186 a RQF-212)

| Necesidad detectada | RF agregados | Módulo |
| :--- | :--- | :--- |
| No existía requerimiento de aceptación de términos y condiciones. | RQF-186, RQF-187 | M01 |
| El flujo alternativo "reenviar token de verificación" (CU-02) no tenía RF que lo respaldara. | RQF-188 | M01 |
| La entidad "cuenta bancaria" solo tenía la operación de registro (RQF-032, RQF-033): faltaban consultar, modificar y eliminar. | RQF-189, RQF-190, RQF-191 | M02 |
| La HU04 contempla la captura de selfie de validación facial, sin RF asociado. | RQF-192 | M03 |
| El paso "consultar el estado de la validación" (CU-14) no tenía RF. | RQF-193 | M03 |
| Asimetría: el resultado de la validación se notificaba solo para KYC (RQF-047), no para KYB. | RQF-194 | M03 |
| La HU06 permite editar descripción, reglas de uso, capacidad y tarifas, pero el catálogo solo permitía editar título (RQF-086) y precio base (RQF-087). | RQF-195, RQF-196, RQF-197, RQF-198 | M04 |
| No existía RF para consultar las reservas del usuario, pese a ser precondición de CU-24, CU-26, CU-33, CU-34 y CU-42. | RQF-199 | M06 |
| Las cancelaciones automáticas (CU-27, CU-28) liberaban las fechas sin RF que lo declarara. | RQF-200 | M06 |
| El rechazo del pago cambiaba de estado (RQF-119) sin notificar al arrendatario. | RQF-201 | M06 |
| El escenario "una parte no firma" (CU-30) no tenía RF que registrara el rechazo notificado por el proveedor. | RQF-202 | M07 |
| La obligatoriedad de evidencia fotográfica en Check-in y Check-out había quedado fuera del catálogo. | RQF-203, RQF-204 | M08 |
| **El arrendador no tenía ninguna acción al término de la reserva.** | RQF-205, RQF-206 | M08 |
| RQF-181 presupone reseñas reportadas, pero no existía RF que permitiera reportar una reseña. | RQF-207 | M09 |
| **No existía RF de liberación de la garantía en el flujo sin reclamos** (solo RQF-175 "saldo restante tras deducciones"). | RQF-208 | M10 |
| El administrador no tenía RF para consultar la evidencia de recepción del arrendador al arbitrar (simetría con RQF-167). | RQF-209 | M10 |
| Solo existía la notificación de apertura de disputa (RQF-164), no la del resultado. | RQF-210 | M10 |
| La boleta electrónica solo se generaba (RQF-176); no se enviaba al arrendador. | RQF-211 | M10 |
| Bloquear/desbloquear una cuenta (RQF-178 a RQF-180) requiere localizarla previamente. | RQF-212 | M11 |

## Requerimientos funcionales complementarios (RQF-213 a RQF-236)

> Los 24 requerimientos complementarios (RQF-213 a RQF-236) respaldan los criterios de aceptación de las historias de usuario que no tenían un requerimiento funcional asociado: preferencia de uso, cambio de contraseña con sesión iniciada, límites y validaciones de los documentos de identidad, edición de publicaciones, política de cancelación, cancelación de la reserva por el arrendatario, reputación recíproca y supervisión de las publicaciones por el administrador. Se incorporan con identificadores nuevos al final del catálogo, sin renumerar ni modificar los requerimientos anteriores.

| ID | Descripción del Requerimiento Funcional |
| :--- | :--- |
| RQF-213 | El sistema debe registrar la preferencia de uso del usuario durante el registro de la cuenta. |
| RQF-214 | El sistema debe permitir al usuario autenticado cambiar su contraseña. |
| RQF-215 | El sistema debe validar la contraseña actual al solicitar un cambio de contraseña. |
| RQF-216 | El sistema debe rechazar una contraseña nueva igual a la contraseña actual. |
| RQF-217 | El sistema debe rechazar una contraseña nueva utilizada en los últimos 3 meses. |
| RQF-218 | El sistema debe notificar al usuario el cambio de contraseña realizado. |
| RQF-219 | El sistema debe validar que las imágenes de la cédula no superen los 10MB. |
| RQF-220 | El sistema debe cambiar el estado del perfil a "Pendiente de Verificación" al recibir los documentos de identidad. |
| RQF-221 | El sistema debe validar que el precio base modificado sea mayor a \$5.000 CLP. |
| RQF-222 | El sistema debe validar que la regla de uso personalizada no exceda los 250 caracteres. |
| RQF-223 | El sistema debe permitir configurar la política de cancelación de la publicación. |
| RQF-224 | El sistema debe mostrar las reglas de uso y la política de cancelación en el detalle de la publicación. |
| RQF-225 | El sistema debe exigir un motivo al registrar un bloqueo manual de fechas. |
| RQF-226 | El sistema debe validar que la fecha de término del bloqueo manual sea posterior a la fecha de inicio. |
| RQF-227 | El sistema debe rechazar la reserva si el usuario no cuenta con su identidad verificada. |
| RQF-228 | El sistema debe permitir al arrendatario cancelar una reserva conforme a la política de cancelación. |
| RQF-229 | El sistema debe calcular el monto a reembolsar según la política de cancelación aplicada. |
| RQF-230 | El sistema debe ejecutar el reembolso al arrendatario tras la cancelación de la reserva. |
| RQF-231 | El sistema debe notificar a las partes la cancelación de la reserva. |
| RQF-232 | El sistema debe permitir al arrendador registrar una calificación numérica del arrendatario. |
| RQF-233 | El sistema debe permitir al arrendador registrar una reseña de texto del arrendatario. |
| RQF-234 | El sistema debe rechazar el registro de una segunda reseña para la misma reserva. |
| RQF-235 | El sistema debe calcular el promedio de calificaciones del espacio. |
| RQF-236 | El sistema debe permitir al administrador consultar el listado de espacios publicados. |

### Origen de los requerimientos complementarios (RQF-213 a RQF-236)

| Historia de usuario | Necesidad detectada | RF agregados | Módulo |
| :--- | :--- | :--- | :--- |
| HU01 | La HU pedía seleccionar el perfil principal (Arrendador/Arrendatario) y no existía RF. Se redefine como **preferencia de uso no excluyente**. | RQF-213 | M01 |
| HU03 | La HU describía el **cambio de contraseña con sesión iniciada**, que no existía en el catálogo (solo la recuperación), junto con sus validaciones (contraseña actual, no repetir la anterior, no reutilizar en 3 meses) y la notificación del cambio. | RQF-214 a RQF-218 | M01 |
| HU04 | La HU definía un **límite de 10 MB** por documento y un estado **"Pendiente de Verificación"** inexistentes en el catálogo. | RQF-219, RQF-220 | M03 |
| HU06 | La HU validaba la tarifa al **modificar** la publicación; el catálogo solo validaba el precio al crearla (RQF-073). | RQF-221 | M04 |
| HU07 | La HU definía un **máximo de 250 caracteres** para reglas personalizadas y la existencia de una **política de cancelación** configurable y visible en el detalle. | RQF-222, RQF-223, RQF-224 | M04 |
| HU21 | La HU exigía **motivo obligatorio** y validación de fechas en el bloqueo manual del calendario. | RQF-225, RQF-226 | M04 |
| HU17 | La HU exigía validar la **identidad verificada antes de pagar**; el caso de uso lo declaraba como precondición sin RF que lo respaldara. | RQF-227 | M06 |
| HU19 | La HU describía la **cancelación de la reserva por el arrendatario** con cálculo de devolución, ejecución del reembolso y notificación a las partes. El catálogo solo contemplaba el rechazo del arrendador, la expiración por falta de pago y la falta de firma. | RQF-228 a RQF-231 | M06 |
| HU25 | La HU pedía **calificación y reseña recíproca** (arrendador hacia arrendatario), el bloqueo de una segunda reseña por reserva y el recálculo del promedio de calificaciones. | RQF-232 a RQF-235 | M09 |
| HU26 | La HU requería que el administrador pudiera **consultar el listado de espacios publicados** para su supervisión. | RQF-236 | M11 |

El catálogo queda consolidado en **236 requerimientos funcionales (RQF-001 a RQF-236)**, distribuidos en los 11 módulos descritos en el Anexo A.