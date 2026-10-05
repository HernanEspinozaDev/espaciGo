# Catálogo extensible de categorías y atributos de EspaciGo

Fecha: 2026-10-05. Estado: base de planificación aplicada al corte M04-ATTR-01 (#135–#140); las decisiones finales y límites implementados están en [decisiones_m04_catalogo_atributos.md](decisiones_m04_catalogo_atributos.md). Los ejemplos son editables; no constituyen atributos obligatorios de producción ni requisitos añadidos a ES1/ES2.

## 1. Objetivo y relación con la entrega actual

Conservar las ocho categorías ratificadas y agregar características particulares mediante **composición**: un espacio tiene datos comunes y un conjunto de características validado según su categoría. El catálogo define etiquetas, tipos y opciones; el Backend aplica validaciones y reglas de negocio.

El PR #134 entregó y fue aceptado como base de borradores propios. Por instrucción de producto del 2026-10-05, el corte M04-ATTR-01 implementa esta extensión mediante una migración V6 incremental, Backend, API/OpenAPI, pruebas enfocadas y mock; V5 permanece intacta. La revisión del código/PR confirma la aplicación concreta de la propuesta, documentada en [decisiones M04 del catálogo](decisiones_m04_catalogo_atributos.md).

La investigación consultó páginas de los propios operadores y recintos. Las fuentes respaldan ejemplos de equipamiento y características anunciadas; los códigos, tipos, rangos y ejemplos siguientes son diseño propuesto para EspaciGo. No trasladar precios, condiciones contractuales o disponibilidad de esos operadores como reglas del proyecto.

## 2. Categorías y conceptos distintos

| Código estable actual | Etiqueta inicial |
| --- | --- |
| `oficina` | Oficina |
| `sala_multiproposito` | Sala/multipropósito |
| `bodega` | Bodega |
| `estacionamiento` | Estacionamiento |
| `local_flexible` | Local flexible |
| `stand` | Stand |
| `quincho` | Quincho |
| `parcela_eventos` | Parcela/eventos |

- **Categoría:** clasificación principal del espacio. Su etiqueta puede cambiar sin cambiar el código.
- **Característica:** dato sobre el espacio, como puestos de trabajo, altura útil o parrilla.
- **Uso permitido:** actividad autorizada por el titular; no equivale a habilitación municipal, permiso sanitario o aprobación comercial. Su contrato se define en un corte posterior.
- **Condición del arriendo:** tarifa, disponibilidad, horarios y reglas. Mantenerla en el módulo y contrato correspondientes; los atributos no deben crear un segundo calendario o precio.

## 3. Datos comunes y valores desconocidos

Los campos actuales permanecen en `espacio`: propietario, título, descripción, categoría, superficie en m², capacidad de personas, reglas de uso, modalidad, precio CLP, dirección privada y estado. Las características no pueden sobrescribirlos.

Características compartidas candidatas, habilitadas solo para categorías en que tengan sentido:

| Código | Tipo | Ejemplo | Significado |
| --- | --- | --- | --- |
| `wifi` | booleano | `true` | El titular declara conexión disponible; no promete velocidad. |
| `energia_electrica` | booleano | `true` | Suministro disponible para el uso acordado. |
| `agua_potable` | booleano | `true` | Disponibilidad declarada. |
| `banos_disponibles` | entero ≥ 0 | `2` | Número de servicios higiénicos accesibles al arrendatario. |
| `climatizacion` | lista de opciones | `["aire_acondicionado"]` | Opciones iniciales: aire acondicionado, calefacción, ventilación mecánica. |

Los atributos nuevos son opcionales en borradores durante el primer corte. Si el usuario los declara, deben cumplir tipo y límites. Un dato omitido significa **no declarado**; `false` significa ausencia declarada y `0` es una cantidad válida solo cuando la definición lo permita. No rellenar automáticamente todo con `false` o `0`. No aceptar `null` salvo que el contrato de ese atributo lo autorice; para borrar un dato opcional se retira del objeto de características mediante el reemplazo documentado.

La capacidad común sigue significando personas. En estacionamientos, `vehiculos_admitidos` es un atributo distinto. En bodegas, volumen y carga tampoco sustituyen la capacidad actual. Revisar una futura obligatoriedad por tipo exige una decisión explícita y compatibilidad con los borradores existentes.

## 4. Perfiles de atributos para las ocho categorías

Convenciones propuestas: dimensiones positivas con hasta dos decimales, cantidades enteras dentro del rango PostgreSQL elegido y catálogos de opciones versionados. Los ejemplos son fixtures sintéticos, no mínimos reglamentarios ni medidas estándar del mercado. `★` identifica los tres atributos sugeridos para destacar primero en el mock; no significa obligatorio.

### 4.1 Oficina — `oficina`

Referencia observada: WeWork describe oficinas privadas, puestos de trabajo, mobiliario y servicios de conectividad. [WeWork On Demand](https://www.wework.com/solutions/wework-on-demand).

| Código propuesto | Tipo / regla inicial | Ejemplo |
| --- | --- | --- |
| `puestos_trabajo` ★ | entero ≥ 1 | `6` |
| `mobiliario_incluido` ★ | booleano | `true` |
| `tipo_uso_oficina` ★ | opción: privada / compartida | `privada` |
| `escritorios` | entero ≥ 0 | `6` |
| `sillas_trabajo` | entero ≥ 0 | `6` |
| `salas_reunion_disponibles` | entero ≥ 0 | `1` |
| `impresora_disponible` | booleano | `false` |

Compartidos aplicables: wifi, energía, baños y climatización. La cantidad de puestos informa equipamiento; no reemplaza el límite de personas del espacio ni implica que otros servicios estén incluidos en el precio.

### 4.2 Sala/multipropósito — `sala_multiproposito`

Referencia observada: WeWork describe salas con capacidad, disposiciones y equipamiento audiovisual, pantallas y herramientas de colaboración; disponibilidad variable por ubicación. [WeWork Meeting Rooms](https://www.wework.com/solutions/meeting-rooms?capacity=1).

| Código propuesto | Tipo / regla inicial | Ejemplo |
| --- | --- | --- |
| `disposiciones_admitidas` ★ | lista: auditorio / mesa_reunion / aula / espacio_libre | `["aula", "mesa_reunion"]` |
| `sillas_disponibles` ★ | entero ≥ 0 | `24` |
| `proyector` ★ | booleano | `true` |
| `mesas_disponibles` | entero ≥ 0 | `6` |
| `pantalla` | booleano | `true` |
| `pizarra` | booleano | `true` |
| `videoconferencia` | booleano | `false` |
| `equipo_audio` | booleano | `true` |

Compartidos aplicables: wifi, energía, baños y climatización. La capacidad común conserva el máximo de personas; no inferir aforo certificado a partir de sillas, superficie o disposición.

### 4.3 Bodega — `bodega`

Referencia observada: Safestore Enfield anuncia acceso para carga, puertas dobles, alturas útiles y carros. [Safestore Enfield](https://www.safestore.co.uk/self-storage/london/north/enfield/).

| Código propuesto | Tipo / regla inicial | Ejemplo |
| --- | --- | --- |
| `altura_util_m` ★ | decimal > 0, unidad m | `3.2` |
| `volumen_util_m3` ★ | decimal > 0, unidad m³ | `64` |
| `acceso_carga` ★ | opción: peatonal / vehiculo_liviano / furgon / camion | `furgon` |
| `ancho_acceso_m` | decimal > 0, unidad m | `2.4` |
| `altura_acceso_m` | decimal > 0, unidad m | `2.8` |
| `carro_carga_disponible` | booleano | `true` |
| `muelle_carga` | booleano | `false` |
| `tipo_almacenamiento` | opción: seco / temperatura_controlada / refrigerado | `seco` |

El volumen útil es una declaración independiente; no calcularlo automáticamente como superficie × altura si no se confirma una geometría adecuada. Temperatura controlada/refrigeración son opciones propuestas y requieren definir alcance si se habilitan; la fuente citada no prueba que esas modalidades existan en cada unidad. Productos admisibles y prohibidos requieren reglas específicas posteriores.

### 4.4 Estacionamiento — `estacionamiento`

Referencia observada: JustPark recomienda declarar dimensiones, límite de altura y tipos de vehículo admitidos. [JustPark: vehículos grandes](https://support-uk.justpark.com/hc/en-gb/articles/207418997-I-don-t-want-to-accept-large-vehicles).

| Código propuesto | Tipo / regla inicial | Ejemplo |
| --- | --- | --- |
| `largo_plaza_m` ★ | decimal > 0, unidad m | `5` |
| `ancho_plaza_m` ★ | decimal > 0, unidad m | `2.5` |
| `vehiculos_admitidos` ★ | lista: motocicleta / automovil / suv / furgon | `["automovil", "suv"]` |
| `altura_maxima_m` | decimal > 0 cuando existe restricción | `2.1` |
| `techado` | booleano | `true` |
| `tipo_acceso` | opción: abierto / porton_manual / porton_automatico | `porton_manual` |
| `carga_electrica` | booleano | `false` |

Para el primer corte, una publicación representa una unidad reservable. No añadir número de plazas como inventario comercial sin definir después disponibilidad y reserva por unidad. El acceso no debe guardar claves de portón, credenciales o instrucciones sensibles en estos atributos. Carga eléctrica es una extensión candidata; su potencia, compatibilidad y tarifa requieren un contrato posterior.

### 4.5 Local flexible — `local_flexible`

Referencia observada: Storefront muestra locales temporales con electricidad, mostradores, probadores y acceso a nivel de calle. [Rosebery Pop Up Space](https://www.thestorefront.com/spaces/australia/new-south-wales/rosebery/61294-rosebery-pop-up-space). Otra ficha describe exposición de vitrina y frente a calle. [Front shop space](https://www.thestorefront.com/fr/spaces/united-kingdom/england/greater-london/59492-front-shop-space).

| Código propuesto | Tipo / regla inicial | Ejemplo |
| --- | --- | --- |
| `acceso_nivel_calle` ★ | booleano | `true` |
| `vitrina` ★ | booleano | `true` |
| `frente_m` ★ | decimal > 0, unidad m | `4.5` |
| `mostradores` | entero ≥ 0 | `1` |
| `probadores` | entero ≥ 0 | `2` |
| `bodega_apoyo_m2` | decimal > 0 cuando existe | `5` |
| `iluminacion_exhibicion` | booleano | `true` |

Compartidos aplicables: wifi, energía, baños y climatización. No copiar tipos de arriendo o duraciones de Storefront. El precio y duración de EspaciGo conservan el contrato del producto.

### 4.6 Stand — `stand`

Referencia observada: el reglamento EXPONOR 2024 describe panelería, iluminación y suministro eléctrico para stands; sirve como ejemplo de equipamiento, no como regulación actual de EspaciGo. [EXPONOR 2024, artículo octavo](https://exponor.cl/wp-content/uploads/2023/11/Reglamento-Oficial-Exponor-2024-espanol-17-11-2023.pdf).

| Código propuesto | Tipo / regla inicial | Ejemplo |
| --- | --- | --- |
| `ancho_stand_m` ★ | decimal > 0, unidad m | `3` |
| `fondo_stand_m` ★ | decimal > 0, unidad m | `3` |
| `montaje_incluido` ★ | booleano | `true` |
| `lados_abiertos` | entero de 1 a 4 | `2` |
| `altura_maxima_m` | decimal > 0, unidad m | `2.5` |
| `potencia_disponible_w` | entero ≥ 0, unidad W | `1000` |
| `paneleria` | booleano | `true` |
| `mesas_incluidas` | entero ≥ 0 | `1` |
| `sillas_incluidas` | entero ≥ 0 | `2` |

Compartidos aplicables: energía y wifi. Lados abiertos y montaje incluido son propuestas de clasificación propias; no equiparar «energía disponible» a potencia ilimitada. Fechas de feria y horarios de montaje requieren contrato posterior, sin sustituir el calendario de disponibilidad.

### 4.7 Quincho — `quincho`

Referencia observada: Club House UC anuncia parrillas, mesas, baños y agua potable. [Quinchos del Club Deportivo Universidad Católica](https://lacatolica.cl/secciones/2108). Casona Culiprán describe lavaplatos, terraza cubierta y equipamiento de frío. [Casona Culiprán](https://www.casonaculipran.cl/reservas).

| Código propuesto | Tipo / regla inicial | Ejemplo |
| --- | --- | --- |
| `tipo_parrilla` ★ | opción: carbon / gas / electrica / lena / sin_parrilla | `carbon` |
| `techado` ★ | booleano | `true` |
| `asientos_disponibles` ★ | entero ≥ 0 | `20` |
| `parrillas_disponibles` | entero ≥ 0 | `1` |
| `mesas_disponibles` | entero ≥ 0 | `4` |
| `lavaplatos` | booleano | `true` |
| `refrigerador` | booleano | `true` |
| `utensilios_incluidos` | booleano | `false` |

Compartidos aplicables: agua, energía y baños. Capacidad común sigue siendo personas. Las alternativas de combustible son opciones propias; no se deducen del término genérico «parrilla» de la fuente. Si se declara `sin_parrilla`, la cantidad debe ser cero; una cantidad positiva exige un tipo compatible. Consumo de combustible, limpieza y actividades permitidas pertenecen a condiciones explícitas posteriores.

### 4.8 Parcela/eventos — `parcela_eventos`

Referencia observada: Casona Culiprán describe áreas verdes, terraza cubierta, servicios higiénicos, cocina opcional y piscina para eventos. [Casona Culiprán](https://www.casonaculipran.cl/reservas). Los Ingleses de Chicureo anuncia salón, capacidad y estacionamientos. [Los Ingleses de Chicureo](https://www.losinglesesdechicureo.cl/).

| Código propuesto | Tipo / regla inicial | Ejemplo |
| --- | --- | --- |
| `superficie_exterior_util_m2` ★ | decimal > 0, unidad m² | `1200` |
| `superficie_cubierta_util_m2` ★ | decimal > 0 cuando existe, unidad m² | `120` |
| `estacionamientos_disponibles` ★ | entero ≥ 0 | `15` |
| `salon_cubierto` | booleano | `true` |
| `cocina_disponible` | booleano | `true` |
| `quincho_disponible` | booleano | `true` |
| `piscina_disponible` | booleano | `false` |
| `acceso_bus` | booleano | `false` |

Compartidos aplicables: agua, energía y baños. Superficies parciales no pueden superar la superficie común declarada ni duplicar zonas cuando se suman; detallar la semántica de superficie total antes de imponer una suma. Estacionamientos y capacidad no acreditan aforo certificado. Ruido, proveedores externos y horarios se definen como condiciones posteriores; no convertirlos en permisos legales presumidos.

## 5. Definición extensible del catálogo

Cada perfil de categoría contendrá metadatos de atributos: código estable, etiqueta, descripción, tipo (`boolean`, `integer`, `number`, `enum`, `enum_list`), unidad, opciones, límites, aplicabilidad, orden y candidato a filtro. El Backend valida este perfil; el mock solo lo consume.

Propuesta inicial: un perfil versionado por categoría con esquema JSON Schema y metadatos de presentación mínimos separados. Conservar códigos de atributo en `snake_case` para evitar que un cambio de etiqueta modifique datos o requests. Un ejemplo conceptual de perfil de oficina:

```json
{
  "category_code": "oficina",
  "schema_version": 1,
  "attributes": [
    {"code": "puestos_trabajo", "label": "Puestos de trabajo", "type": "integer", "minimum": 1, "required_in_draft": false},
    {"code": "mobiliario_incluido", "label": "Mobiliario incluido", "type": "boolean", "required_in_draft": false},
    {"code": "tipo_uso_oficina", "label": "Tipo de uso", "type": "enum", "options": ["privada", "compartida"], "required_in_draft": false}
  ]
}
```

Es una representación de ejemplo, no el contrato final de la API. En implementación se debe elegir una única fuente de tipos/límites; evitar mantener reglas diferentes en este metadato, JSON Schema, structs y DOM. Los catálogos de opciones se versionan; no son nuevos tipos de inmueble.

El corte M04-ATTR-01 siembra perfiles v1 para las ocho categorías con validación declarativa común. Las reglas que relacionan atributos entre sí se agregan mediante validadores específicos cuando aporten comportamiento concreto; no crear ocho clases vacías ni duplicar ocho veces el CRUD.

## 6. Composición y comportamiento en Backend Go

Modelo conceptual:

```text
Espacio
  datos comunes + propietario + estado
  categoría
  características
    perfil de categoría + versión
    valores tipados y validados

Servicio de espacios
  autenticación/ownership y operaciones comunes
  validación declarativa del perfil
  reglas específicas cuando corresponda
  persistencia transaccional
```

Go permite composición mediante structs e interfaces, sin la herencia clásica de clases. [Effective Go: embedding](https://go.dev/doc/effective_go#embedding).

Un perfil/estrategia comparte operaciones de validación; `OfficeDetails`, `WarehouseDetails` o tipos equivalentes se justifican si existen cálculos o invariantes específicos. Una representación genérica de valores JSON solo entra al dominio después de validar tipos y claves con el perfil. Los casos de uso de crear/listar/consultar/editar permanecen comunes.

Dos niveles de extensibilidad:

1. Nueva etiqueta/categoría que usa los tipos y validaciones existentes: datos versionados del catálogo, nueva migración/seed cuando corresponda, contrato y fixtures. No requiere una clase nueva por nombre.
2. Nuevo comportamiento, unidad especial o relación entre campos: implementación explícita de regla/estrategia y pruebas. Un catálogo no elimina este trabajo.

El catálogo no almacena código ejecutable, SQL ni instrucciones de autorización. Las características no cambian propietario, estado comercial, precios, reservas o resultados KYC.

## 7. Persistencia propuesta para el prototipo

Recomendación inicial: conservar columnas relacionales para datos comunes y usar JSONB validado para las características variables. PostgreSQL permite consultas e índices sobre JSONB; almacenar JSONB no valida por sí solo el esquema específico del negocio. [PostgreSQL 18: tipos JSON](https://www.postgresql.org/docs/18/datatype-json.html).

Modelo lógico candidato, sujeto a revisión en la tarea de implementación:

| Entidad | Responsabilidad |
| --- | --- |
| `categoria_espacio` existente | Código estable, etiqueta, orden y activación. |
| `categoria_perfil_atributos` nueva | Perfil inmutable identificado por categoría + versión; esquema y metadatos. |
| `espacio_caracteristicas` nueva | Un conjunto de valores por espacio: referencia al perfil exacto y objeto JSONB validado. |

Invariantes a implementar:

- Vincular las características al espacio y al perfil mediante FK; la categoría del perfil debe coincidir con la categoría del espacio. Si se usa FK compuesta hacia espacio, agregar la clave única necesaria mediante la nueva migración.
- Mantener el objeto JSONB y tamaño máximos controlados; propuesta inicial 16 KiB para características, validado antes de persistir.
- Persistir modificación del espacio y características en la misma transacción; un fallo no deja una categoría nueva con atributos de la anterior.
- Validar claves permitidas, tipos, opciones, rangos y reglas combinadas en el Backend antes de escribir. DB conserva integridad relacional y comprobaciones estructurales básicas.
- No publicar escritura directa de JSONB sin validación ni guardar URLs privadas, documentos, secretos o identidades en los atributos.
- Crear índices solo para consultas/filtros definidos. No indexar todos los atributos desde el primer corte. Para uso intensivo de un dato, evaluar una columna/tipo relacional mediante tarea futura.

La propuesta evita una tabla por categoría y una tabla de valores por cada celda de atributo en esta etapa. Si aparece una categoría con restricciones complejas, podrá justificar una estructura específica, manteniendo los datos comunes.

## 8. Versionado y compatibilidad con datos existentes

- V5 y sus checksums permanecen intactos; cualquier cambio usa una migración posterior.
- Los borradores actuales sin características siguen válidos y legibles. No inventar valores para rellenarlos.
- Cambiar una etiqueta no cambia códigos ni significado. Cambiar unidades, significado, límites u obligatoriedad crea otra versión del perfil.
- Una versión usada por un espacio no se sobrescribe. Publicar un perfil nuevo no convierte automáticamente datos antiguos a otra semántica.
- Leer y editar con el perfil registrado mientras sea compatible. Retirar una versión o convertir datos necesita tarea explícita y reglas de transición.
- Al cambiar categoría de un espacio con características, el cliente envía un conjunto válido para la categoría destino y confirma el reemplazo. No reinterpretar silenciosamente campos anteriores; cambios incompatibles producen un error controlado sin escritura parcial.
- Agregar una categoría o un campo compatible no requiere rediseñar el frontend definitivo. El mock puede construir controles sencillos desde metadatos aceptados, sin un framework o un motor de formularios complejo.

## 9. Contrato API y mock implementados en M04-ATTR-01

Se reutiliza `/api/v1/spaces` y sus operaciones. La extensión compatible añade `attribute_schema_version` y `attributes` a request/response; la versión vigente se infiere si el cliente no la envía. Los campos superiores siguen estrictos y el objeto `attributes` tiene sus propias claves admitidas por perfil. El tratamiento de versiones antiguas está especificado en OpenAPI.

El catálogo conserva código y etiqueta. `GET /api/v1/spaces/categories/{categoryCode}/attributes` devuelve el perfil autenticado con versión, campos, unidades, límites y opciones. REST/JSON es independiente del HTML generado por el mock.

El mock utiliza HTML/CSS/TypeScript, DOM y `fetch`: seleccionar categoría, cargar perfil, mostrar campos aplicables, crear/editar, inspeccionar respuesta y errores. Respetar ownership, mantener tokens según el contrato actual y mostrar solo atributos declarados. Un checkbox debe distinguir «no declarado» de «no»; no enviar `false` simplemente por cargar un formulario vacío.

No diseñar navegación, componentes o UX/UI definitivos. Galería, mapas, calendario, publicación, búsqueda pública, reservas, KYC y cobros mantienen sus cortes propios.

## 10. Entrega y criterios de aceptación propuestos

La entrega vertical se agrupa en el PR asociado a #135–#140: V6, perfiles, Backend, API/OpenAPI, pruebas enfocadas y mock. Issues/Projects registran su alcance y dependencias; no se infiere su aceptación desde la publicación del PR.

Dependencia principal: PR #134 aceptado y fusionado. También reutiliza contratos M01 y el migrador ya entregados. No depende de completar todas las funciones pendientes M02/M03 para editar características de borradores privados.

Criterios:

1. Las ocho categorías conservan sus códigos y reciben perfiles con tipos, unidades y ejemplos trazables a esta propuesta.
2. Borradores previos siguen visibles/editables sin reiniciar volumen ni modificar V5.
3. Se crean, consultan y editan atributos de cada perfil; tipos, opciones, claves desconocidas y atributos de otra categoría se rechazan con errores API definidos.
4. La API aplica ownership a características igual que a espacio: leer/editar otro propietario devuelve 404 y no filtra datos.
5. Cambiar categoría es atómico y no deja mezclados perfiles/valores. Omitido, falso y cero se distinguen según definición.
6. Una nueva categoría basada en tipos soportados puede añadirse mediante perfil de datos, sin duplicar CRUD o introducir un switch obligatorio por todas las etiquetas.
7. Requests y responses de ejemplo validan contra OpenAPI/perfiles. Incluir regresión de cambio de categoría.
8. Validación de persistencia usa PostgreSQL descartable; desarrollo conserva `espacigo_pgdata` y secretos.
9. Mock funciona desde su contenedor, consume API pública y no incorpora acceso DB ni un framework frontend.
10. PR describe qué atributos y reglas se implementaron y qué continúa como propuesta; no declara publicado comercialmente el espacio.

Priorizar pruebas enfocadas de esos criterios y conservar pruebas existentes. No convertir esta entrega en una auditoría repetida del entorno o del tablero.

## 11. Trazabilidad y decisiones futuras

RQF-066–073/CU-15 respaldan superficie, categoría, capacidad, reglas y tarifa. RQF-098–099 respaldan filtros por tipo y superficie; filtros por atributos nuevos son propuestas adicionales, no requisitos históricos asumidos. Consultar [requisitos ES1](referencias/ES1/B_requerimientos_funcionales.md), [diccionario ES2](referencias/ES2/B_diccionario_datos.md) y [decisiones M04](decisiones_m04_borradores.md).

Antes de habilitar publicación comercial, resolver por tarea explícita atributos obligatorios por tipo, usos permitidos, precisiones, eventual evidencia y condiciones. Los ejemplos web no acreditan cumplimiento legal, identidad, seguridad física o autorización de uso del recinto.

Pendientes independientes: M02/M03, DB02-09, #123, storage/galería y calendario. No cerrarlos por adoptar este catálogo. ES1 es inmutable; nuevos requisitos y decisiones se documentan fuera de sus informes congelados.

## 12. Instrucción de implementación atendida

> El usuario autorizó el corte tras el merge #134. La implementación y sus decisiones están en el PR del corte M04-ATTR-01 y en [decisiones_m04_catalogo_atributos.md](decisiones_m04_catalogo_atributos.md). La revisión y aceptación siguen a cargo de HernanEspinozaDev; los criterios y pendientes de otras entregas permanecen separados.
