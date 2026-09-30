# Anexo: Historias de Usuario — Sistema EspaciGo

> El anexo reúne las **35 historias de usuario (HU01–HU35)** del sistema, organizadas en **9 épicas**. Cada historia declara su rol, los requerimientos funcionales (RF) que la implementan, los casos de uso (CU) donde se especifica y sus criterios de aceptación, redactados como **comportamiento esperado del sistema**.
>
> **Cobertura de módulos:** los 11 módulos funcionales quedan cubiertos por al menos una historia de usuario (ver la tabla de cobertura al final) e incluyen los procesos de gestión del perfil y cuenta bancaria, eliminación de cuenta (Ley 21.719), contratos y firma electrónica, mensajería entre las partes y operación del arriendo (check-in, check-out y confirmación de recepción).

**Convenciones aplicadas**

1. **Nomenclatura de actores:** se usa **Arrendador** (nombre oficial del actor) en lugar de "Propietario"/"anfitrión", y **Arrendatario** en lugar de "cliente", para que las HU, los RF y los casos de uso compartan la misma terminología.
2. **Coherencia numérica:** todos los plazos, límites y umbrales de las historias coinciden con los del catálogo de RF (236 en total): intentos de bloqueo de sesión, vigencia del enlace de contraseña, largo del título, tarifa mínima, cantidad de fotografías, tamaño de archivos y estados de la cuenta.
3. **Trazabilidad:** cada HU declara `RF` (requerimientos que la implementan) y `CU` (casos de uso donde está especificada).
4. **Criterios de interfaz:** las validaciones y comportamientos que ocurren solo en la interfaz se indican como *(criterio de interfaz)* cuando no requieren un RF propio.
5. **Alcance:** los criterios que describen funcionalidad fuera del alcance declarado de la Evaluación Sumativa 1 se listan en el bloque **Fuera del alcance de la ES1**, de modo que ninguna HU quede con criterios sin respaldo documental.

**Épicas**

| Épica | Nombre | HU | Módulos involucrados |
|---|---|---|---|
| E1 | Identidad y Acceso | HU01–HU04, HU31, HU32 | M01, M02, M03 |
| E2 | Gestión de Espacios | HU05–HU09 | M04 |
| E3 | Búsqueda y Selección | HU10–HU14 | M05, M09 |
| E4 | Reservas, Pagos y Contratos | HU15–HU19, HU33 | M05, M06, M07, M10 |
| E5 | Disponibilidad y Calendario | HU20–HU24 | M04, M06 |
| E6 | Reputación y Comunicación | HU25, HU34 | M09 |
| E7 | Administración y Moderación | HU26–HU28 | M03, M10, M11 |
| E8 | Soporte y Notificaciones | HU29–HU30 | Transversal |
| E9 | Operación del Arriendo | HU35 | M08 |

# Épica E1 — Identidad y Acceso

## HU01 - Registro de nuevo usuario

**Épica:** E1 · **Rol (Como):** Visitante · **Prioridad:** Alta
**RF:** RQF-001 a RQF-010, RQF-186, RQF-187, RQF-213 · **CU:** CU-01, CU-02

**Como** visitante de la plataforma,
**Quiero** registrarme ingresando mis datos personales y señalando mi intención de uso (ofrecer espacios o arrendar espacios),
**Para que** pueda acceder a las funcionalidades del sistema con mi propia cuenta.

### Criterios de Aceptación
- El formulario de registro debe solicitar: nombre completo, correo electrónico, contraseña, teléfono de contacto y la intención de uso ("Quiero ofrecer espacios" / "Quiero arrendar espacios"). La intención de uso es una **preferencia de onboarding**: no excluye que la misma cuenta pueda publicar y arrendar posteriormente.
- El sistema debe validar que el correo electrónico tenga un formato válido y que no exista previamente; si ya existe, debe mostrar *"El correo electrónico ya está registrado, por favor inicie sesión o recupere su contraseña"*.
- La contraseña debe cumplir obligatoriamente: mínimo 8 caracteres, al menos una letra mayúscula, al menos un número y al menos un carácter especial. Si no cumple, el sistema debe indicar los requisitos faltantes.
- El sistema debe exigir la aceptación de los términos y condiciones para completar el registro y debe registrar la fecha y la versión aceptadas.
- Al confirmar el registro, el sistema debe crear la cuenta en estado **"No Verificado"** (no "activo") y enviar un correo con el token de verificación; debe permitir reenviar dicho correo si el token expira o no llega.
- El sistema debe permitir el inicio de sesión solo cuando la cuenta esté en estado "Verificado"; mientras no lo esté, debe bloquear el acceso e indicar que debe confirmar su correo.
- Al completarse el registro, el sistema debe redirigir al usuario a su panel principal o a la vista de perfil para completar su verificación de identidad.

> *(criterio de interfaz)* La pantalla de registro debe agrupar los campos en un solo formulario con validación en línea y mensajes de error junto a cada campo.

> **Fuera del alcance de la ES1:** registro con proveedores externos (login social) y captcha para el registro.

## HU02 - Inicio de sesión en la plataforma

**Épica:** E1 · **Rol (Como):** Usuario Registrado · **Prioridad:** Alta
**RF:** RQF-010 a RQF-018, RQF-023 · **CU:** CU-03, CU-04, CU-06

**Como** usuario registrado (Arrendador, Arrendatario o Administrador),
**Quiero** ingresar mis credenciales (correo y contraseña) en la plataforma,
**Para que** pueda autenticar mi identidad y acceder a mi panel de control correspondiente a mi rol.

### Criterios de Aceptación
- El formulario de acceso debe tener los campos obligatorios "Correo electrónico" y "Contraseña", el botón "Iniciar Sesión" y el enlace "¿Olvidó su contraseña?".
- Si las credenciales son incorrectas o el correo no está registrado, el sistema debe mostrar un mensaje genérico de seguridad (*"Correo electrónico o contraseña incorrectos"*) para evitar la enumeración de usuarios.
- El sistema debe incrementar un contador de intentos fallidos por cada rechazo y, al alcanzar **5 intentos fallidos consecutivos**, bloquear la cuenta durante **30 minutos**, notificando el bloqueo por correo de alerta de seguridad.
- El sistema debe liberar automáticamente la cuenta bloqueada al cumplir los 30 minutos e informar el tiempo restante si el usuario intenta ingresar durante el bloqueo.
- Si la cuenta se encuentra en estado "No Verificado", el sistema debe bloquear el acceso e indicar que debe confirmar su correo electrónico.
- Si las credenciales son correctas, el sistema debe generar una sesión temporal y redirigir al usuario a la vista correspondiente a su rol (Arrendador, Arrendatario o Administrador).
- El sistema debe permitir cerrar la sesión activa, invalidando la credencial temporal y redirigiendo al catálogo público.

> *(criterio de interfaz)* El formulario debe bloquear el botón de envío mientras se valida la respuesta del servidor y mostrar el aviso de bloqueo con el tiempo restante.

> **Fuera del alcance de la ES1:** autenticación de doble factor (2FA) para usuarios finales y bloqueo por dirección IP.

## HU03 - Cambiar y recuperar la contraseña

**Épica:** E1 · **Rol (Como):** Usuario Registrado · **Prioridad:** Media
**RF:** RQF-019 a RQF-022, RQF-214 a RQF-218 · **CU:** CU-05, CU-50

**Como** usuario registrado,
**Quiero** cambiar mi contraseña cuando la conozco y recuperarla cuando la he olvidado,
**Para que** pueda mantener la seguridad de mi cuenta y no perder el acceso a la plataforma.

### Criterios de Aceptación

**Escenario A — Cambio de contraseña con sesión iniciada**
- El sistema debe permitir al usuario autenticado cambiar su contraseña solicitando la contraseña actual, la nueva y su repetición.
- El sistema debe validar la contraseña actual; si no coincide, debe rechazar la operación con el mensaje *"La contraseña actual no es correcta"*.
- La contraseña nueva debe cumplir la misma política del registro (mínimo 8 caracteres, una mayúscula, un número y un carácter especial) y no puede ser igual a la contraseña actual.
- El sistema debe rechazar una contraseña nueva que haya sido utilizada en los últimos 3 meses, indicando *"La contraseña nueva ha sido registrada hace n° mes/es, ingrese una fuera de ese rango"*.
- Si el cambio es válido, el sistema debe actualizar la contraseña y enviar un correo de notificación al usuario informando que la contraseña fue modificada.

**Escenario B — Recuperación de contraseña olvidada**
- El usuario debe poder solicitar la recuperación ingresando su correo electrónico registrado.
- El sistema debe enviar al correo un enlace único de recuperación y **expirarlo a los 15 minutos** de su emisión.
- Si el enlace está expirado, el sistema debe invalidar la verificación y permitir solicitar uno nuevo.
- Con el enlace vigente, el usuario debe poder definir una nueva contraseña sujeta a la misma política y a la restricción de reutilización de 3 meses.

> *(criterio de interfaz)* El correo de notificación debe incluir un enlace "¿No fue usted?" que redirija al flujo de recuperación de contraseña y a los canales de soporte.

> **Fuera del alcance de la ES1:** bloqueo automático de la cuenta ante el aviso "¿No fue usted?" (la acción se deriva a soporte) y autenticación de doble factor para confirmar el cambio.

## HU04 - Verificación de identidad (KYC / KYB)

**Épica:** E1 · **Rol (Como):** Usuario Registrado · **Prioridad:** Alta
**RF:** RQF-038 a RQF-059, RQF-192, RQF-193, RQF-194, RQF-219, RQF-220 · **CU:** CU-10, CU-11, CU-12, CU-13, CU-14

**Como** usuario registrado (para operar como Arrendador o como Arrendatario),
**Quiero** acreditar mi identidad como persona natural (KYC) o la de mi empresa (KYB),
**Para que** mi perfil quede verificado y pueda publicar y reservar espacios de forma segura.

### Criterios de Aceptación

**Escenario A — Persona natural (KYC)**
- El perfil debe mostrar el estado de verificación de identidad del usuario ("No verificada", "Pendiente de Verificación", "Verificada _KYC" o "Rechazada") y permitir consultarlo en cualquier momento.
- El sistema debe permitir cargar la fotografía del **anverso y reverso de la Cédula de Identidad** en formatos de imagen compatibles y con un peso máximo de **10 MB por archivo**.
- El sistema debe permitir (opcionalmente) capturar una **selfie de validación facial** para contrastarla con el documento cargado.
- Al enviar los documentos, el sistema debe cambiar el estado del perfil a **"Pendiente de Verificación"** y mostrar *"Sus documentos han sido enviados con éxito y están siendo evaluados por nuestro sistema de seguridad"*.
- El sistema debe extraer el RUT y el número de serie desde las fotografías y validar la vigencia de la cédula ante el Registro Civil.
- Si el documento está vencido, el sistema debe rechazar la validación y derivar el caso a revisión manual del administrador, notificando el motivo al usuario.

**Escenario B — Empresa (KYB)**
- El sistema debe permitir ingresar el RUT de la empresa, validar su dígito verificador y consultar el inicio de actividades ante el SII.
- Si la empresa no registra inicio de actividades vigente, el sistema debe rechazar la validación, derivar el caso a revisión manual y notificar el resultado.
- Si la validación es exitosa, el sistema debe cambiar el estado del perfil a **"Verificado_KYB"** y notificar al usuario el resultado.

**Escenario C — Validación manual y reintento**
- El administrador debe poder consultar el listado de validaciones fallidas y aprobarlas o rechazarlas, ingresando obligatoriamente un motivo al rechazar.
- El usuario debe poder consultar el estado de su validación, corregir los antecedentes rechazados y solicitar un nuevo intento de validación.

> **Fuera del alcance de la ES1:** validación de Pasaporte u otros documentos distintos de la Cédula de Identidad, y validación biométrica presencial.

## HU31 - Administrar mi perfil y mi cuenta bancaria

**Épica:** E1 · **Rol (Como):** Usuario Registrado · **Prioridad:** Media
**RF:** RQF-024 a RQF-033, RQF-189 a RQF-191 · **CU:** CU-07, CU-08

**Como** usuario registrado,
**Quiero** mantener actualizados mis datos personales, mi fotografía y mi cuenta bancaria,
**Para que** mi información sea correcta y pueda recibir las liquidaciones de mis arriendos.

### Criterios de Aceptación
- El sistema debe permitir consultar los datos del perfil del usuario en cualquier momento.
- El usuario debe poder modificar su nombre de perfil, validándose que contenga únicamente caracteres alfabéticos y espacios.
- El usuario debe poder modificar su número telefónico, validándose que contenga exactamente 9 dígitos.
- El usuario debe poder cargar una fotografía de perfil, validándose que no supere los 2 MB, y eliminarla cuando lo desee.
- El usuario debe poder registrar su cuenta bancaria, validándose que el RUT del titular coincida con el RUT verificado del usuario.
- El usuario debe poder consultar, modificar y eliminar la cuenta bancaria registrada.
- La cuenta bancaria registrada es la que se utiliza para transferir el monto de la estadía en la liquidación de la reserva.

> **Fuera del alcance de la ES1:** validación bancaria mediante micro-depósitos y registro de múltiples cuentas bancarias simultáneas.

## HU32 - Ejercer el derecho de eliminación de mi cuenta

**Épica:** E1 · **Rol (Como):** Usuario Registrado · **Prioridad:** Media
**RF:** RQF-034 a RQF-037 · **RNF:** RNF-018, RNF-026, RNF-029 · **CU:** CU-09

**Como** usuario registrado,
**Quiero** solicitar la eliminación de mi cuenta y de mis datos personales,
**Para que** pueda ejercer mi derecho de supresión de datos conforme a la normativa de protección de datos personales.

### Criterios de Aceptación
- El sistema debe permitir solicitar la eliminación de la cuenta desde la configuración del perfil (derecho al olvido).
- El sistema debe verificar previamente que no existan reservas activas, pagos pendientes ni disputas abiertas.
- Si existen procesos activos, el sistema debe rechazar la solicitud e informar el motivo y las condiciones para reintentarla.
- Cuando la solicitud es aprobada, el sistema debe eliminar de forma irreversible los datos personales identificables y anonimizar la información transaccional histórica.
- El sistema debe cerrar la sesión activa del usuario al completar la eliminación.
- La solicitud y su resolución deben quedar registradas en la auditoría del sistema.

> **Fuera del alcance de la ES1:** eliminación selectiva de datos individuales (por ejemplo, solo el teléfono) y portabilidad de datos a otro prestador.

# Épica E2 — Gestión de Espacios

## HU05 - Publicar un espacio físico

**Épica:** E2 · **Rol (Como):** Arrendador · **Prioridad:** Alta
**RF:** RQF-060 a RQF-085 · **CU:** CU-15, CU-16, CU-17

**Como** arrendador con identidad verificada,
**Quiero** registrar y publicar los detalles de mi espacio físico (quinchos, bodegas, estacionamientos, parcelas u otros),
**Para que** los interesados puedan visualizarlo, conocer sus características y solicitar su arriendo de forma digital.

### Criterios de Aceptación
- El formulario de publicación debe estar estructurado en secciones: Información General, Ubicación, Tarifas y Multimedia.
- El sistema debe bloquear el acceso al panel de publicación a usuarios cuya identidad no esté verificada (KYC/KYB aprobado).
- El título del espacio debe cumplir un máximo de **70 caracteres**; si lo excede, el sistema debe mostrar *"El título no puede exceder los 70 caracteres"*.
- La descripción del espacio debe contener al menos 100 caracteres.
- El sistema debe permitir seleccionar el tipo de inmueble desde una lista predefinida, ingresar la superficie en metros cuadrados (mayor a cero) y establecer la capacidad máxima de personas.
- El sistema debe permitir configurar la modalidad tarifaria (por hora, día o mes) y el precio base, validando que sea superior a **$5.000 CLP**.
- En la sección de ubicación, el sistema debe exigir la dirección completa y registrar la ubicación geográfica a partir de ella.
- En multimedia, el sistema debe permitir cargar hasta **10 fotografías** en formatos de imagen válidos, cada una de máximo **5 MB**, y permitir seleccionar la portada y eliminar imágenes.
- Si faltan campos obligatorios o algún valor no cumple las validaciones, el sistema debe resaltar los campos con error e impedir el guardado.
- Al confirmar el formulario, el sistema debe guardar la publicación primero en estado "Borrador" y luego cambiarla a **"Activa"**, mostrar el mensaje *"Su espacio ha sido publicado con éxito"* y generar el calendario de disponibilidad asociado.

> *(criterio de interfaz)* La pantalla de éxito debe incluir un botón "Ver mi publicación".

> **Fuera del alcance de la ES1:** publicación asistida con migración automática de anuncios desde otras plataformas.

## HU06 - Editar y actualizar información de un espacio

**Épica:** E2 · **Rol (Como):** Arrendador · **Prioridad:** Alta
**RF:** RQF-086, RQF-087, RQF-195 a RQF-198, RQF-221 · **CU:** CU-18

**Como** arrendador,
**Quiero** modificar los datos, la descripción, las fotografías o las tarifas de un anuncio publicado,
**Para que** la información que ven los arrendatarios refleje siempre las condiciones reales del inmueble.

### Criterios de Aceptación
- El panel del arrendador debe listar sus publicaciones con una acción de "Editar" en cada tarjeta.
- Al seleccionar "Editar", el sistema debe cargar el formulario precargado con los datos vigentes de la publicación.
- El sistema debe permitir modificar el título, la descripción, el precio base, las reglas de uso, la capacidad máxima y la modalidad tarifaria, además de agregar o eliminar fotografías de la galería.
- El sistema debe validar que el precio base modificado sea mayor a **$5.000 CLP**; si no lo es, debe mostrar *"La tarifa actualizada debe ser mayor a $5.000"* y bloquear la actualización.
- Al guardar, el sistema debe solicitar confirmación (*"¿Está seguro de actualizar la información de este espacio?"*), actualizar los registros y mostrar *"Anuncio actualizado correctamente"*, redirigiendo al listado de espacios.
- El sistema debe rechazar la eliminación permanente de una publicación que posea reservas futuras pendientes.

> **Fuera del alcance de la ES1:** edición masiva de múltiples publicaciones simultáneas y versionado histórico de cada cambio de la publicación.

## HU07 - Establecer reglas de uso y políticas del espacio

**Épica:** E2 · **Rol (Como):** Arrendador · **Prioridad:** Media
**RF:** RQF-070, RQF-196, RQF-222, RQF-223, RQF-224 · **CU:** CU-15, CU-18, CU-20

**Como** arrendador del espacio,
**Quiero** definir reglas de uso, restricciones y la política de cancelación aplicable a mi inmueble,
**Para que** los arrendatarios conozcan las condiciones antes de reservar y se proteja la integridad de mi propiedad.

### Criterios de Aceptación
- El formulario de publicación y el de edición deben incluir una sección "Reglas de uso y política de cancelación" con opciones predefinidas y un campo de texto libre para reglas personalizadas.
- El sistema debe permitir configurar la política de cancelación del espacio (Flexible, Moderada o Estricta) con la descripción de sus plazos.
- Si una regla personalizada excede los **250 caracteres**, el sistema debe mostrar *"La regla personalizada no puede superar los 250 caracteres"*.
- El sistema debe asociar cada regla y la política de cancelación al identificador de la publicación correspondiente.
- El sistema debe mostrar las reglas de uso y la política de cancelación en la vista de detalle de la publicación, de modo que el arrendatario las conozca antes de iniciar la reserva.

> **Fuera del alcance de la ES1:** reglas con validación automática por sensores o accesos físicos, y penalizaciones automáticas por incumplimiento distinto a la garantía.

## HU08 - Pausar o reactivar un espacio

**Épica:** E2 · **Rol (Como):** Arrendador · **Prioridad:** Media
**RF:** RQF-088 a RQF-092 · **CU:** CU-18

**Como** arrendador del espacio,
**Quiero** cambiar el estado de mi anuncio entre "Activa" y "Oculta",
**Para que** el espacio deje de aparecer en los resultados de búsqueda cuando no esté disponible y pueda reactivarlo después.

### Criterios de Aceptación
- El panel del arrendador debe mostrar el estado actual de cada publicación ("Activa" / "Oculta") con una acción para cambiarlo.
- Al ocultar una publicación, el sistema debe excluirla de forma inmediata de los resultados públicos y mantener intactas las reservas vigentes.
- El sistema debe permitir volver a activar una publicación en estado "Oculta", dejándola nuevamente visible en el catálogo.
- El sistema debe rechazar la **eliminación permanente** de una publicación cuando existan reservas futuras pendientes, mostrando una advertencia que explique que primero debe atenderlas.
- El sistema debe permitir eliminar permanentemente la publicación cuando no existan reservas futuras pendientes.

> *(criterio de interfaz)* La acción de ocultar debe solicitar confirmación mediante un modal (*"¿Desea ocultar este espacio del marketplace?"*).

## HU09 - Describir el equipamiento y las condiciones del espacio

**Épica:** E2 · **Rol (Como):** Arrendador · **Prioridad:** Baja
**RF:** RQF-064, RQF-070, RQF-195, RQF-196 · **CU:** CU-15, CU-18, CU-20

**Como** arrendador del espacio,
**Quiero** describir el equipamiento y las condiciones incluidas en mi espacio (proyector, parlantes, mobiliario, servicios incluidos),
**Para que** el arrendatario conozca con precisión qué encontrará en el inmueble antes de reservar.

### Criterios de Aceptación
- El sistema debe permitir detallar el equipamiento y las condiciones incluidas dentro de la descripción de la publicación y de sus reglas de uso.
- El sistema debe permitir modificar esta información en cualquier momento desde la edición de la publicación.
- El sistema debe mostrar el equipamiento y las condiciones descritas en la vista de detalle del espacio, junto con las fotografías de la galería.

> **Fuera del alcance de la ES1:** **adicionales o equipamiento extra con tarifa propia** (ítems seleccionables con costo adicional que se sumen al desglose de cobro). Si el equipo decide incorporarlos, requieren nuevos RF, un caso de uso de gestión de adicionales y la actualización del desglose de cobro (RQF-107).

# Épica E3 — Búsqueda y Selección

## HU10 - Búsqueda general de espacios

**Épica:** E3 · **Rol (Como):** Visitante / Arrendatario · **Prioridad:** Alta
**RF:** RQF-094 a RQF-096, RQF-101 · **CU:** CU-19

**Como** arrendatario,
**Quiero** buscar espacios escribiendo términos en una barra de texto (nombre, comuna o tipo de espacio),
**Para que** obtenga un listado preliminar de anuncios coincidentes.

### Criterios de Aceptación
- La plataforma debe ofrecer una barra de búsqueda accesible desde la página de inicio y la cabecera principal.
- Al confirmar la búsqueda, el sistema debe actualizar la vista mostrando las tarjetas resumen de las publicaciones coincidentes.
- El sistema debe excluir de los resultados las publicaciones sin disponibilidad en el rango de fechas consultado (cuando se hayan definido fechas).
- Si no existen coincidencias, el sistema debe mostrar una vista vacía con el mensaje *"No se encontraron resultados para '[término_buscado]', intente con otra palabra clave"*.
- La búsqueda debe estar disponible para visitantes sin sesión iniciada.

> *(criterio de interfaz)* La búsqueda puede sugerir términos mientras el usuario escribe y no debe recargar la página completa.

> **Fuera del alcance de la ES1:** buscador con indexación semántica o recomendaciones personalizadas por historial.

## HU11 - Filtrado avanzado de espacios

**Épica:** E3 · **Rol (Como):** Arrendatario · **Prioridad:** Alta
**RF:** RQF-097 a RQF-102 · **CU:** CU-19

**Como** arrendatario,
**Quiero** aplicar filtros de precio, tipo de inmueble, metros cuadrados y rango de fechas,
**Para que** los resultados se acoten a los espacios que cumplen mis requerimientos.

### Criterios de Aceptación
- El sistema debe ofrecer filtros de precio mínimo y máximo, tipo de inmueble, metros cuadrados mínimos y rango de fechas.
- El sistema debe excluir de los resultados las publicaciones sin disponibilidad dentro del rango consultado.
- El sistema debe permitir limpiar todos los filtros aplicados y volver al listado general.
- Los resultados deben actualizarse sin recargar la página completa.

> *(criterio de interfaz)* Si el usuario define un precio mínimo mayor al máximo, se debe mostrar la validación *"El precio mínimo no puede superar al precio máximo"* y deshabilitar la aplicación de filtros.

## HU12 - Visualización de espacios en mapa interactivo

**Épica:** E3 · **Rol (Como):** Arrendatario · **Prioridad:** Media
**RF:** RQF-095, RQF-096, RQF-103 · **CU:** CU-19, CU-20

**Como** arrendatario,
**Quiero** visualizar los espacios disponibles como pines dentro de un mapa interactivo,
**Para que** pueda evaluar su ubicación respecto a mi zona de interés.

### Criterios de Aceptación
- La vista de resultados debe integrar un mapa interactivo con un pin por cada publicación activa de la zona visible.
- Cada pin debe desplegar una tarjeta resumen con la fotografía de portada, el título y el precio base del espacio, con acceso directo a su detalle.
- Al desplazar o ampliar el mapa, los pines visibles deben actualizarse según el área mostrada.
- Seleccionar un pin o una tarjeta debe llevar a la vista de detalle de la publicación.

## HU13 - Visualización del detalle de un espacio

**Épica:** E3 · **Rol (Como):** Arrendatario · **Prioridad:** Alta
**RF:** RQF-103, RQF-155 · **CU:** CU-20, CU-36

**Como** arrendatario,
**Quiero** acceder a la vista detallada de un espacio (galería, descripción, reglas, equipamiento y reseñas),
**Para que** cuente con toda la información antes de decidir solicitar una reserva.

### Criterios de Aceptación
- La vista de detalle debe estar estructurada en secciones: galería de imágenes, descripción, reglas de uso y política de cancelación, equipamiento y reseñas de otros usuarios.
- El sistema debe mostrar las reseñas y calificaciones publicadas del espacio.
- El sistema debe mostrar el precio base por unidad de tiempo y habilitar la acción principal "Reservar".
- Si la publicación dejó de estar disponible (oculta o eliminada) entre la búsqueda y el acceso, el sistema debe informarlo y volver a los resultados.

> *(criterio de interfaz)* La galería debe permitir abrir las imágenes en un visor ampliado y la información de precio debe permanecer visible durante el desplazamiento.

## HU14 - Consultar el historial de reservas propias

**Épica:** E3 · **Rol (Como):** Arrendatario / Arrendador · **Prioridad:** Alta
**RF:** RQF-199 · **CU:** CU-47

**Como** usuario de la plataforma,
**Quiero** consultar el historial y el estado de las reservas asociadas a mi cuenta,
**Para que** pueda hacer seguimiento de mis arriendos y acceder a las acciones disponibles en cada caso.

### Criterios de Aceptación
- El sistema debe desplegar el historial de reservas del usuario con el estado vigente de cada una (pendiente de pago, pagada, aprobada, en firma, lista para check-in, en curso, finalizada, en disputa o cerrada).
- Al seleccionar una reserva, el sistema debe mostrar su detalle y habilitar únicamente las acciones válidas para su estado (pagar, aprobar, firmar, descargar contrato, realizar check-in o check-out, reseñar).
- Las reservas finalizadas deben mostrarse en modo de solo lectura.
- Si la cuenta no registra reservas, el sistema debe mostrar el estado vacío del historial.

> **Fuera del alcance de la ES1:** el reemplazo del panel de favoritos. **"Guardar espacios en favoritos"** no forma parte del alcance: el catálogo y la búsqueda con filtros cubren la necesidad de volver a encontrar un espacio; si el equipo lo incorpora, requiere nuevos RF y un caso de uso de favoritos.

# Épica E4 — Reservas, Pagos y Contratos

## HU15 - Selección de bloques de fecha y hora para reserva

**Épica:** E4 · **Rol (Como):** Arrendatario · **Prioridad:** Alta
**RF:** RQF-071, RQF-108 a RQF-112 · **CU:** CU-19, CU-21, CU-22

**Como** arrendatario,
**Quiero** seleccionar un rango de fechas y horarios disponibles en el calendario del espacio,
**Para que** el sistema verifique la disponibilidad y me permita avanzar con la solicitud de arriendo.

### Criterios de Aceptación
- La vista de detalle debe incluir un calendario interactivo que muestre los bloques ocupados o bloqueados y los disponibles, según la modalidad tarifaria del espacio (por hora, día o mes).
- El sistema debe validar que la fecha de inicio sea posterior a la fecha actual y que la fecha de término sea posterior a la de inicio; si no, debe mostrar *"No se pueden realizar reservas en fechas u horas pasadas"* o *"La fecha de término debe ser posterior a la fecha de inicio"*.
- Si el rango seleccionado contiene bloques reservados o bloqueados, el sistema debe mostrar *"El rango seleccionado contiene bloques de tiempo no disponibles, por favor elija otro horario"*.
- El sistema debe validar la disponibilidad del intervalo contra la base de datos al confirmar la reserva y rechazar la solicitud si detecta una colisión de último minuto.
- Al completar una selección válida, el sistema debe calcular la cantidad total de horas o días seleccionados y mostrar el resumen previo a la reserva.

## HU16 - Cálculo automático de tarifas, comisiones y garantía

**Épica:** E4 · **Rol (Como):** Arrendatario · **Prioridad:** Alta
**RF:** RQF-104 a RQF-107 · **CU:** CU-21

**Como** arrendatario,
**Quiero** visualizar el desglose de costos (estadía, comisión de servicio y garantía) antes de pagar,
**Para que** sepa con exactitud el monto total que se me cobrará.

### Criterios de Aceptación
- Al seleccionar el tiempo de uso, el sistema debe calcular el costo total multiplicando el precio base por el tiempo seleccionado.
- El sistema debe calcular y mostrar la comisión de servicio aplicada por la plataforma.
- El sistema debe calcular y mostrar el monto de la garantía definida para el espacio, indicando que es un monto retenido y no un cargo definitivo.
- El sistema debe mostrar el desglose de cobro con los ítems: Estadía, Comisión de servicio, Garantía y Total a pagar.
- Si el usuario modifica el rango de tiempo seleccionado, el sistema debe recalcular el desglose antes de continuar.

> *(criterio de interfaz)* Si ocurre un error al calcular los costos, el sistema debe mostrar *"Error al calcular los costos de la reserva, por favor intente nuevamente"* y bloquear el avance al pago.

## HU17 - Pago de la reserva mediante pasarela integrada

**Épica:** E4 · **Rol (Como):** Arrendatario · **Prioridad:** Alta
**RF:** RQF-113 a RQF-122, RQF-201, RQF-227 · **CU:** CU-22, CU-24, CU-25

**Como** arrendatario,
**Quiero** pagar la reserva a través de la pasarela de pago integrada (**Mercado Pago**),
**Para que** la transacción se procese con tokenización y estándares de seguridad del sector financiero.

### Criterios de Aceptación
- Al iniciar el pago, el sistema debe registrar la reserva en estado "Pendiente de Pago" y validar que el usuario cuente con su identidad verificada; si no la tiene, debe interrumpir el flujo y dirigirlo a la validación de identidad.
- El sistema debe permitir seleccionar el método de pago en la pasarela integrada e iniciar el cobro comunicándose con ella, sin capturar ni almacenar datos de tarjeta en la plataforma (tokenización).
- Al recibir la aprobación de la pasarela, el sistema debe cambiar el estado de la reserva a "Pagada" y mantener los fondos retenidos en custodia (Escrow) hasta que se cumplan las condiciones de liberación.
- El sistema debe ejecutar la pre-autorización por el monto de la garantía y registrar su resultado.
- Si la pasarela rechaza la transacción, el sistema debe cambiar el estado a "Cancelada_Por_Pago", notificar al arrendatario el rechazo y permitir reintentar con otro medio de pago.
- Si el pago no se completa dentro de **15 minutos**, el sistema debe cancelar automáticamente la reserva y liberar las fechas bloqueadas.
- Si el usuario cancela el pago voluntariamente, el sistema debe informar *"El proceso de pago fue cancelado por el usuario, su reserva ha quedado pendiente"* y mantener la reserva en estado pendiente hasta su expiración.
- Al procesar el pago, el sistema debe notificar al arrendatario y al arrendador sobre la nueva solicitud pagada con fondos retenidos.

> **Fuera del alcance de la ES1:** pago con transferencia bancaria manual, pago en cuotas y emisión de documentos tributarios distintos de la boleta de comisión de la plataforma.

## HU18 - Confirmación de reserva y bloqueo de calendario

**Épica:** E4 · **Rol (Como):** Sistema (automatización) · **Prioridad:** Alta
**RF:** RQF-113, RQF-116, RQF-117, RQF-121, RQF-122 · **CU:** CU-24, CU-26, CU-47

**Como** sistema de la plataforma EspaciGo,
**Quiero** procesar la respuesta exitosa de la pasarela de pago y bloquear las fechas reservadas,
**Para que** la reserva quede formalmente confirmada sin riesgo de sobreventa y las partes sean notificadas.

### Criterios de Aceptación
- Al recibir la confirmación de pago de la pasarela, el sistema debe cambiar el estado de la reserva a "Pagada" de forma automática.
- El sistema debe bloquear los bloques de fecha y hora correspondientes en el calendario del espacio, evitando superposiciones con otras reservas o bloqueos.
- El sistema debe notificar por correo al arrendatario el resultado del pago y el estado de su reserva.
- El sistema debe notificar al arrendador la recepción de una nueva solicitud pagada, indicando que los fondos permanecen **retenidos en custodia** (no acreditados) hasta que se cumplan las condiciones de liberación.
- El sistema debe dejar la reserva visible en el historial del usuario, con las acciones habilitadas según su estado.

> **Fuera del alcance de la ES1:** comprobante de reserva en PDF independiente del contrato. La constancia formal de la reserva es el contrato electrónico firmado por las partes (CU-29, CU-30).

## HU19 - Cancelación de reserva y gestión de devolución

**Épica:** E4 · **Rol (Como):** Arrendatario · **Prioridad:** Media
**RF:** RQF-117, RQF-200, RQF-223, RQF-226 a RQF-231 · **CU:** CU-51, CU-42

**Como** arrendatario,
**Quiero** cancelar una reserva conforme a la política de cancelación del espacio,
**Para que** se anule el servicio y se gestione la devolución de los fondos según corresponda.

### Criterios de Aceptación
- El historial de reservas debe ofrecer la acción "Cancelar reserva" para las reservas que cumplan los plazos de la política de cancelación del espacio.
- Si el usuario intenta cancelar fuera del plazo permitido, el sistema debe mostrar *"No es posible cancelar la reserva debido a que se encuentra fuera del plazo establecido por la política de cancelación"*.
- El sistema debe mostrar la política de cancelación aplicable y el monto estimado a devolver antes de confirmar la cancelación.
- Al confirmar, el sistema debe cambiar el estado de la reserva a "Cancelada", liberar las fechas bloqueadas y calcular el monto a reembolsar según la política configurada.
- El sistema debe ejecutar el reembolso a través de la pasarela de pago y registrar el resultado de la operación.
- El sistema debe notificar por correo a ambas partes la cancelación y el estado de la devolución de los fondos.
- Los montos retenidos que no correspondan a devolución según la política deben mantenerse en custodia hasta el cierre de la reserva.

> **Fuera del alcance de la ES1:** cancelación con reasignación automática a otro espacio, y seguros o reembolsos por causas de fuerza mayor.

## HU33 - Generar y firmar el contrato de arriendo

**Épica:** E4 · **Rol (Como):** Arrendatario / Arrendador · **Prioridad:** Alta
**RF:** RQF-130 a RQF-142, RQF-202 · **RNF:** RNF-003, RNF-014, RNF-042 · **CU:** CU-29, CU-30, CU-31, CU-32

**Como** parte de una reserva aprobada (Arrendatario o Arrendador),
**Quiero** que el contrato de arriendo se genere y se firme electrónicamente,
**Para que** la relación quede formalizada por escrito antes de usar el espacio, conforme a los requisitos de seguridad jurídica del proyecto.

### Criterios de Aceptación
- Al aprobarse la reserva, el sistema debe generar automáticamente el contrato con los datos legales de las partes y del inmueble.
- El sistema debe enviar el documento al proveedor de firma electrónica y distribuir los enlaces de firma a ambas partes por correo electrónico.
- Al confirmarse la primera firma, el estado del contrato debe pasar a "Firma_Parcial"; al completarse todas las firmas, el sistema debe almacenar el contrato final y dejar la reserva en estado "Lista_Para_Checkin".
- El sistema debe notificar a ambas partes cuando el contrato esté completamente firmado.
- Si una de las partes rechaza la firma, el sistema debe registrar el rechazo notificado por el proveedor y mantener el contrato en estado parcial hasta el cumplimiento del plazo.
- Si la fecha de inicio de la reserva se alcanza sin el contrato firmado por todas las partes, el sistema debe cancelar la reserva y reembolsar el pago al arrendatario.
- Cada parte debe poder descargar el contrato firmado asociado a su reserva.

> *(criterio de interfaz)* El usuario debe poder revisar el contrato en pantalla antes de firmar.

> **Fuera del alcance de la ES1:** firma notarial presencial, cláusulas personalizadas por el arrendador y versionado de contratos con anexos.

# Épica E5 — Disponibilidad y Calendario

## HU20 - Gestión del calendario de disponibilidad

**Épica:** E5 · **Rol (Como):** Arrendador · **Prioridad:** Alta
**RF:** RQF-083 a RQF-085, RQF-112 · **CU:** CU-17, CU-19

**Como** arrendador,
**Quiero** visualizar y gestionar el calendario de disponibilidad de mi espacio,
**Para que** pueda controlar los períodos disponibles, reservados y bloqueados.

### Criterios de Aceptación
- El panel del arrendador debe incluir un calendario mensual y semanal que muestre el estado de cada bloque (disponible, reservado u ocupado, bloqueado).
- El sistema debe permitir bloquear y desbloquear fechas u horas directamente desde el calendario.
- Si el arrendador intenta bloquear un bloque que posee una reserva confirmada vigente, el sistema debe denegar la acción e indicar el motivo.
- Los cambios de disponibilidad deben reflejarse de inmediato en el buscador de los arrendatarios.
- El sistema debe permitir navegar entre meses para consultar disponibilidad futura o pasada.

> *(criterio de interfaz)* El calendario debe distinguir los estados mediante colores y leyenda.

> **Fuera del alcance de la ES1:** sincronización con calendarios externos en formato iCal y exportación del calendario.

## HU21 - Bloqueo manual de fechas por mantenimiento o uso personal

**Épica:** E5 · **Rol (Como):** Arrendador · **Prioridad:** Media
**RF:** RQF-084, RQF-085, RQF-112, RQF-225, RQF-226 · **CU:** CU-17

**Como** arrendador,
**Quiero** bloquear un rango de fechas u horas indicando un motivo (reparaciones, uso personal),
**Para que** ningún arrendatario solicite reservas en esos períodos.

### Criterios de Aceptación
- El calendario debe ofrecer la acción "Nuevo bloqueo manual", solicitando fecha u hora de inicio, fecha u hora de término y **motivo obligatorio**.
- El sistema debe validar que la fecha de término del bloqueo sea estrictamente posterior a la de inicio; si no, debe mostrar *"La fecha de término debe ser posterior a la fecha de inicio"*.
- Si el motivo está vacío, el sistema debe resaltar el campo y mostrar *"Debe ingresar obligatoriamente un motivo para el bloqueo manual"*.
- El sistema debe verificar que el rango no se cruce con reservas existentes; si hay cruce, debe mostrar *"El rango seleccionado interfiere con una reserva activa existente"* y denegar el bloqueo.
- Al confirmar, el bloqueo debe reflejarse de inmediato en el calendario y en la disponibilidad pública del espacio.
- El sistema debe permitir desbloquear un período previamente bloqueado de forma manual.
- El registro del bloqueo debe quedar asociado a la publicación para efectos de trazabilidad y auditoría.

## HU22 - Consultar la actividad y las reservas del espacio

**Épica:** E5 · **Rol (Como):** Arrendador · **Prioridad:** Media
**RF:** RQF-199 · **CU:** CU-47

**Como** arrendador,
**Quiero** consultar las reservas de mis espacios y su estado,
**Para que** pueda hacer seguimiento de la operación de cada inmueble.

### Criterios de Aceptación
- El sistema debe permitir al arrendador consultar el historial de reservas de sus espacios con el estado vigente de cada una.
- Al seleccionar una reserva, el sistema debe mostrar su detalle, las fechas comprometidas y las acciones disponibles según su estado (aprobar, rechazar, confirmar recepción, abrir reclamo).
- El sistema debe permitir al arrendador consultar únicamente las reservas de sus propias publicaciones.

> **Fuera del alcance de la ES1:** **panel de estadísticas de ocupación y rentabilidad** (KPIs, gráficos por período y exportación de reportes del arrendador). El seguimiento operativo queda cubierto por el historial de reservas (CU-47). Si el equipo incorpora el panel de estadísticas, requiere nuevos RF y un caso de uso de reportes para el arrendador.

## HU23 - Sincronización automática de estados en el calendario

**Épica:** E5 · **Rol (Como):** Sistema (automatización) · **Prioridad:** Alta
**RF:** RQF-112, RQF-120, RQF-129, RQF-200 · **RNF:** RNF-002, RNF-006, RNF-009, RNF-011 · **CU:** CU-17, CU-22, CU-27, CU-28, CU-51

**Como** sistema de la plataforma EspaciGo,
**Quiero** actualizar automáticamente el calendario ante cada cambio del ciclo de vida de una reserva,
**Para que** no existan discrepancias entre las reservas y la disponibilidad publicada.

### Criterios de Aceptación
- Cuando una reserva pase a estado "Pendiente de Pago" o "Pagada", el sistema debe mantener o marcar los bloques correspondientes como no disponibles.
- Si una reserva "Pendiente de Pago" expira a los 15 minutos, el sistema debe cancelarla y liberar automáticamente las fechas.
- Si el arrendador no responde una solicitud en 24 horas, el sistema debe cancelarla y liberar las fechas.
- Ante la cancelación válida de una reserva, el sistema debe liberar las fechas y reflejarlo de inmediato en el calendario público.
- Si dos usuarios intentan reservar el mismo bloque de forma simultánea, el sistema debe resolverlo de manera transaccional (atomicidad) adjudicando la reserva al primer pago validado y rechazando el segundo intento.
- Cada actualización automática de disponibilidad debe quedar registrada para auditoría y reflejarse en el calendario en el menor tiempo posible (objetivo de propagación: máximo 2 segundos, conforme a RNF-001/RNF-009).

> **Fuera del alcance de la ES1:** motor de sincronización bidireccional con canales externos de venta.

## HU24 - Configuración de disponibilidad recurrente

**Épica:** E5 · **Rol (Como):** Arrendador · **Prioridad:** Baja
**RF —** (sin RF asociado) · **CU:** —

**Como** arrendador,
**Quiero** definir patrones de disponibilidad recurrente (por ejemplo, solo fines de semana o de lunes a viernes en horario vespertino),
**Para que** el sistema mantenga el calendario configurado sin bloquear día por día.

### Criterios de Aceptación
- **No implementado en la ES1.** El alcance actual cubre la disponibilidad mediante la modalidad tarifaria y el bloqueo/desbloqueo manual de fechas en el calendario (HU20, HU21).
- Si el equipo incorpora la recurrencia, debe definir: días de la semana operativos, rango horario de apertura y cierre por día, validación de hora de cierre posterior a la de apertura, aplicación masiva de bloqueos fuera del patrón, y detección de colisión con reservas confirmadas.

> **Fuera del alcance de la ES1:** horarios recurrentes, plantillas de disponibilidad por temporada y reglas automáticas de precio por demanda.

# Épica E6 — Reputación y Comunicación

## HU25 - Sistema de calificaciones y reseñas

**Épica:** E6 · **Rol (Como):** Arrendatario / Arrendador · **Prioridad:** Media
**RF:** RQF-153 a RQF-155, RQF-207, RQF-232 a RQF-235 · **CU:** CU-35, CU-36, CU-49

**Como** usuario de la plataforma (Arrendatario o Arrendador),
**Quiero** calificar con estrellas y dejar un comentario sobre la experiencia de la reserva,
**Para que** se construya una reputación confiable dentro de la comunidad de EspaciGo.

### Criterios de Aceptación
- El sistema debe habilitar la opción de calificar únicamente cuando la reserva se encuentre finalizada.
- El arrendatario debe poder registrar una calificación numérica y una reseña de texto del espacio.
- El arrendador debe poder registrar una calificación numérica y una reseña del arrendatario.
- El sistema debe exigir al menos la calificación numérica para aceptar el registro de la reseña.
- El sistema debe rechazar el registro de una segunda reseña para la misma reserva, mostrando *"Ya ha emitido una calificación para esta reserva"*.
- Al registrar una calificación, el sistema debe recalcular el promedio de calificaciones del espacio y de la contraparte evaluada.
- El sistema debe mostrar las reseñas y calificaciones del espacio en su vista de detalle.
- El sistema debe permitir al arrendador reportar una reseña publicada que incumpla las reglas, quedando disponible para la moderación del administrador.

> **Fuera del alcance de la ES1:** ventana de calificación recíproca con plazo de 14 días, límite de 500 caracteres por comentario y perfiles públicos con historial de evaluaciones del usuario.

## HU34 - Comunicarme con la contraparte durante la reserva

**Épica:** E6 · **Rol (Como):** Arrendatario / Arrendador · **Prioridad:** Baja
**RF:** RQF-156 a RQF-158 · **CU:** CU-37, CU-38

**Como** parte de una reserva,
**Quiero** intercambiar mensajes con la contraparte dentro de la plataforma,
**Para que** pueda coordinar detalles del arriendo dejando registro de la conversación.

### Criterios de Aceptación
- El sistema debe permitir al arrendatario enviar mensajes en el chat asociado a una reserva.
- El sistema debe permitir al arrendador enviar mensajes en el chat asociado a una reserva.
- Cada mensaje debe quedar asociado al hilo de la reserva correspondiente.
- Ambas partes deben poder consultar el historial de mensajes de la reserva.
- El historial de la conversación queda disponible como antecedente cuando existe una disputa sobre la reserva.
- El chat solo se habilita cuando existe una reserva entre las partes.

> **Fuera del alcance de la ES1:** adjuntar archivos en el chat, indicadores de conexión en tiempo real, y notificaciones push por cada mensaje recibido.

# Épica E7 — Administración y Moderación

## HU26 - Panel de control y supervisión del administrador

**Épica:** E7 · **Rol (Como):** Administrador · **Prioridad:** Alta
**RF:** RQF-178 a RQF-180, RQF-183 a RQF-185, RQF-212, RQF-236 · **CU:** CU-43, CU-45, CU-46

**Como** administrador del sistema,
**Quiero** acceder a un panel de control con las herramientas de gobierno de la plataforma,
**Para que** pueda supervisar la operación, gestionar usuarios, moderar contenido y auditar las acciones críticas.

### Criterios de Aceptación
- El sistema debe restringir el acceso al panel de administración a las cuentas con rol de Administrador, denegando el acceso a cualquier otro rol.
- El sistema debe permitir buscar cuentas de usuario para consultar su estado, bloquearlas o desbloquearlas, registrando obligatoriamente el motivo del bloqueo.
- El sistema debe permitir consultar el listado de espacios publicados para su supervisión.
- El sistema debe permitir generar reportes de reservas, pagos y disputas.
- El sistema debe permitir consultar el historial de auditoría de un usuario filtrando por RUT y exportar los registros consultados.
- Toda acción crítica ejecutada por el administrador debe quedar registrada en la auditoría del sistema.

> **Fuera del alcance de la ES1:** autenticación de doble factor para administradores y tablero de métricas globales en tiempo real (usuarios activos, volumen transaccionado).

## HU27 - Moderación de contenido reportado

**Épica:** E7 · **Rol (Como):** Administrador · **Prioridad:** Media
**RF:** RQF-181, RQF-182, RQF-207 · **CU:** CU-44, CU-49

**Como** administrador del sistema,
**Quiero** revisar las reseñas reportadas por los usuarios,
**Para que** pueda ocultar el contenido que incumpla las reglas de publicación.

### Criterios de Aceptación
- El sistema debe permitir al arrendador reportar una reseña publicada sobre su espacio.
- El sistema debe permitir al administrador acceder a las reseñas reportadas, junto con el motivo del reporte y el contenido afectado.
- El administrador debe poder ocultar una reseña reportada, ingresando **obligatoriamente** el motivo de la moderación.
- Al ocultar la reseña, el sistema debe dejar de mostrarla en la vista pública del espacio.
- La acción de moderación debe quedar registrada en la auditoría con el administrador responsable y la fecha.

> **Fuera del alcance de la ES1:** moderación y suspensión de **anuncios** reportados (publicaciones), bandeja de denuncias con estados y apelaciones, y notificación automática al propietario por suspensión de anuncio.

## HU28 - Resolución de disputas e incidencias de reservas

**Épica:** E7 · **Rol (Como):** Administrador · **Prioridad:** Alta
**RF:** RQF-159 a RQF-175, RQF-209, RQF-210 · **RNF:** RNF-027, RNF-028, RNF-042 · **CU:** CU-39, CU-40, CU-41, CU-42

**Como** administrador del sistema,
**Quiero** mediar en las disputas entre arrendador y arrendatario,
**Para que** pueda resolver el caso determinando la retención o devolución de la garantía.

### Criterios de Aceptación
- El sistema debe listar las disputas abiertas al administrador.
- Al abrir una disputa, el sistema debe mostrar el expediente con los antecedentes de la reserva, las fotografías del check-in y check-out, la confirmación de recepción del espacio (si existe) y los descargos presentados por el arrendatario.
- El administrador debe poder emitir un fallo a favor del arrendatario (devolución de la garantía) o a favor del arrendador (retención parcial o total), indicando el monto a deducir en este último caso.
- Si falta el monto en un fallo a favor del arrendador, el sistema debe exigir completarlo antes de cerrar el caso.
- El sistema debe ejecutar el cobro de la pre-autorización de garantía cuando el fallo exija compensación, y liberar el saldo restante al arrendatario.
- El sistema debe cambiar la disputa a estado "Resuelta" y notificar a ambas partes el resultado.
- El sistema debe impedir la apertura de una nueva disputa sobre una reserva ya cerrada.

# Épica E8 — Soporte y Notificaciones

## HU29 - Centro de ayuda y soporte

**Épica:** E8 · **Rol (Como):** Usuario de la plataforma · **Prioridad:** Baja
**RF —** (sin RF asociado) · **CU:** —

**Como** usuario de la plataforma,
**Quiero** acceder a un centro de ayuda con preguntas frecuentes y un canal de contacto,
**Para que** pueda resolver dudas o reportar problemas operativos.

### Criterios de Aceptación
- **No implementado en la ES1.** El contacto con soporte se gestiona por el correo de contacto de la plataforma.
- Si el equipo incorpora el centro de ayuda, debe definir: categorías de preguntas frecuentes, buscador predictivo, formulario de contacto con asunto y categoría, generación del ticket con código de seguimiento, notificación al usuario y gestión interna de los tickets.

> **Fuera del alcance de la ES1:** centro de ayuda con FAQ, formulario de tickets, códigos de seguimiento y bandeja de soporte con estados.

## HU30 - Notificaciones y alertas de la plataforma

**Épica:** E8 · **Rol (Como):** Usuario Registrado · **Prioridad:** Media
**RF:** RQF-047, RQF-121, RQF-122, RQF-139, RQF-149, RQF-164, RQF-194, RQF-201, RQF-210, RQF-218, RQF-231 · **RNF:** RNF-023, RNF-024 · **CU:** CU-02, CU-03, CU-24, CU-30, CU-33, CU-41, CU-50, CU-51

**Como** usuario registrado de la plataforma,
**Quiero** recibir notificaciones por correo electrónico sobre el estado de mis reservas, pagos y validaciones,
**Para que** esté informado de cada movimiento relevante de mi actividad en EspaciGo.

### Criterios de Aceptación
- El sistema debe notificar por correo electrónico: la verificación de cuenta, el resultado de la validación de identidad (KYC y KYB), el procesamiento o rechazo del pago, la recepción de nuevas solicitudes pagadas, el estado de la firma del contrato, el check-in realizado, la apertura y el resultado de una disputa, el cambio de contraseña y la cancelación de una reserva.
- Las notificaciones críticas deben enviarse de forma automática, sin intervención del usuario.
- Si el servicio de correo o el proveedor externo presenta fallas, el sistema debe reintentar el envío conforme a las políticas de resiliencia (RNF-023, RNF-024) y registrarlo para auditoría.
- El sistema debe informar al usuario en la interfaz el resultado de las operaciones críticas que generan notificación (pago, firma, disputa, cancelación).

> **Fuera del alcance de la ES1:** centro de notificaciones dentro de la aplicación (campana con contador, marcado de leídas, historial en pantalla), notificaciones push en navegador y preferencias de notificación configurables por el usuario.

# Épica E9 — Operación del Arriendo

## HU35 - Registrar check-in, check-out y confirmar la recepción

**Épica:** E9 · **Rol (Como):** Arrendatario / Arrendador · **Prioridad:** Alta
**RF:** RQF-143 a RQF-152, RQF-203 a RQF-206 · **RNF:** RNF-042 · **CU:** CU-33, CU-34, CU-48

**Como** parte de una reserva con contrato firmado,
**Quiero** registrar el check-in y el check-out del espacio con evidencia fotográfica, y confirmar la recepción al término del arriendo,
**Para que** exista un registro verificable del estado del inmueble al inicio y al final del uso.

### Criterios de Aceptación
- El sistema debe habilitar el check-in únicamente el día de inicio de la reserva y bloquearlo fuera de esa fecha.
- El arrendatario debe poder registrar el check-in cargando al menos una fotografía del estado del espacio, junto con comentarios de texto.
- El sistema debe registrar la fecha y la ubicación al procesar el check-in, cambiar la reserva al estado "En_Curso" y notificar al arrendador.
- El arrendatario debe poder registrar el check-out con al menos una fotografía del estado final del espacio.
- Al registrar el check-out, el sistema debe cambiar la reserva al estado "Finalizada" y habilitar el período de 24 horas para el registro de reclamos.
- El arrendador debe poder confirmar la recepción del espacio tras el check-out, registrándose la fecha y la ubicación de la confirmación.
- La confirmación de recepción no altera el período de gracia de 24 horas que rige la liquidación de los fondos.
- Si el arrendador detecta daños al recibir el espacio, debe registrar el reclamo dentro de las 24 horas (HU28).

> **Fuera del alcance de la ES1:** gestión domótica de accesos físicos (cerraduras inteligentes), validación biométrica de ingreso y check-in asistido por el arrendador.

## Resumen de trazabilidad de las Historias de Usuario

| HU | Épica | RF que la implementan (resumen) | CU relacionados |
|---|---|---|---|
| HU01 | E1 | RQF-001–010, 186, 187, 213 | CU-01, CU-02 |
| HU02 | E1 | RQF-010–018, 023 | CU-03, CU-04, CU-06 |
| HU03 | E1 | RQF-019–022, 214–218 | CU-05, CU-50 |
| HU04 | E1 | RQF-038–059, 192–194, 219, 220 | CU-10 … CU-14 |
| HU05 | E2 | RQF-060–085 | CU-15 … CU-17 |
| HU06 | E2 | RQF-086, 087, 195–198, 221 | CU-18 |
| HU07 | E2 | RQF-070, 196, 222–224 | CU-15, CU-18, CU-20 |
| HU08 | E2 | RQF-088–092 | CU-18 |
| HU09 | E2 | RQF-064, 070, 195, 196 | CU-15, CU-18, CU-20 |
| HU10 | E3 | RQF-094–096, 101 | CU-19 |
| HU11 | E3 | RQF-097–102 | CU-19 |
| HU12 | E3 | RQF-095, 096, 103 | CU-19, CU-20 |
| HU13 | E3 | RQF-103, 155 | CU-20, CU-36 |
| HU14 | E3 | RQF-199 | CU-47 |
| HU15 | E4 | RQF-071, 108–112 | CU-19, CU-21, CU-22, CU-23 |
| HU16 | E4 | RQF-104–107 | CU-21 |
| HU17 | E4 | RQF-113–122, 201, 227 | CU-22, CU-24, CU-25 |
| HU18 | E4 | RQF-113, 116, 117, 121, 122 | CU-24, CU-26, CU-47 |
| HU19 | E4 | RQF-117, 200, 223, 226–231 | CU-51, CU-42 |
| HU20 | E5 | RQF-083–085, 112 | CU-17, CU-19 |
| HU21 | E5 | RQF-084, 085, 112, 225, 226 | CU-17 |
| HU22 | E5 | RQF-199 | CU-47 |
| HU23 | E5 | RQF-112, 120, 129, 200 | CU-17, CU-22, CU-27, CU-28, CU-51 |
| HU24 | E5 | Sin RF (fuera de alcance) | — |
| HU25 | E6 | RQF-153–155, 207, 232–235 | CU-35, CU-36, CU-49 |
| HU26 | E7 | RQF-178–180, 183–185, 212, 236 | CU-43, CU-45, CU-46, CU-52 |
| HU27 | E7 | RQF-181, 182, 207 | CU-44, CU-49 |
| HU28 | E7 | RQF-159–175, 209, 210 | CU-39, CU-40, CU-41, CU-42 |
| HU29 | E8 | Sin RF (fuera de alcance) | — |
| HU30 | E8 | RQF-047, 121, 122, 139, 149, 164, 194, 201, 210, 218, 231 | CU-02, CU-03, CU-24, CU-30, CU-33, CU-41, CU-50, CU-51 |
| HU31 | E1 | RQF-024–033, 189–191 | CU-07, CU-08 |
| HU32 | E1 | RQF-034–037 | CU-09 |
| HU33 | E4 | RQF-130–142, 202 | CU-29, CU-30, CU-31, CU-32 |
| HU34 | E6 | RQF-156–158 | CU-37, CU-38 |
| HU35 | E9 | RQF-143–152, 203–206 | CU-33, CU-34, CU-48 |

**Cobertura:** 33 de las 35 HU quedan implementadas por requerimientos funcionales del catálogo. Las 2 restantes (HU24 y HU29) están declaradas explícitamente **fuera del alcance de la ES1**, con los criterios que deberían definirse si el equipo decide incorporarlas.

### Cobertura de los módulos del sistema por historias de usuario

| Módulo | HU que lo cubren | Estado |
|---|---|---|
| M01 — Autenticación y Gestión de Cuenta | HU01, HU02, HU03 | Cubierto |
| M02 — Perfil y Privacidad del Usuario | HU31, HU32 | Cubierto |
| M03 — Verificación de Identidad (KYC/KYB) | HU04 | Cubierto |
| M04 — Gestión de Publicaciones | HU05, HU06, HU07, HU08, HU09 | Cubierto |
| M05 — Búsqueda y Cotización de Espacios | HU10, HU11, HU12, HU13, HU15, HU16 | Cubierto |
| M06 — Reservas y Pagos (Escrow) | HU15, HU16, HU17, HU18, HU19 | Cubierto |
| M07 — Contratos y Firma Electrónica | HU33 | Cubierto |
| M08 — Check-in y Check-out | HU35 | Cubierto |
| M09 — Comunicación y Reputación | HU25, HU34 | Cubierto |
| M10 — Disputas, Payout y Facturación | HU19, HU28 | Cubierto |
| M11 — Administración y Auditoría | HU26, HU27, HU28 | Cubierto |

**Resultado: 11/11 módulos cubiertos por al menos una historia de usuario.**
