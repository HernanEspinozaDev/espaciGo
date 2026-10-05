# Referencia Realmo: categorías y contenido de la ficha de espacio

Fecha de análisis: 2026-10-05.

Estado: referencia de producto y propuestas para evaluar. No modifica requisitos, categorías seed, arquitectura, contratos API ni criterios de aceptación del PR #134. No define el frontend definitivo.

## Fuente y alcance de la observación

Fuente primaria: [ficha de Realmo, 2448 State Highway 361, Ingleside](https://realmo.com/listing/2448-state-highway-361-ingleside-tx-78362/20941840).

El contenido consultado distingue tipos y subtipos de inmueble, usos del espacio y condiciones del arriendo. La ficha reúne fotografías/mapa, precio por período, superficies, descripción, características, ubicación y ofertas relacionadas. También presenta análisis de inversión y de mercado.

La revisión se basa en el contenido textual recuperado. El navegador mostró una verificación Cloudflare, por lo que no se confirmó visualmente la composición, comportamiento responsive ni interacciones. Esta ficha no demuestra presencia o ausencia de Realmo en Latinoamérica ni una superioridad medida frente a otros portales.

## Aplicación propuesta a EspaciGo

EspaciGo busca facilitar arriendos flexibles y un recorrido posterior de disponibilidad, cotización y reserva. La propuesta propia es organizar datos que permitan comparar espacios y determinar si sirven para un uso concreto.

Separar conceptualmente:

1. **Categoría:** tipo del espacio; conservar las ocho categorías ya ratificadas.
2. **Usos permitidos:** actividades que el titular permite; requieren decisión y reglas propias. La categoría por sí sola no acredita permisos, habilitación comercial o normativa.
3. **Características:** equipamiento y condiciones físicas relevantes para el tipo de espacio.
4. **Oferta de arriendo:** moneda, unidad tarifaria, precio y, en cortes posteriores, disponibilidad y condiciones.

No convertir cada característica o actividad en una categoría nueva. No agregar una taxonomía internacional completa por tomar esta página como referencia.

## Categorías ratificadas y atributos candidatos

Los nombres corresponden a la decisión del usuario para el seed M04. La columna de atributos es una propuesta nueva, no un conjunto obligatorio ni una autorización para modificar persistencia.

| Categoría vigente | Atributos candidatos para evaluar con el producto |
| --- | --- |
| Oficina | Puestos de trabajo, mobiliario, conexión a internet y sala de reuniones. |
| Sala/multipropósito | Capacidad, disposición de mesas/sillas, proyector y equipamiento de audio. |
| Bodega | Volumen útil, condiciones de almacenamiento, acceso para carga y altura útil. |
| Estacionamiento | Dimensiones de la plaza, altura máxima, cubierta y horarios de acceso. |
| Local flexible | Superficie utilizable, mobiliario/equipamiento disponible y actividades permitidas. |
| Stand | Dimensiones, conexión eléctrica, mobiliario y condiciones del recinto anfitrión. |
| Quincho | Parrilla, mesas, servicios higiénicos y capacidad de personas. |
| Parcela/eventos | Superficie destinada al evento, capacidad, servicios y restricciones de ruido/horario. |

Reglas de evaluación:

- Mantener códigos estables y etiquetas editables en el catálogo; no introducir un enum cerrado de categorías en código.
- Distinguir atributo desconocido, no declarado y no aplicable. Ausencia de información no equivale automáticamente a «no disponible».
- Establecer tipo, unidad, obligatoriedad y límites únicamente cuando el atributo se acepte para un corte funcional.
- Mantener superficie en m², moneda CLP y las unidades hora/día/mes del contrato actual. No confundir capacidad de personas con volumen o dimensiones de almacenamiento.
- No agregar una tabla por categoría ni un mecanismo genérico de atributos antes de justificarlo con necesidades aceptadas. El diseño se evalúa como parte del corte que requiera esos datos.

## Información que ya puede mostrarse con M04

Con los campos actuales puede probarse una lista sencilla y un detalle privado de borrador que presenten:

- título y etiqueta de categoría;
- superficie, capacidad y estado `borrador`;
- precio en CLP acompañado de su unidad;
- descripción y reglas de uso;
- dirección privada, exclusivamente al propietario autenticado.

La mejora inmediata posible es facilitar la lectura de estos datos en el mock HTML/CSS/TypeScript. Los controles de crear, consultar y editar seguirían consumiendo las APIs existentes. Esto no requiere fotografías, mapas, nuevos datos, publicación comercial ni un frontend de producción.

Para una ficha pública futura quedan pendientes decisiones de ubicación visible, precisión del mapa, tratamiento de dirección exacta, publicación, validación de archivos y autorización. No trasladar automáticamente a una vista pública los campos privados del borrador.

## Trazabilidad y secuencia sugerida

Referencias internas: [visión y módulos](vision_y_modulos.md), [decisiones de borradores M04](decisiones_m04_borradores.md), [prototipo M04](prototipo_local_m04.md), [requisitos ES1](referencias/ES1/B_requerimientos_funcionales.md) y [diccionario ES2](referencias/ES2/B_diccionario_datos.md).

- Tipo, superficie, capacidad y tarifa se relacionan con RQF-066–073 y CU-15; fotografías, ubicación y disponibilidad pertenecen a sus requisitos y cortes posteriores de M04/M05.
- Los nuevos atributos de la matriz son propuestas: no atribuirlos a ES1 o ES2 sin comprobar una referencia concreta.
- Completar las correcciones y revisión del PR #134 antes de ampliar su entrega.
- Si el usuario autoriza mejorar la legibilidad del mock, agrupar esa mejora en una tarea pequeña usando solo datos/API actuales. No convertirla en un nuevo prerrequisito global.
- Si el usuario acepta atributos por categoría, definir su alcance, impacto DB/Backend/API y validación en una tarea explícita. Aplicar cambios persistentes mediante nuevas migraciones; conservar migraciones ya aplicadas y el volumen local.
- Galería/storage, calendario, búsqueda y publicación conservan sus tareas y dependencias. Este análisis no las declara resueltas ni habilita operaciones comerciales.

## Instrucción sugerida para el siguiente agente

> Lee `planning/referencia_realmo_categorias_y_ficha.md` como referencia de producto. Conserva las ocho categorías ratificadas y distingue categorías, usos y características. Termina primero las correcciones del PR #134. Tras su aceptación, propón un corte pequeño para presentar mejor los borradores en el mock mediante los datos y endpoints existentes, con título, categoría, superficie, capacidad, precio/unidad y detalle legibles. Mantén HTML/CSS/TypeScript y los controles de autorización del Backend. Guarda los atributos nuevos como propuestas pendientes; no amplíes el esquema, el alcance comercial ni el frontend definitivo sin autorización. Documenta el avance en GitHub Projects y publica con HernanMEC cuando se autorice la implementación.
