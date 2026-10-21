# Categoría Inmuebles: alcance funcional y datos

## Objetivo

Incorporar una categoría para ofrecer en arriendo casas, departamentos, habitaciones y otros espacios residenciales por hora, día o mes. El perfil debe describir el espacio y sus condiciones sin duplicar el precio, el calendario, la dirección privada ni la capacidad general que ya administra `espacio`.

## Investigación de publicaciones

Las guías consultadas destacan que la publicación debe identificar con precisión el tipo de espacio y su privacidad, la cantidad y disposición de habitaciones, camas y baños, las áreas compartidas, los servicios, las reglas y la ubicación. Para habitaciones, es relevante indicar si el dormitorio es privado, qué espacios se comparten, quién puede estar en el inmueble y si el baño es privado o compartido. Las amenidades deben declararse con exactitud y sus restricciones también deben ser visibles.

- [Airbnb: tipos de alojamientos](https://www.airbnb.com/help/article/317): distingue inmueble completo, habitación privada y habitación compartida; describe el acceso a áreas comunes y la privacidad del baño.
- [Airbnb: publicar una habitación](https://www.airbnb.com/help/article/3400): una habitación privada tiene dormitorio con puerta y puede compartir baño, cocina o sala.
- [Airbnb: detalles de habitaciones y espacios](https://www.airbnb.com/help/topic/1484): trata cantidad y tipos de espacios, camas, privacidad, amenidades y restricciones de acceso.
- [Airbnb: reglas para anfitriones](https://www.airbnb.com/help/article/2895): pide que tipo, tamaño, privacidad, ubicación, fotos, servicios y reglas describan fielmente el inmueble.

Estas referencias sirven para diseñar campos informativos; no constituyen requisitos legales chilenos ni sustituyen la revisión de contratos, normativa o condiciones de seguridad.

## Tipos cubiertos

El atributo `tipo_inmueble` contempla: casa, departamento, habitación/pieza, estudio o loft, cabaña, casa anexa, dúplex y otro. `tipo_de_espacio` diferencia inmueble completo, habitación privada, habitación compartida y cama en dormitorio compartido. Esta segunda dimensión permite publicar una pieza dentro de una casa o departamento sin clasificar todo el inmueble como una unidad completa.

La modalidad de cobro se conserva en el campo común `modalidad_tarifa`, con hora, día o mes. Cada publicación mantiene una tarifa base para la unidad seleccionada; el calendario y el motor actual de reservas siguen determinando disponibilidad. No se declara soporte para varios precios simultáneos por publicación ni se añade un segundo calendario.

## Perfil de características propuesto

Los atributos del perfil son opcionales salvo `tipo_inmueble` y `tipo_de_espacio`. La omisión significa “sin informar”. Las selecciones booleanas permiten declarar sí/no; las cantidades pueden ser cero donde corresponda.

| Grupo | Campos |
| --- | --- |
| Identificación | `tipo_inmueble`, `tipo_de_espacio`, `amoblado` |
| Superficies y distribución | superficie construida, terreno, dormitorios, camas, baños completos y medios baños |
| Privacidad y cocina | privacidad del baño (en suite, privado, compartido o sin baño); cocina completa, kitchenette, compartida o no disponible |
| Equipamiento | calefacción, aire acondicionado, agua caliente, Wi-Fi, lavadora, secadora, refrigerador y mobiliario |
| Espacios y acceso | estacionamientos, bodegas, piso, pisos del edificio, ascensor, balcón, terraza, patio, jardín y piscina |
| Condiciones | servicios incluidos (agua, electricidad, gas, internet, gastos comunes, limpieza y ropa de cama); mascotas; visitas; fumadores; reuniones o fiestas |
| Duración | mínimo y máximo de unidades de la modalidad tarifaria (horas, días o meses), cuando se informe |

La superficie general y capacidad que ya exige el formulario siguen siendo obligatorias y comunes a todas las categorías. Los nuevos campos complementan esos datos; Go valida sus tipos/opciones desde el perfil persistido y rechaza atributos desconocidos. La API pública conserva su DTO minimizado y solo expone los atributos declarados por la publicación.

## Alcance de este PR

- Agrega V67 para registrar la categoría y el perfil versión 1; V1–V66 permanecen intactas.
- Extiende el contrato del perfil con `required`; Go rechaza atributos obligatorios ausentes y mantiene las validaciones existentes para categorías anteriores.
- Reutiliza el formulario genérico de publicación para exponer los campos requeridos, y agrega la categoría a la portada.
- Requiere que el baseline de despliegue incluya el archivo V67 antes de permitir un rollout de aplicación. Los releases que incluyen migraciones o tienen migraciones pendientes siguen bloqueados.
- No aplica cambios a una base persistente o remota. La evidencia de migración proviene exclusivamente de una instancia PostgreSQL desechable.

## Límites del primer corte

- Una tarifa base por publicación: hora, día o mes. No representa tres tarifas a la vez.
- La disponibilidad usa el calendario existente; no se introduce inventario de varias unidades dentro de una sola publicación.
- No se publican direcciones privadas, coordenadas exactas, datos de acceso ni información de contacto.
- Sin cobros, firma de contratos, verificación de titularidad del inmueble ni afirmaciones de cumplimiento regulatorio.
- Mantener las migraciones históricas V1–V66 byte a byte para reconstruir y auditar el esquema. V67 se valida tanto desde una instalación vacía como desde V66.
