## Modelo de datos

El diseño de persistencia de ES2 cubre **el producto completo y todas las categorías de espacios** de la entrega anterior. El **Anexo B de ES2** es el diccionario oficial de diseño: define 43 tablas por módulo, cada campo con tipo, nulabilidad, clave y significado, además de relaciones, estados, restricciones y tratamiento por finalidad. Sustituye el núcleo preliminar de 17 entidades y 137 atributos como alcance del modelo. Se construirá por incrementos en PostgreSQL 18/PostGIS, manteniendo coherencia con los módulos del monolito Go de 3.3. Ni el diccionario ni las figuras prueban migraciones ejecutadas o integraciones habilitadas.

El modelo separa: (1) identidad, permisos y derechos; (2) catálogo, tarifas, cotización y calendario; (3) intentos y hechos financieros; (4) contratos, uso y disputa; (5) comunicación y reputación; y (6) auditoría, promoción y eventos analíticos. Cada módulo es dueño lógico de sus tablas; los casos de uso coordinan transacciones entre módulos sin permitir escrituras arbitrarias desde un controlador HTTP. Los importes cobrables usan CLP entero y decimal exacto; tarifa del proveedor, comisión de EspaciGo, IVA aplicable, precio del comprador y neto del arrendador son conceptos distintos.

### Identidad, catálogo y calendario

![Vista del núcleo de identidad, oferta y reservas](imagenes/figura-datos_reservas.png){width=6.3in} <!--#fig:es2-datos-reservas--> <!--#fuente:elaboración propia a partir de los requisitos de ES1 y del diccionario de ES2.-->

La figura muestra relaciones centrales; el diccionario incluye también perfil, sesión revocable, token de acción, aceptación de términos, cuenta de cobro y vínculo del vendedor con el proveedor. `usuario` admite roles coexistentes de arrendador y arrendatario. `categoria_espacio` conserva todas las clases del producto como datos; cada `espacio` referencia una categoría y representa una unidad física exclusiva. El aforo de una sala restringe asistentes y no incrementa el número de reservas simultáneas.

`regla_tarifa` y `politica_cancelacion` se versionan. La cotización apunta a las versiones usadas y registra desglose y vencimiento; no retiene inventario. La reserva conserva una instantánea del precio, comisión, IVA aplicable, garantía prevista, total del comprador y condiciones aceptadas. Un cambio posterior en una publicación no modifica ese acuerdo. El cálculo de día, mes o bloque usa la zona horaria IANA del espacio y una regla propia de la modalidad; no se convierte un mes en un número fijo de horas.

`ocupacion` unifica retenciones, reservas y bloqueos manuales. Su `tstzrange` es finito, no vacío y semiabierto `[inicio, fin)`, de modo que dos usos adyacentes no se superponen. `EXCLUDE USING gist (espacio_id WITH =, intervalo WITH &&) WHERE (activo)` impide que dos intervalos activos del mismo espacio coexistan, incluso bajo solicitudes concurrentes. La FK compuesta vincula una ocupación de reserva al mismo espacio. La expiración cambia `activo` en la misma transacción que el estado de reserva; la disponibilidad no depende únicamente de que un worker despierte a tiempo [@es2pgrangos].

### Pago, contrato, uso y disputa

![Vista del núcleo financiero](imagenes/figura-datos_finanzas.png){width=6.3in} <!--#fig:es2-datos-finanzas--> <!--#fuente:elaboración propia a partir de los requisitos de ES1 y del diccionario de ES2.-->

`reserva.estado` describe el negocio; `pago.estado` describe el intento remoto; `disputa.estado` describe la reclamación. El diccionario agrega `reserva_transicion` para registrar cada salto con actor, motivo, tiempo y versión en el mismo commit. `pago` contiene intentos idempotentes de cobro, reembolso y reverso; `evento_proveedor` deduplica y autentica webhooks; `movimiento_financiero` conserva hechos confirmados sin sobrescribirlos. `garantia` representa el importe previsto y solo atribuye autorización/captura cuando exista confirmación del proveedor. `liquidacion` registra el neto observado y queda bloqueada con disputa abierta. El escenario económico del Anexo A usa comisión **3 % neta** descontada al arrendador y tarifa pública del proveedor descontada primero al mismo vendedor; el comprador paga el precio final publicado. El Split 1:1, su tarifa efectiva y su capacidad de retener/liberar fondos deben verificarse en la investigación separada de Mercado Pago; el modelo no presupone custodia ni una merma porcentual fija.

![Vista del núcleo de contrato, uso y disputas](imagenes/figura-datos_operacion.png){width=6.3in} <!--#fig:es2-datos-operacion--> <!--#fuente:elaboración propia a partir de los requisitos de ES1 y del diccionario de ES2.-->

Contrato versiona plantilla y contenido final; `firma_contrato` vincula firmantes de la reserva con evidencias de proveedor. Check-in y check-out requieren un acto autorizado y evidencia según el requisito, no un avance automático por reloj. El flujo completo incorpora aprobación del arrendador, firmas, uso, reclamación, cierre y cancelaciones. La aplicación valida transiciones y permisos; los `CHECK` de PostgreSQL delimitan literales, pero no prueban por sí solos que un salto de estado sea válido.

![Vista del núcleo de documentos y auditoría](imagenes/figura-datos_evidencias.png){width=6.3in} <!--#fig:es2-datos-evidencias--> <!--#fuente:elaboración propia a partir de los requisitos de ES1 y del diccionario de ES2.-->

`documento` guarda bucket privado, clave, generación, MIME detectado, tamaño, hash y una FK propietaria entre los recursos admitidos. El binario permanece en Cloud Storage. Una URL firmada es temporal y no se conserva como referencia permanente; el hash permite comparar contenido, pero la garantía de inmutabilidad de RNF-017 requiere el mecanismo de retención y su ensayo propio.

### Comunicación, promoción y analítica

`mensaje_reserva` solo es visible a participantes. `resena` se vincula a una reserva elegible y `reporte_resena` registra moderación motivada. `notificacion` conserva la intención, mientras `entrega_notificacion` registra canal y reintentos. Campaña, orden de promoción y derecho de reporte se diseñan desde ahora, pero su venta depende de precio y condiciones comerciales aprobados. Las impresiones y clics de alto volumen se publican a Pub/Sub y se agregan en BigQuery; no se inserta una fila operativa por vista. PostgreSQL determina titularidad y vigencia del derecho antes de servir métricas. `respuesta_nps` es voluntaria y su texto libre no se exporta íntegro a analítica.

`outbox_evento` se inserta junto con el hecho de dominio en una transacción local y el publicador del monolito lo reclama con lease, reintenta y envía a Pub/Sub. La entrega puede repetirse; el consumidor deduplica por ID. Los logs técnicos, los eventos analíticos y `evento_auditoria` tienen finalidades y retenciones distintas. BigQuery no es la fuente de verdad de pagos, contratos, disputas ni derechos.

### Protección de datos desde el inicio

**La Ley 21.719 es criterio de diseño desde el primer incremento de ES2**, como ya decidió el usuario, aunque su entrada en vigencia sea posterior a esta entrega. Antes de crear tablas/endpoints se identifica finalidad, dato mínimo, acceso, encargado, ubicación, plazo por categoría y acción al cierre; se implementan permisos por recurso, objetos privados, registro de solicitudes y caminos de acceso, rectificación, supresión, oposición, portabilidad y bloqueo. PT-16 comprobará estos mecanismos cuando exista producto [@ley21719].

La matriz del Anexo B organiza los tratamientos por finalidad. RNF-042/043 conservan sus mínimos de cinco años para contratos/evidencias y auditoría, pero no se convierten en un plazo universal para perfil, KYC, chat o telemetría. RNF-026 mantiene la meta interna de 72 horas sin atribuirla a un plazo legal no verificado. Conservar un UUID y reemplazar nombre/correo por hash puede dejar datos vinculables; cada solicitud exige decisión fundada y revisión de copias, objetos y derivados. Las bases jurídicas exactas, los plazos legales aplicables y el bloqueo irreversible de retención requieren validación competente antes de automatizar eliminación o inmovilización definitiva.

### Construcción y evidencia

El diccionario es un **contrato de diseño**, no SQL ejecutado. Las migraciones iniciales crearán catálogos, claves, roles mínimos y trazabilidad; la restricción `EXCLUDE` se incorpora con la tabla de ocupación, antes de habilitar reservas. Las migraciones siguientes incorporarán los demás módulos sin quebrar contratos ya aceptados. El backend aplicará `pgx/pgxpool` y `sqlc` con transacciones explícitas. Los ensayos MD-01–MD-13 y PT asociados medirán exclusión concurrente, coherencia de estados, idempotencia, acceso, privacidad, migración y restauración en PostgreSQL real. Hasta contar con versión de motor, migración, entorno y resultado fechados, esos controles siguen **previstos**.

[[PENDIENTE: ejecutar las migraciones y MD/PT con evidencia; validar con contador y proveedor los conceptos tributarios y capacidades de pago; aprobar bases jurídicas y plazos por finalidad antes de automatizar políticas irreversibles.]]
