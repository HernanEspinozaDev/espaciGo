# Decisiones técnicas M04 — borradores propios

Fecha: 2026-10-05. Decisiones aplicables al corte #128/#129–#133; no sustituyen ni dan por aceptado el módulo M04 completo.

## Alcance del primer corte

El primer recorrido de M04 es crear, listar, consultar y editar el borrador completo de un espacio propio. No contempla guardar fichas incompletas: para que CU-15 tenga un resultado verificable se exigen título, descripción, superficie, categoría, capacidad, reglas, modalidad y monto base, y dirección. Las restricciones conservan los límites explícitos del requisito. No se ofrece transición a publicación, estado comercial, búsqueda pública, reserva ni cobro.

## Persistencia y categorías

Se agrega V000005 de forma incremental sobre el esquema M01–M03. `categoria_espacio` es catálogo de datos, no tabla por tipo; las categorías no dependen de KYC. `espacio.propietario_id` referencia `usuario.id` con `ON DELETE RESTRICT`. El estado solo admite `borrador` en este corte, haciendo imposible por API y DB activar accidentalmente un espacio. El catálogo está preparado para revisión cuando se congelen sus etiquetas; los nombres precisos conservan las distinciones de ES2 Anexo B y las familias INV-011: Oficina, Sala/multipropósito, Bodega, Estacionamiento, Local flexible, Stand, Quincho y Parcela/eventos.

La granularidad se resolvió con la usuaria el 2026-10-05: separar las subcategorías documentadas Oficina, Sala/multipropósito, Bodega, Estacionamiento, Local flexible, Stand, Quincho y Parcela/eventos. La elección conserva los tipos de ES2 Anexo B e INV-011 sin colapsar stand y local flexible ni excluir quincho.

El borrador guarda la dirección como texto privado. No calcula ni guarda coordenadas, no hace geocodificación y no expone una ruta de búsqueda. Esta decisión evita incorporar un servicio externo y evita hacer pública la dirección antes de un flujo de publicación autorizado. El esquema y el contrato podrán ampliarse mediante una nueva migración cuando galería, storage o ubicación tengan su propio corte.

## Ownership y autenticación

Todas las rutas de borrador requieren bearer válido y actividad de usuario conforme a M01. La única fuente de propietario es el `AccountID` resuelto por `Authorize`; el servidor ignora datos de titularidad, estado o permisos enviados por cliente (JSON estricto rechaza campos inesperados). Toda lectura y mutación aplica `propietario_id` en SQL. Un UUID inexistente y uno perteneciente a otro usuario producen el mismo 404. El rol/KYC fixture no es una concesión de permiso comercial y no existe operación de activación.

La lista de categorías requiere sesión en esta iteración, como parte del mismo contrato autenticado del mock. No contiene datos personales ni otorga autorización adicional.

## Dependencias y trabajo posterior

Dependencia real: la cuenta activa y el identificador dueño de M01 (#21–25 ya aceptadas); los endpoints existentes de perfil M02 pueden ayudar al usuario a identificar su sesión, pero ni M02 #43 ni UI KYC #51 son prerrequisitos para guardar datos privados. Por ello el nuevo subcorte puede avanzar sin cerrar #43 ni manipular bloqueos de #52. El original #52 y sus Issues #53–#61 siguen abiertos por sus restantes criterios.

Galería necesita especificación de metadatos, límites MIME/tamaño, almacenamiento privado, autorización y retención; calendario necesita reglas de ocupación e intervalos; geocodificación/publicación necesitan decidir privacidad, precisión y superficie pública. Esas dependencias se mantienen en #53–#61 y el grafo del Project; no se simulan en esta entrega.

M02 (incluidos cobro/KYC no sintéticos, foto, supresión y retención), M03 pendiente, DB02-09 y limpieza de servidor #123 no cambian con esta entrega.
