# Anexo B. Diccionario de datos y contrato lógico de ES2

**Versión de diseño:** 2.0, 29-09-2026. **Estado:** definición arquitectónica para construir el producto completo; ninguna tabla, migración, instancia o ensayo se declara implementado. Sustituye el núcleo preliminar de 17 entidades y 137 atributos como especificación de diseño. El SQL ilustrativo anterior queda como antecedente en el historial de Git y no debe ejecutarse como migración vigente. La fuente ejecutable futura será `db/migrations/` del backend Go; este diccionario será su contrato académico. PostgreSQL 18 + PostGIS + `btree_gist` es el objetivo, sujeto a verificación local y en Cloud SQL.

La Ley 21.719 se aplica **como criterio de diseño desde el primer incremento ES2**, por decisión del usuario. Su entrada en vigencia, prevista para el 1 de diciembre de 2026, no retrasa minimización, permisos, inventario de tratamientos, derechos de titulares ni controles de seguridad del desarrollo. Tampoco se declara cumplimiento probado sin implementación y evidencia [@ley21719].

## B.1 Convenciones y decisiones vinculantes de diseño

- Cada `espacio` es una **unidad reservable exclusiva**. Todas las categorías de la entrega anterior están representadas en `categoria_espacio`; agregar una categoría es un cambio de datos y reglas, no una migración de tabla. El aforo limita asistentes y no permite reservas simultáneas.
- `uuid` identifica hechos y recursos internos; `timestamptz` registra instantes; `tstzrange` con límites `[)` representa ocupación. La zona IANA del espacio interpreta días, meses, horarios y cambios de hora. Un intervalo de ocupación es finito y no vacío.
- CLP se almacena en pesos enteros `numeric(14,0)`; porcentajes/cálculos intermedios en `numeric` de escala declarada, nunca `float`. Toda operación financiera lleva `moneda char(3)`. El redondeo aritmético de importes se versiona y se reconcilia con documentos/proveedor; la regla legal chilena de redondeo de efectivo no se aplica automáticamente a pagos electrónicos.
- `!` significa `NOT NULL`; `?` significa nullable. Toda FK de hechos históricos usa `ON DELETE RESTRICT` o `NO ACTION`; no hay borrado en cascada de reservas, pagos, contratos, evidencias, auditoría ni solicitudes de derechos. PK/UK/CHECK/EXCLUDE se materializan en migraciones; autorización y transiciones se validan además en casos de uso.
- Clases de dato: **P** personal vinculable, **R** restringido (identidad, finanzas, geolocalización precisa, texto privado), **O** operacional, **T** técnico, **A** analítico minimizado. Una clase no define por sí sola base jurídica ni plazo. La tabla B.5 establece tratamiento por finalidad. `uuid` puede seguir siendo dato personal vinculable.
- Los campos `*_ref` son identificadores externos, no secretos; tokens de proveedor se guardan mediante referencia a Secret Manager. Fotografías, PDF y documentos KYC residen en Cloud Storage privado; PostgreSQL conserva metadatos, FK, hash y generación del objeto, nunca URLs firmadas persistentes.
- La comisión de EspaciGo queda como regla versionada inicial de **3 % neto** del arriendo publicado y, en el escenario vigente, se descuenta al arrendador por Split 1:1 junto con el IVA de esa comisión cuando corresponda. La tarifa del proveedor y su IVA se descuentan primero al vendedor según el flujo público; no se registran como costo/crédito fiscal de EspaciGo sin documento y acuerdo distintos. El comprador paga el precio final publicado en la simulación; un cargo adicional exige cotización y aceptación separadas. Tarifa, comisión, IVA, precio y neto observado tienen campos distintos. La cifra aproximada de 7 % no es constante de datos. El reparto entre socios de la SpA ocurre fuera del Split 1:1 y de la reserva.
- Los módulos Go son dueños lógicos de las tablas; la API usa `pgx/pgxpool` y SQL revisable con `sqlc`. Los workers son goroutines del mismo monolito y reclaman trabajo durable. `outbox_evento` se publica a Pub/Sub/BigQuery; BigQuery no decide estados operativos. La telemetría de impresiones/clics de alto volumen viaja a Pub/Sub y no crea una fila PostgreSQL por vista.

**Notación del diccionario:** en las tablas siguientes, `PK`, `FK→tabla.campo` y `UK` indican claves; `CHECK` indica dominio o relación local. Cada campo tiene tipo y nulabilidad. Los valores de catálogo se fijan aquí como contrato de diseño; cambiar su semántica exige migración o versión de API. Los campos de clase P/R quedan sujetos al acceso del titular, participante autorizado o rol administrativo con finalidad documentada; los T/A no admiten payload libre con PII.

## B.2 Identidad, consentimiento y habilitación de vendedores (M01–M03)

### `usuario` — cuenta y credencial (P/R; RQF-001–023)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK, identificador interno estable. |
| `correo_normalizado` | text ! | UK sobre normalización/casefold; acceso y avisos; P. |
| `hash_clave` | text ! | Hash de contraseña con algoritmo y parámetros versionados; R; nunca contraseña. |
| `estado` | text ! | CHECK `correo_pendiente`, `activo`, `bloqueado`, `baja_solicitada`, `desidentificado`. |
| `creado_en` | timestamptz ! | Alta de cuenta. |
| `actualizado_en` | timestamptz ! | Último cambio de cuenta. |
| `baja_solicitada_en` | timestamptz ? | Inicio de trámite, no dispara borrado indiscriminado. |

### `rol_usuario` — roles coexistentes (O; RQF-213)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `usuario_id` | uuid ! | PK compuesta, FK→`usuario.id`. |
| `rol` | text ! | PK compuesta; CHECK `arrendatario`, `arrendador`, `administrador`; administración se concede por proceso auditado. |
| `concedido_en` | timestamptz ! | Trazabilidad de habilitación. |
| `concedido_por` | uuid ? | FK→`usuario.id`; nulo para rol básico automático. |

### `perfil_usuario` — presentación e identidad mínima (P; RQF-024–031)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `usuario_id` | uuid ! | PK/FK→`usuario.id`; relación 1:1. |
| `nombre_visible` | text ! | Nombre mostrado, límite de longitud en API y DDL. |
| `telefono_normalizado` | text ? | Teléfono de contacto verificado si se usa; P. |
| `razon_social` | text ? | Solo arrendador persona jurídica. |
| `identificador_fiscal_cifrado` | bytea ? | Solo cuando el flujo tributario lo exige; acceso restringido, índice por huella separada si se requiere. |
| `actualizado_en` | timestamptz ! | Control de vigencia del perfil. |

### `sesion` — autenticación revocable (R/T; RQF-018/023)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK; identificador de sesión. |
| `usuario_id` | uuid ! | FK→`usuario.id`. |
| `token_hash` | char(64) ! | UK; nunca token en claro. |
| `creada_en` | timestamptz ! | Inicio. |
| `expira_en` | timestamptz ! | Mayor que `creada_en`. |
| `revocada_en` | timestamptz ? | Cierre o revocación. |
| `cliente_resumen` | text ? | Huella técnica mínima sin agente/IP completos persistentes por defecto. |

### `token_accion` — correo y recuperación de cuenta (R/T; RQF-008/019–022)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `usuario_id` | uuid ! | FK→`usuario.id`. |
| `proposito` | text ! | CHECK `verificar_correo`, `recuperar_clave`, `cambiar_correo`. |
| `token_hash` | char(64) ! | UK; token de un solo uso. |
| `creado_en` | timestamptz ! | Emisión. |
| `expira_en` | timestamptz ! | Mayor que `creado_en`. |
| `consumido_en` | timestamptz ? | Impide reutilización. |
| `intentos` | integer ! | CHECK `>=0`; límite en caso de uso. |

### `version_terminos` y `aceptacion_terminos` — texto aceptado (O/P; RQF-186–187)

| Tabla.campo | Tipo | Regla y significado |
| --- | --- | --- |
| `version_terminos.id` | uuid ! | PK. |
| `version_terminos.codigo` | text ! | UK; versión pública. |
| `version_terminos.tipo` | text ! | CHECK `terminos`, `privacidad`, `politica_arrendador`. |
| `version_terminos.hash_sha256` | char(64) ! | Digest del texto exacto publicado. |
| `version_terminos.publicada_en` | timestamptz ! | Inicio de disponibilidad. |
| `aceptacion_terminos.id` | uuid ! | PK. |
| `aceptacion_terminos.usuario_id` | uuid ! | FK→`usuario.id`. |
| `aceptacion_terminos.version_id` | uuid ! | FK→`version_terminos.id`; UK con usuario si solo una aceptación por versión. |
| `aceptacion_terminos.aceptada_en` | timestamptz ! | Acto afirmativo/contractual registrado. |
| `aceptacion_terminos.canal` | text ! | CHECK `web`, `api`, `administrado`; evidencia de origen. |

### `verificacion` — KYC/KYB y revisión (R; RQF-038–060/192–194)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `usuario_id` | uuid ! | FK→`usuario.id`; titular. |
| `tipo` | text ! | CHECK `kyc`, `kyb`. |
| `estado` | text ! | CHECK `pendiente`, `en_revision`, `aprobada`, `rechazada`, `vencida`. |
| `proveedor_ref` | text ? | Referencia externa, no credencial. |
| `revisor_id` | uuid ? | FK→`usuario.id`; administrador cuando revisión manual. |
| `motivo_codigo` | text ? | Obligatorio en rechazo. |
| `creada_en` | timestamptz ! | Solicitud. |
| `resuelta_en` | timestamptz ? | Obligatoria en estado final. |

### `cuenta_cobro` y `vinculo_proveedor_vendedor` — recepción del arrendador (R; RQF-032–033/189–191)

| Tabla.campo | Tipo | Regla y significado |
| --- | --- | --- |
| `cuenta_cobro.id` | uuid ! | PK. |
| `cuenta_cobro.arrendador_id` | uuid ! | FK→`usuario.id`; rol validado. |
| `cuenta_cobro.proveedor` | text ! | Adaptador usado. |
| `cuenta_cobro.cuenta_token_ref` | text ? | Token/referencia externa, nunca número bancario completo en claro. |
| `cuenta_cobro.estado` | text ! | CHECK `pendiente`, `validada`, `suspendida`, `revocada`. |
| `cuenta_cobro.validada_en` | timestamptz ? | Evidencia de verificación. |
| `vinculo_proveedor_vendedor.id` | uuid ! | PK. |
| `vinculo_proveedor_vendedor.arrendador_id` | uuid ! | FK→`usuario.id`. |
| `vinculo_proveedor_vendedor.proveedor` | text ! | UK por arrendador/proveedor. |
| `vinculo_proveedor_vendedor.vendedor_ref` | text ! | Identificador externo; UK con proveedor. |
| `vinculo_proveedor_vendedor.secreto_ref` | text ! | Ruta/version de secreto en Secret Manager, sin token en BD. |
| `vinculo_proveedor_vendedor.estado` | text ! | CHECK `pendiente`, `activo`, `vencido`, `revocado`. |
| `vinculo_proveedor_vendedor.actualizado_en` | timestamptz ! | Última autorización/verificación. |

## B.3 Oferta, precio, reserva y calendario (M04–M06)

### `categoria_espacio` — catálogo de todos los tipos del producto (O; RQF-068/098)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `codigo` | text ! | PK estable; se cargan oficinas, salas/multipropósito, bodegas, estacionamientos, locales/stands, quinchos y parcelas/eventos; la lista exacta de etiquetas se conserva en datos versionados. |
| `nombre` | text ! | Etiqueta pública. |
| `descripcion` | text ! | Alcance de la categoría. |
| `activa` | boolean ! | Desactivar no elimina espacios históricos. |
| `version_definicion` | integer ! | CHECK `>0`; seguimiento de reglas de categoría. |
| `orden` | integer ! | Orden de presentación; sin semántica financiera. |

### `espacio` — una publicación, una unidad exclusiva (P/O; RQF-060–093/195–198/221–224)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `arrendador_id` | uuid ! | FK→`usuario.id`; dueño autorizado. |
| `categoria_codigo` | text ! | FK→`categoria_espacio.codigo`. |
| `titulo` | text ! | Nombre público. |
| `descripcion` | text ! | Texto moderado. |
| `direccion_privada` | text ! | Dirección exacta; R, no exposición en búsqueda general. |
| `ubicacion` | geography(Point,4326) ! | Coordenada precisa restringida; búsqueda pública puede devolver zona general. |
| `zona_horaria` | text ! | Nombre IANA validado, sin asumir Santiago para todas las publicaciones. |
| `superficie_m2` | numeric(12,2) ? | CHECK `>0` cuando aplica. |
| `aforo_personas` | integer ? | CHECK `>0` cuando aplica; no es cupo de reservas. |
| `reglas_uso` | text ! | Condiciones publicadas; versión aceptada se congela en reserva. |
| `estado` | text ! | CHECK `borrador`, `activa`, `oculta`, `suspendida`. |
| `creado_en` | timestamptz ! | Alta. |
| `actualizado_en` | timestamptz ! | Última edición. |

### `politica_cancelacion` — condiciones versionadas (O; RQF-223/228–231)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `codigo` | text ! | Familia de política. |
| `version` | integer ! | UK con `codigo`; CHECK `>0`. |
| `texto_publicado` | text ! | Texto exacto mostrado. |
| `vigente_desde` | timestamptz ! | Inicio. |
| `vigente_hasta` | timestamptz ? | Fin exclusivo; no sobrescribir versión aceptada. |

### `tramo_cancelacion` — reglas estructuradas de devolución (O; RQF-228–230)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `politica_id` | uuid ! | FK→`politica_cancelacion.id`. |
| `anticipacion_min_horas` | integer ! | CHECK `>=0`; inicio inclusivo de la ventana previa al arriendo. |
| `anticipacion_max_horas` | integer ? | Fin exclusivo, mayor que mínimo; nulo para tramo superior abierto. |
| `porcentaje_reembolso` | numeric(5,4) ! | CHECK entre 0 y 1; aplicado a conceptos definidos en la política. |
| `orden` | integer ! | UK con `politica_id`; ventana/precedencia documentada. |

Los tramos de una versión publicada no se solapan y cubren los momentos cancelables definidos por la política. Se congelan el texto y la versión aceptados en la reserva; el cálculo se realiza sobre esos tramos, no sobre la política vigente al pedir la devolución.

### `regla_tarifa` — precio publicado por unidad temporal (O; RQF-071–073/104)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `espacio_id` | uuid ! | FK→`espacio.id`. |
| `version` | integer ! | UK con `espacio_id`; CHECK `>0`. |
| `unidad` | text ! | CHECK `hora`, `dia`, `semana`, `mes`, `anio`, `bloque`; cada una tiene cálculo civil documentado. |
| `precio_clp` | numeric(14,0) ! | CHECK `>5000` conforme RQF-073 mientras ese requisito rija. |
| `duracion_minima` | integer ! | CHECK `>0`; unidades de la regla, no conversión tácita de meses a horas. |
| `duracion_maxima` | integer ? | CHECK `>= duracion_minima`. |
| `politica_id` | uuid ! | FK→`politica_cancelacion.id`. |
| `vigencia` | tstzrange ! | Inicio finito, `[)`; fin abierto permitido hasta reemplazo, sin solape para el mismo espacio/ámbito; versión inmutable. |
| `estado` | text ! | CHECK `borrador`, `publicada`, `retirada`. |
| `publicada_en` | timestamptz ? | Obligatoria cuando publicada. |

### `regla_comision` — parámetro comercial versionado (O; RQF-105; RNF-020)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `version` | integer ! | UK por ámbito, CHECK `>0`. |
| `categoria_codigo` | text ? | FK→`categoria_espacio.codigo`; nulo significa regla general. |
| `porcentaje_neto` | numeric(7,5) ! | Inicial `0.03000`; CHECK entre 0 y 1; es hipótesis comercial versionada, no tasa de proveedor. |
| `base_calculo` | text ! | CHECK `precio_arriendo`; no incluye garantía. |
| `vigencia` | tstzrange ! | Inicio finito, `[)`; fin abierto permitido; una regla aplicable por categoría/fecha según prioridad explícita. |
| `motivo` | text ! | Justificación del cambio. |
| `aprobador_id` | uuid ? | FK→`usuario.id`; cambio administrativo auditado. |

### `cotizacion` — cálculo reproducible, sin reservar inventario (P/O; RQF-104–110)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `espacio_id` | uuid ! | FK→`espacio.id`. |
| `arrendatario_id` | uuid ? | FK→`usuario.id`; nulo para cotización pública sin PII adicional. |
| `tarifa_id` | uuid ! | FK→`regla_tarifa.id`; mismo espacio validado por FK compuesta o caso de uso. |
| `comision_id` | uuid ! | FK→`regla_comision.id`. |
| `intervalo` | tstzrange ! | Futuro, finito, `[)` y compatible con modalidad. |
| `precio_arriendo_clp` | numeric(14,0) ! | Precio final de arriendo publicado y aceptado; no negativo. La posible fracción tributaria del arrendador se identifica aparte, sin sumarla dos veces. |
| `iva_arriendo_clp` | numeric(14,0) ! | Cero o porción de IVA contenida en el precio final cuando corresponda al arrendador; documento/emisor se validan aparte. |
| `comision_neta_clp` | numeric(14,0) ! | Comisión según regla/snapshot; inicialmente descontada al vendedor. |
| `iva_comision_clp` | numeric(14,0) ! | Cero o IVA de esa comisión, también descontado al vendedor en el escenario vigente. |
| `cargo_proveedor_comprador_clp` | numeric(14,0) ! | Cero o cargo trasladado al comprador si el checkout/precio lo admite y lo muestra. |
| `iva_cargo_proveedor_comprador_clp` | numeric(14,0) ! | IVA del cargo anterior si corresponde; no se infiere de la tarifa neta. |
| `garantia_prevista_clp` | numeric(14,0) ! | Separada del cobro efectivo; no negativa. |
| `total_comprador_clp` | numeric(14,0) ! | Importe exigible al comprador según la incidencia de cargos mostrada; no incorpora por defecto comisión deducida al vendedor ni garantía solo prevista. |
| `calculo_version` | text ! | Versión de algoritmo y redondeo usada en la cotización. |
| `moneda` | char(3) ! | `CLP` en fase inicial. |
| `creada_en` | timestamptz ! | Inicio de validez. |
| `expira_en` | timestamptz ! | Mayor que `creada_en`; consulta posterior recalcula si cambió regla. |

### `reserva` — acuerdo y estado de negocio (P/R; RQF-104–129/228–231)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `espacio_id` | uuid ! | FK→`espacio.id`; UK auxiliar `(id,espacio_id)` para coherencia de ocupación. |
| `arrendatario_id` | uuid ! | FK→`usuario.id`; no se borra el hecho contractual por baja de cuenta. |
| `cotizacion_id` | uuid ! | FK→`cotizacion.id`; UK para impedir uso duplicado. |
| `tarifa_id` | uuid ! | FK→`regla_tarifa.id`; versión aceptada. |
| `comision_id` | uuid ! | FK→`regla_comision.id`; versión aceptada. |
| `intervalo` | tstzrange ! | Finito, `[)`; igual al de su ocupación al crear. |
| `estado` | text ! | Catálogo B.6; inicial `pendiente_de_pago`. |
| `precio_arriendo_clp` | numeric(14,0) ! | Snapshot inmutable del precio final publicado. |
| `iva_arriendo_clp` | numeric(14,0) ! | Porción tributaria del arriendo contenida en el precio final si aplica; separada del IVA de servicio. |
| `comision_neta_clp` | numeric(14,0) ! | Snapshot de comisión EspaciGo. |
| `iva_comision_clp` | numeric(14,0) ! | Snapshot separado, según afectación confirmada. |
| `cargo_proveedor_comprador_clp` | numeric(14,0) ! | Snapshot de cargo trasladado al comprador si existe. |
| `iva_cargo_proveedor_comprador_clp` | numeric(14,0) ! | Snapshot de IVA del cargo trasladado si existe. |
| `garantia_prevista_clp` | numeric(14,0) ! | No es captura ni ingreso. |
| `total_comprador_clp` | numeric(14,0) ! | Snapshot del importe exigible al comprador; garantía prevista y descuentos al vendedor se concilian aparte. |
| `calculo_version` | text ! | Versión de algoritmo y redondeo usada al aceptar. |
| `moneda` | char(3) ! | Igual en cotización/pago; inicial CLP. |
| `condiciones_snapshot` | text ! | Política de cancelación y reglas de uso aceptadas; hash/versión si se almacena objeto. |
| `creada_en` | timestamptz ! | Alta. |
| `expira_pago_en` | timestamptz ! | Plazo de retención, inicialmente 15 min según RQF-120. |
| `version` | integer ! | Control optimista; aumenta con cada transición. |

### `ocupacion` — calendario único (O; RQF-083–085/111–112/120)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `espacio_id` | uuid ! | FK→`espacio.id`. |
| `reserva_id` | uuid ? | FK compuesta `(reserva_id,espacio_id)`→`reserva(id,espacio_id)`; UK para ocupación de reserva. Nulo para bloqueo manual. |
| `intervalo` | tstzrange ! | Finito, no vacío y `[)`; inicio anterior a fin. |
| `tipo` | text ! | CHECK `retencion`, `reserva`, `bloqueo_manual`; coherente con nulabilidad de reserva. |
| `activo` | boolean ! | Participa en `EXCLUDE USING gist (espacio_id WITH =, intervalo WITH &&) WHERE (activo)`. |
| `expira_en` | timestamptz ? | Obligatoria para `retencion`; sin `now()` en predicado de índice. |
| `motivo` | text ? | Obligatorio para bloqueo manual. |
| `creada_en` | timestamptz ! | Trazabilidad. |
| `desactivada_en` | timestamptz ? | Transición de liberación; consistente con `activo=false`. |

### `reserva_transicion` — historia de estados (P/O; RQF-113–177)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `reserva_id` | uuid ! | FK→`reserva.id`. |
| `version_anterior` | integer ! | CHECK `>=0`. |
| `version_nueva` | integer ! | UK con `reserva_id`; debe ser `version_anterior+1`. |
| `desde` | text ? | Nulo solo para creación. |
| `hacia` | text ! | Estado B.6. |
| `actor_id` | uuid ? | FK→`usuario.id`; nulo para worker/proveedor. |
| `actor_tipo` | text ! | CHECK `usuario`, `worker`, `proveedor`, `administrador`. |
| `motivo_codigo` | text ! | Causa trazable, sin texto PII libre. |
| `ocurrio_en` | timestamptz ! | Instante del cambio. |
| `correlacion_id` | uuid ! | Une petición, pago, evento y auditoría. |

## B.4 Pagos, contratos, operación y comunicación (M06–M10)

### `pago` — intento idempotente de cobro o reverso (R; RQF-114–120/128/230)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `reserva_id` | uuid ? | FK→`reserva.id`; uno entre reserva y orden de promoción. |
| `orden_promocion_id` | uuid ? | FK→`orden_promocion.id`; XOR con `reserva_id`. |
| `proveedor` | text ! | Adaptador; `simulador` solo en entorno de prueba. |
| `tipo` | text ! | CHECK `cobro`, `reembolso`, `contracargo`, `ajuste`. |
| `clave_idempotencia` | text ! | UK `(proveedor,tipo,clave_idempotencia)`; estable entre reintentos. |
| `referencia_externa` | text ? | UK `(proveedor,referencia_externa)` cuando exista. |
| `operacion_origen_id` | uuid ? | FK→`pago.id` para reembolso/contracargo. |
| `monto_clp` | numeric(14,0) ! | CHECK `>0` para intento; no asume dinero recibido. |
| `moneda` | char(3) ! | Igual al recurso asociado. |
| `estado` | text ! | CHECK `pendiente`, `confirmado`, `rechazado`, `por_conciliar`, `revertido`. |
| `creado_en` | timestamptz ! | Intento local. |
| `confirmado_en` | timestamptz ? | Solo ante evidencia verificada/conciliada. |

### `evento_proveedor` — inbox de webhooks y conciliación (R/T; RQF-116/136–139)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `proveedor` | text ! | Origen. |
| `evento_externo` | text ! | UK con `proveedor`; deduplicación. |
| `tipo` | text ! | Tipo declarado por adaptador. |
| `pago_id` | uuid ? | FK→`pago.id`; nulo mientras se correlaciona. |
| `reserva_id` | uuid ? | FK→`reserva.id`; opcional. |
| `hash_contenido` | char(64) ! | Evidencia mínima; no almacena cuerpo completo por defecto. |
| `verificado_en` | timestamptz ? | Solo tras autenticar origen/firma con regla del proveedor. |
| `recibido_en` | timestamptz ! | Recepción. |
| `estado` | text ! | CHECK `recibido`, `procesado`, `rechazado`, `por_conciliar`, `error`. |
| `intentos` | integer ! | CHECK `>=0`. |
| `ultimo_error_codigo` | text ? | Código saneado, sin secreto. |

### `movimiento_financiero` — hecho económico inmutable (R; RQF-105–107/172–177)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `reserva_id` | uuid ? | FK→`reserva.id`; XOR con orden de promoción si aplica. |
| `orden_promocion_id` | uuid ? | FK→`orden_promocion.id`; XOR con reserva. |
| `pago_id` | uuid ? | FK→`pago.id`; hecho externo origen. |
| `tipo` | text ! | CHECK `cobro_comprador`, `tarifa_proveedor`, `iva_tarifa_proveedor`, `comision_plataforma`, `iva_comision`, `iva_arriendo`, `garantia`, `reembolso`, `contracargo`, `neto_arrendador`, `ajuste`. |
| `sentido` | text ! | CHECK `a_favor`, `en_contra` del beneficiario indicado; no es libro mayor de partida doble. |
| `monto_clp` | numeric(14,0) ! | CHECK `>0`; corrección = nueva fila inversa/ajuste, no UPDATE del hecho. |
| `moneda` | char(3) ! | Inicial CLP. |
| `beneficiario_tipo` | text ! | CHECK `arrendatario`, `arrendador`, `plataforma`, `proveedor`. |
| `proveedor_ref` | text ? | Referencia para conciliación. |
| `fuente_importe` | text ! | CHECK `cotizacion`, `webhook`, `reporte_proveedor`, `documento`, `ajuste_manual`. |
| `ocurrio_en` | timestamptz ! | Instante del hecho confirmado. |
| `registrado_en` | timestamptz ! | Inserción local; puede diferir de ocurrencia. |

### `garantia` y `liquidacion` — obligaciones y resultados observados (R; RQF-117–118/172–175/208)

| Tabla.campo | Tipo | Regla y significado |
| --- | --- | --- |
| `garantia.id` | uuid ! | PK. |
| `garantia.reserva_id` | uuid ! | FK→`reserva.id`. |
| `garantia.monto_previsto_clp` | numeric(14,0) ! | No negativo; snapshot de reserva. |
| `garantia.monto_autorizado_clp` | numeric(14,0) ? | Solo si proveedor confirma autorización real. |
| `garantia.monto_capturado_clp` | numeric(14,0) ? | Solo si proveedor confirma captura; `<= autorizado` cuando proceda. |
| `garantia.estado` | text ! | CHECK `prevista`, `solicitada`, `autorizada`, `capturada`, `liberada`, `no_disponible`, `por_conciliar`. |
| `garantia.proveedor_ref` | text ? | Correlación sin prometer escrow. |
| `garantia.actualizada_en` | timestamptz ! | Último resultado. |
| `liquidacion.id` | uuid ! | PK. |
| `liquidacion.reserva_id` | uuid ! | FK→`reserva.id`; UK por reserva, reintentos conservan misma intención. |
| `liquidacion.estado` | text ! | CHECK `pendiente`, `bloqueada_disputa`, `por_conciliar`, `confirmada`, `fallida`. |
| `liquidacion.neto_arrendador_clp` | numeric(14,0) ? | Resultado observado; no inferido del precio publicado. |
| `liquidacion.tarifa_proveedor_clp` | numeric(14,0) ? | Importe observado, separado del IVA del proveedor. |
| `liquidacion.iva_tarifa_proveedor_clp` | numeric(14,0) ? | IVA observado/documentado del cargo del proveedor. |
| `liquidacion.proveedor_ref` | text ? | Identificador externo. |
| `liquidacion.confirmada_en` | timestamptz ? | Requiere reporte/confirmación externa y ausencia de disputa abierta. |

### `documento_tributario` — respaldo de operación gravada (R; RQF-176/211)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `reserva_id` | uuid ? | FK→`reserva.id`; según documento. |
| `orden_promocion_id` | uuid ? | FK→`orden_promocion.id`; según documento. |
| `tipo` | text ! | Boleta/factura/nota según emisor y criterio fiscal confirmado. |
| `emisor_ref` | text ! | Identidad fiscal protegida/referencia. |
| `receptor_ref` | text ? | Identidad fiscal protegida/referencia, solo si necesaria. |
| `folio` | text ? | UK con emisor/tipo si existe. |
| `base_clp` | numeric(14,0) ! | Base documentada. |
| `iva_clp` | numeric(14,0) ! | IVA documentado, puede ser cero. |
| `total_clp` | numeric(14,0) ! | CHECK suma de componentes aplicables. |
| `estado` | text ! | CHECK `pendiente`, `emitido`, `anulado`, `por_conciliar`. |
| `emitido_en` | timestamptz ? | Solo con evidencia de emisión. |

### `contrato` y `firma_contrato` — documento final y participantes (R; RQF-130–142/202)

| Tabla.campo | Tipo | Regla y significado |
| --- | --- | --- |
| `contrato.id` | uuid ! | PK. |
| `contrato.reserva_id` | uuid ! | FK→`reserva.id`. |
| `contrato.version` | integer ! | UK con `reserva_id`; CHECK `>0`. |
| `contrato.plantilla_version` | text ! | Plantilla exacta usada. |
| `contrato.hash_final` | char(64) ? | Obligatorio para documento final. |
| `contrato.estado` | text ! | CHECK `generado`, `firma_parcial`, `firmado`, `anulado`, `por_conciliar`. |
| `contrato.proveedor_ref` | text ? | Sin credenciales. |
| `contrato.generado_en` | timestamptz ! | Creación. |
| `contrato.firmado_en` | timestamptz ? | Tras todas las firmas exigidas. |
| `firma_contrato.contrato_id` | uuid ! | PK compuesta, FK→`contrato.id`. |
| `firma_contrato.usuario_id` | uuid ! | PK compuesta, FK→`usuario.id`; debe ser parte de la reserva. |
| `firma_contrato.estado` | text ! | CHECK `pendiente`, `firmada`, `rechazada`, `por_conciliar`. |
| `firma_contrato.proveedor_ref` | text ? | Referencia externa de firma. |
| `firma_contrato.firmado_en` | timestamptz ? | Solo con verificación. |

### `operacion_arriendo` y `disputa` — uso y reclamación (P/R; RQF-143–171/203–210)

| Tabla.campo | Tipo | Regla y significado |
| --- | --- | --- |
| `operacion_arriendo.id` | uuid ! | PK. |
| `operacion_arriendo.reserva_id` | uuid ! | FK→`reserva.id`; UK por `tipo` cuando acto único. |
| `operacion_arriendo.actor_id` | uuid ! | FK→`usuario.id`; participante autorizado. |
| `operacion_arriendo.tipo` | text ! | CHECK `checkin`, `checkout`, `recepcion`. |
| `operacion_arriendo.ocurrio_en` | timestamptz ! | Instante real; no generado por worker solo por reloj. |
| `operacion_arriendo.ubicacion` | geography(Point,4326) ? | Precisa y R; solo si RF/permiso del acto la exige. |
| `operacion_arriendo.observacion` | text ? | Texto libre restringido. |
| `disputa.id` | uuid ! | PK. |
| `disputa.reserva_id` | uuid ! | FK→`reserva.id`; UK parcial para una disputa abierta por reserva. |
| `disputa.reclamante_id` | uuid ! | FK→`usuario.id`; cualquiera de las partes autorizadas. |
| `disputa.motivo` | text ! | Fundamento privado. |
| `disputa.estado` | text ! | CHECK `abierta`, `en_descargos`, `resuelta`, `cerrada`. |
| `disputa.abierta_en` | timestamptz ! | Ventana de reclamo según RF. |
| `disputa.resolutor_id` | uuid ? | FK→`usuario.id`; administrador. |
| `disputa.fallo` | text ? | Obligatorio al resolver. |
| `disputa.deduccion_clp` | numeric(14,0) ? | Entre cero y garantía capturada si aplica. |
| `disputa.resuelta_en` | timestamptz ? | Trazabilidad de decisión. |

### `documento` — objeto binario privado y dueño verificable (R/O; RQF-074–078/137/145/151/161/165/211)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `espacio_id` | uuid ? | FK→`espacio.id`; galería. |
| `perfil_usuario_id` | uuid ? | FK→`perfil_usuario.usuario_id`; fotografía de perfil. |
| `verificacion_id` | uuid ? | FK→`verificacion.id`; identidad. |
| `contrato_id` | uuid ? | FK→`contrato.id`; contrato. |
| `reserva_id` | uuid ? | FK→`reserva.id`; comprobante/evidencia general. |
| `operacion_arriendo_id` | uuid ? | FK→`operacion_arriendo.id`; foto de uso. |
| `disputa_id` | uuid ? | FK→`disputa.id`; reclamo/descargo. |
| `version_terminos_id` | uuid ? | FK→`version_terminos.id`; texto de condiciones publicado. |
| `documento_tributario_id` | uuid ? | FK→`documento_tributario.id`; respaldo fiscal emitido. |
| `categoria` | text ! | Catálogo coherente con dueño; exactamente una FK dueña no nula. |
| `bucket` | text ! | Bucket privado permitido por ambiente. |
| `clave_objeto` | text ! | UK con bucket/generación; nunca URL firmada. |
| `generacion_objeto` | text ! | Versión GCS validada. |
| `mime_detectado` | text ! | Tipo real validado, no solo declarado. |
| `bytes` | bigint ! | CHECK `>0`; límites por categoría. |
| `hash_sha256` | char(64) ! | Integridad del contenido; no prueba por sí solo inmutabilidad. |
| `retencion_clase` | text ! | Clase de la matriz B.7; no fija por sí sola el plazo legal. |
| `estado` | text ! | CHECK `pendiente`, `validado`, `rechazado`, `retirado`. |
| `autor_id` | uuid ? | FK→`usuario.id`; nulo solo para generación de sistema auditada. |
| `creado_en` | timestamptz ! | Carga/alta. |

### `mensaje_reserva`, `resena` y `reporte_resena` — comunicación y reputación (P/R; RQF-153–158/181–182/207)

| Tabla.campo | Tipo | Regla y significado |
| --- | --- | --- |
| `mensaje_reserva.id` | uuid ! | PK. |
| `mensaje_reserva.reserva_id` | uuid ! | FK→`reserva.id`; solo participantes acceden. |
| `mensaje_reserva.autor_id` | uuid ! | FK→`usuario.id`; participante autorizado. |
| `mensaje_reserva.cuerpo` | text ! | Contenido privado minimizado/moderable. |
| `mensaje_reserva.estado` | text ! | CHECK `visible`, `oculto`, `retirado`. |
| `mensaje_reserva.creado_en` | timestamptz ! | Envío. |
| `resena.id` | uuid ! | PK. |
| `resena.reserva_id` | uuid ! | FK→`reserva.id`; UK con `autor_id` para una reseña por parte autorizada. |
| `resena.espacio_id` | uuid ! | FK→`espacio.id`; debe coincidir con reserva. |
| `resena.autor_id` | uuid ! | FK→`usuario.id`; parte de reserva. |
| `resena.nota` | smallint ! | CHECK `BETWEEN 1 AND 5`. |
| `resena.texto` | text ? | Público tras moderación; P si identifica persona. |
| `resena.estado` | text ! | CHECK `pendiente`, `publicada`, `oculta`. |
| `resena.creada_en` | timestamptz ! | Solo tras uso elegible. |
| `reporte_resena.id` | uuid ! | PK. |
| `reporte_resena.resena_id` | uuid ! | FK→`resena.id`. |
| `reporte_resena.denunciante_id` | uuid ! | FK→`usuario.id`. |
| `reporte_resena.motivo` | text ! | Fundamento privado. |
| `reporte_resena.estado` | text ! | CHECK `pendiente`, `resuelto`, `rechazado`. |
| `reporte_resena.decisor_id` | uuid ? | FK→`usuario.id`; administrador. |
| `reporte_resena.resuelto_en` | timestamptz ? | Fecha de decisión. |

### `notificacion` y `entrega_notificacion` — intención y canal (P/T; avisos RQF-008/116/129/139/152/171/231)

| Tabla.campo | Tipo | Regla y significado |
| --- | --- | --- |
| `notificacion.id` | uuid ! | PK. |
| `notificacion.destinatario_id` | uuid ! | FK→`usuario.id`. |
| `notificacion.tipo` | text ! | Catálogo de aviso de cuenta/reserva/pago/contrato/uso/disputa. |
| `notificacion.plantilla_version` | text ! | Contenido reproducible; no copiar PII al outbox. |
| `notificacion.recurso_tipo` | text ! | Tipo permitido; validación de propietario en caso de uso. |
| `notificacion.recurso_id` | uuid ! | Referencia lógica; alternativa FK tipada al implementar si se consulta/depura por recurso. |
| `notificacion.creada_en` | timestamptz ! | Intención durable. |
| `entrega_notificacion.id` | uuid ! | PK. |
| `entrega_notificacion.notificacion_id` | uuid ! | FK→`notificacion.id`. |
| `entrega_notificacion.canal` | text ! | CHECK `correo`, `interno`; proveedor no se infiere. |
| `entrega_notificacion.estado` | text ! | CHECK `pendiente`, `enviada`, `fallida`, `reintento`. |
| `entrega_notificacion.intentos` | integer ! | CHECK `>=0`. |
| `entrega_notificacion.disponible_en` | timestamptz ! | Programación durable. |
| `entrega_notificacion.enviada_en` | timestamptz ? | Solo tras confirmación del adaptador. |
| `entrega_notificacion.ultimo_error_codigo` | text ? | Saneado, sin dirección ni cuerpo. |

## B.5 Auditoría, derechos, analítica y promoción (M02/M09/M11)

### `evento_auditoria` — acciones críticas y correlación (P/T; RQF-178–185/212; RNF-017/043)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `actor_id` | uuid ? | FK→`usuario.id`; nulo para proceso técnico identificado en `actor_tipo`. |
| `actor_tipo` | text ! | CHECK `usuario`, `administrador`, `servicio`, `proveedor`. |
| `recurso_tipo` | text ! | Catálogo de recursos auditables. |
| `recurso_id` | uuid ? | Identificador mínimo; FK tipada si la consulta operativa lo necesita. |
| `accion` | text ! | Código estable: acceso, cambio, decisión, exportación o intento fallido. |
| `resultado` | text ! | CHECK `aceptado`, `rechazado`, `error`. |
| `motivo_codigo` | text ? | Obligatorio en decisiones administrativas y rechazo. |
| `correlacion_id` | uuid ! | Une solicitud y evento de dominio. |
| `ocurrio_en` | timestamptz ! | Instante. |
| `resumen_minimo` | jsonb ! | Esquema permitido sin credenciales, PII directa ni cuerpo completo. |

La tabla soporta consulta operacional. El mecanismo propuesto para RNF-017 exporta lotes minimizados a Cloud Storage con retención bloqueada y hash por lote **solo después de definir plazo y alcance compatibles con la matriz**; un hash aislado o BigQuery no acreditan inmutabilidad. La prueba de alteración está pendiente.

### `solicitud_titular` — ejercicio de derechos desde el primer incremento (P/R; RQF-034–037; RNF-018/026/029; PT-16)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `usuario_id` | uuid ? | FK→`usuario.id`; nulo solo si titular aún no tiene cuenta o ya fue desidentificado. |
| `identidad_reclamante_cifrada` | bytea ? | Obligatoria si `usuario_id` es nulo; dato mínimo para verificar y responder, con acceso restringido. |
| `contacto_respuesta_cifrado` | bytea ? | Obligatorio cuando no hay cuenta activa; canal de respuesta protegido que se elimina al cerrar según política validada. |
| `tipo` | text ! | CHECK `acceso`, `rectificacion`, `supresion`, `oposicion`, `portabilidad`, `bloqueo`. |
| `canal` | text ! | CHECK `web`, `correo`, `administrado`. |
| `identidad_verificada_en` | timestamptz ? | Requisito antes de entregar/cambiar datos. |
| `solicitada_en` | timestamptz ! | Inicio del plazo administrativo interno. |
| `estado` | text ! | CHECK `recibida`, `identidad_pendiente`, `en_revision`, `resuelta`, `denegada_fundada`. |
| `responsable_id` | uuid ? | FK→`usuario.id`; administrador/encargado designado. |
| `decision_codigo` | text ? | Fundamento estructurado de supresión, bloqueo, excepción o denegación. |
| `resultado_ref` | text ? | Referencia privada a exportación/evidencia de ejecución, sin dato suprimido. |
| `resuelta_en` | timestamptz ? | Obligatoria al cerrar. |

RNF-026 fija una meta interna de 72 horas; no se atribuye ese plazo a la Ley 21.719 sin análisis jurídico. La solicitud registra también acciones en GCS, BigQuery, backups y encargados cuando correspondan. Borrar PII de `usuario` sin revisar referencias, derivados y copias no resuelve el derecho.

### `outbox_evento` — evento transaccional publicable (T/A; RNF-012/024/028)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK; identificador de deduplicación. |
| `tipo_evento` | text ! | Contrato versionado. |
| `version_esquema` | integer ! | CHECK `>0`. |
| `agregado_tipo` | text ! | Reserva, espacio, pago u otro agregado permitido. |
| `agregado_id` | uuid ! | Recurso de origen. |
| `payload` | jsonb ! | Esquema mínimo validado, sin secretos ni PII directa. |
| `creado_en` | timestamptz ! | Misma transacción que hecho de dominio. |
| `disponible_en` | timestamptz ! | Próximo intento. |
| `intentos` | integer ! | CHECK `>=0`. |
| `lease_token` | uuid ? | Reclamo del worker. |
| `lease_hasta` | timestamptz ? | Permite recuperación tras reinicio. |
| `publicado_en` | timestamptz ? | Confirmación local de publicación; consumidor deduplica. |
| `ultimo_error_codigo` | text ? | Diagnóstico saneado. |

### `campana`, `orden_promocion` y `derecho_reporte` — promoción y acceso a métricas (O/R; monetización planificada)

| Tabla.campo | Tipo | Regla y significado |
| --- | --- | --- |
| `campana.id` | uuid ! | PK. |
| `campana.espacio_id` | uuid ! | FK→`espacio.id`; titular arrendador del espacio. |
| `campana.arrendador_id` | uuid ! | FK→`usuario.id`; coherencia con dueño del espacio. |
| `campana.nombre` | text ! | Etiqueta privada de campaña. |
| `campana.inicio` | timestamptz ! | Inicio de vigencia. |
| `campana.fin` | timestamptz ! | CHECK `fin>inicio`. |
| `campana.estado` | text ! | CHECK `borrador`, `pendiente_pago`, `activa`, `pausada`, `terminada`, `cancelada`. |
| `campana.creada_en` | timestamptz ! | Alta. |
| `orden_promocion.id` | uuid ! | PK. |
| `orden_promocion.campana_id` | uuid ! | FK→`campana.id`; términos/versiones aceptadas. |
| `orden_promocion.precio_clp` | numeric(14,0) ! | Precio pactado, separado de comisión por arriendo. |
| `orden_promocion.iva_clp` | numeric(14,0) ! | Según documento tributario aplicable. |
| `orden_promocion.total_clp` | numeric(14,0) ! | CHECK suma. |
| `orden_promocion.moneda` | char(3) ! | Inicial CLP. |
| `orden_promocion.estado` | text ! | CHECK `pendiente`, `pagada`, `cancelada`, `reembolsada`. |
| `orden_promocion.creada_en` | timestamptz ! | Alta. |
| `derecho_reporte.id` | uuid ! | PK. |
| `derecho_reporte.espacio_id` | uuid ! | FK→`espacio.id`. |
| `derecho_reporte.arrendador_id` | uuid ! | FK→`usuario.id`; dueño autorizado. |
| `derecho_reporte.orden_promocion_id` | uuid ! | FK→`orden_promocion.id`; acceso solo si pago/estado válido. |
| `derecho_reporte.vigencia` | tstzrange ! | Finita, `[)`; período de métricas autorizado. |
| `derecho_reporte.estado` | text ! | CHECK `activo`, `suspendido`, `vencido`. |

La campaña y el derecho se diseñan ahora para el producto completo; su activación comercial se condiciona a política/precio aprobados. Impresiones, clics y CTR viven en BigQuery; PostgreSQL verifica titular y derecho antes de devolver agregados. Los reportes de prueba en Looker Studio usan vistas y acceso segregado por vendedor.

### `respuesta_nps` — evaluación voluntaria y minimizada (P/A; capacidad Customer Success)

| Campo | Tipo | Regla y significado |
| --- | --- | --- |
| `id` | uuid ! | PK. |
| `usuario_id` | uuid ? | FK→`usuario.id` solo si seguimiento autorizado; encuesta anónima no guarda vínculo. |
| `reserva_id` | uuid ? | FK→`reserva.id` solo si finalidad comunicada. |
| `puntuacion` | smallint ! | CHECK `BETWEEN 0 AND 10`. |
| `comentario` | text ? | Texto libre separado y restringido; no sale íntegro a BigQuery. |
| `finalidad_version` | text ! | Texto informado/consentimiento cuando corresponda. |
| `respondida_en` | timestamptz ! | Instante. |

## B.6 Cardinalidades, estados y restricciones de integridad

| Relación/regla | Contrato para migración y caso de uso |
| --- | --- |
| Cuenta y oferta | `usuario` 1:N `rol_usuario`, `sesion`, `verificacion`, `espacio`; `usuario` 1:1 `perfil_usuario`; `espacio` N:1 `categoria_espacio`. |
| Precio | `espacio` 1:N `regla_tarifa`; `politica_cancelacion` 1:N `regla_tarifa`; regla publicada inmutable y sin vigencias solapadas para el mismo ámbito. `cotizacion` referencia regla de tarifa y comisión vigentes; `reserva` congela aceptación. |
| Calendario | `espacio` 1:N `ocupacion`; `reserva` 0..1 `ocupacion`; bloqueo manual sin reserva. `EXCLUDE USING gist (espacio_id WITH =, intervalo WITH &&) WHERE (activo)` y FK compuesta impiden solape y cruce de espacios. La aplicación captura conflicto SQLSTATE `23P01` como indisponibilidad. |
| Pago | `reserva` 1:N `pago` y `movimiento_financiero`; `evento_proveedor` puede llegar sin correlación; `pago` apunta a reserva **o** orden de promoción. Nunca se llama a un proveedor dentro de una transacción PostgreSQL abierta. |
| Contrato y uso | `reserva` 1:N `contrato`, `operacion_arriendo`, `disputa`; `contrato` 1:N `firma_contrato`. Solo firmantes de la reserva; check-in requiere contrato final y firmas exigidas, salvo entorno de simulación explícitamente identificado. |
| Liquidación | `reserva` 0..1 `liquidacion`; disputa abierta bloquea confirmación. Reembolso/contracargo se registran como hechos nuevos; no reescribir pago confirmado. |
| Documento | Exactamente una FK propietaria entre espacio, perfil, verificación, contrato, reserva, operación, disputa, versión de términos y documento tributario; categoría y rol de acceso coherentes. Los objetos privados no se exponen por clave o URL permanente. |
| Privacidad | Ninguna FK personal se borra en cascada; la solicitud funda acción por dato/finalidad, incluyendo derivados y respaldos. Un UUID conservado no demuestra anonimización. |
| Analítica | Cambio de dominio y `outbox_evento` comparten commit; entrega a Pub/Sub al menos una vez, consumidor deduplica; impresiones/clics no llenan PostgreSQL. |

**Estados de `reserva` para el producto completo:** `pendiente_de_pago`, `pagada`, `aprobada_host`, `firma_parcial`, `lista_para_checkin`, `en_curso`, `finalizada`, `en_disputa`, `cerrada`, `cancelada_por_pago`, `rechazada_arrendador`, `cancelada_por_vencimiento`, `cancelada_por_firma`, `cancelada_arrendatario`. Los literales técnicos normalizan nombres de la entrega anterior sin cambiar su significado. `pago.estado` y `disputa.estado` son máquinas distintas; no se confunden con la reserva.

| Transición permitida | Actor/condición y efecto |
| --- | --- |
| creación → `pendiente_de_pago` | Arrendatario autorizado; cotización vigente; insertar reserva, ocupación y outbox en una transacción. |
| `pendiente_de_pago` → `pagada`/`cancelada_por_pago` | Pago confirmado por evento/consulta verificada; rechazo o vencimiento de 15 min. Resultado ambiguo va a conciliación antes de liberar ocupación. |
| `pagada` → `aprobada_host`/`rechazada_arrendador`/`cancelada_por_vencimiento` | Arrendador decide o vence plazo de 24 h; reembolso es operación separada. |
| `aprobada_host` → `firma_parcial` → `lista_para_checkin` | Firma verificada de partes y documento final; se admite salto directo si ambas firmas llegan juntas. |
| `aprobada_host`/`firma_parcial` → `cancelada_por_firma` | Vence plazo o firma rechazada; compensación según política. |
| `lista_para_checkin` → `en_curso` → `finalizada` | Actos autorizados de check-in y check-out con evidencia requerida; el worker no fabrica uso por reloj. |
| `finalizada` → `en_disputa` → `finalizada` | Reclamo dentro de plazo y resolución motivada; liquidación bloqueada mientras abierta. |
| `finalizada` → `cerrada` | Plazo de reclamo concluido y liquidación conciliada. |
| estado cancelable → `cancelada_arrendatario` | Política snapshot, reembolso e información a partes; estados cancelables definidos por la política aceptada. |

Cada transición incrementa `reserva.version` y escribe `reserva_transicion` en el mismo commit. Los `CHECK` delimitan literales; el caso de uso valida el arco, actor, estado financiero y evidencia. Un webhook tardío tras expiración no reactiva una ocupación ya liberada: se concilia y se resuelve devolución o intervención.

## B.7 Tratamiento de datos desde el primer incremento

La Ley 21.719 es **criterio de construcción desde ahora**, sin esperar su vigencia. Cada migración y endpoint identifica finalidad, mínimo de datos, rol autorizado, receptor/encargado, ubicación, control de acceso, ciclo de vida y evidencia de supresión o excepción. La Ley 19.628 vigente y las obligaciones sectoriales se consideran simultáneamente. La matriz siguiente es un contrato de clasificación y trabajo; base jurídica y plazo exacto por finalidad requieren validación competente antes de automatizar retención definitiva [@ley21719].

| Clase/finalidad | Tablas y objetos | Acceso y salida | Evento de cierre y acción diseñada |
| --- | --- | --- | --- |
| Cuenta y autenticación | `usuario`, `perfil_usuario`, `sesion`, `token_accion`, aceptaciones | Titular y servicio de identidad; hash/token fuera de logs/analítica | Revocar sesiones al cerrar; suprimir/minimizar perfil y credencial tras resolver solicitud, preservando solo hechos con fundamento. |
| Verificación e identidad | `verificacion`, documentos KYC/KYB, `cuenta_cobro`, vínculo vendedor | Personal habilitado y proveedor encargado; cifrado/referencia externa | Vencimiento/revocación del propósito; borrar objeto y referencias conforme a excepción y plazo aprobados. |
| Oferta y localización | `espacio`, galería, ubicación | Público solo contenido autorizado y ubicación gruesa; exacta a participantes habilitados | Retirar publicación sin borrar reservas/contratos históricos; purgar medios conforme a finalidad. |
| Reserva y comunicación | `cotizacion`, `reserva`, `ocupacion`, `mensaje_reserva`, notificaciones, reseñas | Participantes; administrador motivado; sin mensajes completos en BigQuery | Vencimiento, cierre y solicitud; separar borrado de conversación/perfil de conservación de hechos contractuales. |
| Pago y tributación | `pago`, `evento_proveedor`, movimientos, liquidación, DTE | Finanzas/administración autorizada; solo referencias mínimas a proveedor | Conservar comprobantes durante plazo tributario aplicable; disociar datos no exigidos. [[PENDIENTE: validar con contador el plazo y documento exacto por operación.]] |
| Contrato y evidencia | `contrato`, firmas, operación, disputa, `documento` | Participantes y resolución autorizada; objetos privados | RNF-042 exige al menos cinco años para contratos/evidencia; confirmar inicio, excepciones y plazo legal efectivo antes de bloqueo irreversible. |
| Auditoría y derechos | `evento_auditoria`, `solicitud_titular` | Administrador/privacidad con motivo; exportación mínima protegida | RNF-043 exige al menos cinco años de auditoría; no guardar PII directa en lote inmutable. Resolver derechos con decisión fundada y trazabilidad PT-16. |
| Analítica y promoción | `outbox_evento`, campañas, derechos, NPS, datasets BigQuery | Agregados autorizados por titular del espacio; seudónimo no equivale a anonimato | Retención y desidentificación propias por evento/dataset; suprimir derivados según solicitud y finalidad. |

El plazo interno de 72 horas de RNF-026 se trata como objetivo operacional a instrumentar y medir. No se afirma que ese número sea un plazo legal de la Ley 21.719. Tampoco se aplica un plazo único de cinco o seis años a toda fila. Toda excepción a supresión debe guardar fundamento y fecha de revisión; no se sustituye por hashes del nombre/correo mientras las FK permitan reidentificar.

### B.7.1 Matriz de acceso de la API

| Sujeto | Lectura/escritura autorizada | Denegación o condición verificable |
| --- | --- | --- |
| Visitante | Publicaciones activas, precio publicado y reseñas moderadas | Sin dirección exacta, perfil privado, mensajería, reserva ni métricas de vendedor. |
| Usuario registrado | Su perfil, sesiones y solicitudes de derechos | No ve PII de otras cuentas; cierre revoca sesiones. |
| Arrendatario | Sus cotizaciones/reservas, pagos propios, contratos, mensajes y actos de uso | No resuelve disputas ni accede a cuenta de cobro del arrendador; cancelación según snapshot. |
| Arrendador | Sus espacios, tarifas, calendario, solicitudes, contratos y agregados de métricas con derecho vigente | No ve token de pago, documento KYC o conversaciones ajenas; dirección exacta de otro anuncio sujeta a autorización. |
| Administrador | Moderación, verificaciones, disputa, soporte y auditoría necesaria | Cada acceso sensible exige motivo/correlación; roles comerciales no se heredan automáticamente. |
| Worker del monolito | Filas pendientes de expiración, notificación, outbox y conciliación | Cuenta de servicio sin sesión humana; lease e idempotencia; consultas limitadas a su finalidad. |
| Consumidor analítico | Eventos minimizados y agregados autorizados | Sin secretos, PAN/CVV, KYC, mensajes completos ni capacidad de modificar hechos operativos. |

El backend comprueba rol, titularidad y finalidad **por recurso y en cada consulta/comando**; los filtros visuales y el `usuario_id` enviado por el cliente no son autoridad. Los roles PostgreSQL separan migración, API y operación, pero no sustituyen esta autorización de negocio.

## B.8 Seguridad, migración y evidencias requeridas

- **Roles DB:** credencial de migración con DDL separada de `app_rw`; cuentas de lectura/operación con permisos mínimos. La API aplica autorización por recurso/rol y consultas filtradas. Secret Manager custodia secretos; no guardar PAN/CVV ni tokens OAuth en PostgreSQL.
- **Migraciones:** SQL numerado y versionado en `db/migrations/`, checksum, ejecución serial y registro de versión; patrón expandir–migrar datos–contraer. Comprobar PostgreSQL 18/PostGIS/`btree_gist` en local y Cloud SQL antes de afirmar portabilidad. No convertir el ejemplo del anexo previo en la primera migración.
- **Dinero:** igualdad de moneda y conciliación entre cotización, reserva, intento, reporte externo, movimiento, documento y liquidación. `comision_neta`, `iva_comision`, `tarifa_proveedor` y `neto_arrendador` son conceptos separados. Tarifa y distribución real de Split se verifican en sandbox y con proveedor/contador.
- **Concurrencia:** exclusión GiST para reservas/bloqueos, idempotencia y unicidad externa para pagos/webhooks, leases para worker/outbox. Una transacción local no revierte una operación externa confirmada.
- **Continuidad:** respaldos/PITR y restauración aislada, incluidas referencias de GCS y secretos; RPO/RTO RNF-010 se acreditan solo con ensayo fechado. HA se decide con costo/disponibilidad medidos; activar una opción no demuestra RTO.
- **Rastreo académico:** RQF/RNF/CU anteriores se conservan; PT-16 cubre derechos. Las pruebas MD-01–MD-13 del diseño preliminar siguen como criterios planificados: integridad referencial, exclusión y concurrencia, estados, documentos, pagos/webhooks, worker/outbox, privacidad, migración y restauración. Se actualizarán sus entradas y resultados al existir migraciones, ambiente y producto.

[[PENDIENTE: confirmar con contador y documentos reales la afectación/IVA, emisor y retención de cada componente financiero; con proveedor las capacidades efectivas de garantía, reembolso y Split; con responsable de privacidad las bases jurídicas/plazos por finalidad y el bloqueo de retención; ejecutar migraciones y MD/PT con evidencia fechada.]]
