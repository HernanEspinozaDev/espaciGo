# Anexo: Requerimientos No Funcionales (RNF) — Sistema EspaciGo

> Catálogo de **43 requerimientos no funcionales (RNF-001 a RNF-043)**, clasificados según las categorías y subcategorías de calidad del estándar **ISO/IEC 25010** y del modelo de Sommerville.

**Convenciones aplicadas**

1. **Redacción estándar:** todos los RNF se redactan como **"El sistema debe [condición de calidad]"**, evitando verbos de acción funcional (los verbos de función pertenecen al catálogo de RF). Excepción controlada: los RNF de *Implementación / Gestión de configuración*, redactados como *"El código fuente debe almacenarse..."*, conforme al formato de referencia del catálogo.
2. **Categoría y subcategoría:** cada RNF declara su categoría principal (en negrita en la sección de resumen) y su subcategoría específica.
3. **Mínimo 3 ejemplos por categoría:** todas las categorías declaradas tienen al menos 3 requerimientos (ver sección de resumen).
4. **Separación RF/RNF:** los RNF describen *condiciones de calidad, restricciones o cumplimiento*. Las acciones del sistema (registrar, generar, ejecutar, notificar, etc.) pertenecen exclusivamente al catálogo de RF (Anexo B), y cada RNF expresa una única condición de calidad.
5. **Sin duplicación de RF:** ningún RNF reemplaza a un requerimiento funcional; cuando un RNF se apoya en una función, esta existe además como RF (por ejemplo: boleta electrónica → RQF-176 y RQF-211).

## 1. Catálogo de Requerimientos No Funcionales

| ID | Nombre del requerimiento | Categoría / Subcategoría | Descripción |
| :--- | :--- | :--- | :--- |
| RNF-001 | Tiempo de respuesta del motor de búsqueda | Rendimiento / Comportamiento temporal | El sistema debe retornar los resultados de búsqueda en un tiempo máximo de 2 segundos bajo una carga concurrente de 200 usuarios, medido desde que el usuario confirma los parámetros. |
| RNF-002 | Tiempo de procesamiento del flujo de pago Escrow | Rendimiento / Comportamiento temporal | El sistema debe garantizar que el ciclo de pago retenido (Escrow) se complete en un tiempo máximo de 8 segundos, excluyendo el tiempo de respuesta de la pasarela externa. |
| RNF-003 | Tiempo de generación del contrato PDF | Rendimiento / Comportamiento temporal | El sistema debe garantizar que el contrato en formato PDF esté disponible en un tiempo máximo de 10 segundos desde su solicitud, con un tamaño resultante inferior a 2 MB. |
| RNF-004 | Optimización de recursos multimedia | Rendimiento / Eficiencia de recursos | El sistema debe mantener las imágenes de galería optimizadas en peso antes de su almacenamiento, reduciendo los tiempos de carga de la interfaz. |
| RNF-005 | Acceso a la funcionalidad de reserva en pocos pasos | Usabilidad / Operabilidad | El sistema debe permitir al usuario completar el proceso de reserva en un máximo de 7 interacciones (clics) y sin superar los 3 niveles de profundidad de navegación. |
| RNF-006 | Retroalimentación visual ante errores de validación | Usabilidad / Reconocimiento de errores | El sistema debe mostrar mensajes de error descriptivos (máximo 120 caracteres) en un tiempo inferior a 500 milisegundos tras una validación fallida en formularios, sin recargar la página. |
| RNF-007 | Adaptabilidad visual a dispositivos móviles | Usabilidad / Adaptabilidad visual | El sistema debe presentar sus interfaces sin superposición de elementos en resoluciones desde 375px hasta 1920px de ancho, manteniendo botones con un área mínima de 44x44 píxeles. |
| RNF-008 | Accesibilidad de los flujos principales | Usabilidad / Accesibilidad | El sistema debe permitir operar los flujos de registro, búsqueda y reserva exclusivamente mediante teclado, cumpliendo el estándar de accesibilidad WCAG 2.1 nivel AA. |
| RNF-009 | Disponibilidad continua de la plataforma | Fiabilidad / Disponibilidad | El sistema debe garantizar una disponibilidad mínima del 99,9% mensual para las funciones críticas de autenticación, búsqueda y reservas. |
| RNF-010 | Respaldo y recuperación de la base de datos | Fiabilidad / Recuperabilidad | El sistema debe ejecutar respaldos automáticos de la base de datos que permitan una recuperación con un RPO máximo de 4 horas y un RTO máximo de 6 horas. |
| RNF-011 | Consistencia transaccional en flujos de pago | Fiabilidad / Tolerancia a fallos | El sistema debe garantizar la atomicidad transaccional ejecutando un rollback automático (revirtiendo el estado de la base de datos) ante cualquier fallo en el flujo de pagos. |
| RNF-012 | Idempotencia de los intentos de pago | Fiabilidad / Integridad | El sistema debe garantizar la idempotencia de cada intento de pago mediante una clave única de operación, evitando cargos duplicados ante reintentos del usuario o de la pasarela. |
| RNF-013 | Almacenamiento de contraseñas con hash irreversible | Seguridad / Confidencialidad | El sistema debe almacenar las contraseñas exclusivamente en formato de hash irreversible utilizando bcrypt con un factor de costo mínimo de 12 rounds, prohibiendo su almacenamiento en texto plano. |
| RNF-014 | Cifrado de datos en reposo | Seguridad / Confidencialidad | El sistema debe aplicar cifrado en reposo mediante el algoritmo AES-256 a los datos bancarios y a los documentos PDF finales almacenados. |
| RNF-015 | Protección de comunicaciones y credenciales | Seguridad / Confidencialidad | El sistema debe transmitir credenciales y datos sensibles exclusivamente mediante HTTPS con protocolo TLS 1.2 o superior, impidiendo que los tokens viajen por URL. |
| RNF-016 | Invalidación automática de sesión por inactividad | Seguridad / Autenticidad | El sistema debe invalidar automáticamente el token de sesión tras 30 minutos de inactividad del usuario, con un límite de caducidad absoluta de 8 horas. |
| RNF-017 | Inmutabilidad de los registros de auditoría | Seguridad / Integridad | El sistema debe preservar los logs de auditoría transaccional en un repositorio inmutable, bloqueando a nivel de infraestructura cualquier intento de modificación o eliminación de los registros. |
| RNF-018 | Eliminación irreversible de datos personales | Seguridad / Privacidad | El sistema debe garantizar la eliminación irreversible de los datos personales identificables (PII) tras aprobarse la baja de una cuenta de usuario. |
| RNF-019 | Escalabilidad horizontal del servicio | Flexibilidad / Escalabilidad | El sistema debe soportar un escalamiento horizontal automático para absorber incrementos del 200% de carga, desplegando nuevas instancias en un tiempo máximo de 10 minutos. |
| RNF-020 | Modificación de parámetros sin redespliegue | Flexibilidad / Modificabilidad | El sistema debe permitir la modificación de parámetros de negocio (como el porcentaje de comisión) aplicando los cambios en máximo 60 segundos sin requerir reinicio del servidor. |
| RNF-021 | Cobertura mínima de pruebas automatizadas | Mantenibilidad / Testeabilidad | El sistema debe mantener una cobertura de pruebas unitarias mínima del 80% sobre la capa de negocio, forzando el fallo del proceso de integración (CI/CD) si desciende de este umbral. |
| RNF-022 | Compatibilidad con navegadores actuales | Compatibilidad / Interoperabilidad | El sistema debe ejecutarse correctamente en las dos últimas versiones estables de Chrome, Firefox, Edge y Safari. |
| RNF-023 | Tolerancia a degradación de servicios API externos | Compatibilidad / Interoperabilidad | El sistema debe abortar las llamadas a APIs externas tras un timeout de 5 segundos, reintentando automáticamente un máximo de 2 veces con intervalos de 2 segundos antes de reportar la caída del servicio. |
| RNF-024 | Recepción y recuperación de webhooks | Compatibilidad / Interoperabilidad | El sistema debe validar la firma de los webhooks entrantes y reintentar su procesamiento hasta 5 veces con un backoff exponencial ante fallos de conexión externa. |
| RNF-025 | Cumplimiento PCI-DSS en datos de tarjetas de pago | Requerimientos Externos / Estándares | El sistema debe procesar los pagos exclusivamente mediante tokenización, sin almacenar en la base de datos los datos PAN o CVV, para dar estricto cumplimiento al estándar PCI-DSS. |
| RNF-026 | Protección de datos personales conforme a la Ley 21.719 | Requerimientos Externos / Legislativos | El sistema debe procesar las solicitudes de eliminación o rectificación de datos personales en un plazo administrativo máximo de 72 horas, en cumplimiento de la Ley 21.719. |
| RNF-027 | Boleta electrónica conforme a la normativa del SII | Requerimientos Externos / Legislativos | El sistema debe cumplir la normativa de boleta electrónica del SII, garantizando que toda boleta emitida por la plataforma quede timbrada y disponible en formato XML y PDF dentro de un plazo máximo de 24 horas. |
| RNF-028 | Conciliación de pagos sin confirmación | Fiabilidad / Integridad | El sistema debe conciliar automáticamente cada 5 minutos los eventos de pago que no registren confirmación, dejando evidencia del resultado de la conciliación para su revisión. |
| RNF-029 | Anonimización de la información transaccional | Seguridad / Privacidad | El sistema debe garantizar la anonimización de la información transaccional histórica tras aprobarse la baja de una cuenta, preservando las obligaciones de auditoría. |
| RNF-030 | Capacidad de usuarios y escrituras concurrentes | Rendimiento / Capacidad | El sistema debe soportar al menos 500 usuarios concurrentes y 100 operaciones de escritura por segundo sobre la base de datos transaccional, sin degradar los tiempos de respuesta definidos. |
| RNF-031 | Adaptabilidad funcional del catálogo | Flexibilidad / Adaptabilidad | El sistema debe permitir incorporar nuevos tipos de inmueble y nuevos métodos de pago sin modificar los módulos existentes. |
| RNF-032 | Documentación del contrato de la API | Mantenibilidad / Analizabilidad | El sistema debe mantener documentado el contrato de su API REST (especificación OpenAPI) y actualizado con cada versión desplegada. |
| RNF-033 | Modularidad del código por dominio | Mantenibilidad / Modularidad | El sistema debe organizarse en módulos desacoplados por dominio funcional, con interfaces documentadas entre ellos. |
| RNF-034 | Despliegue agnóstico del proveedor de nube | Portabilidad / Adaptabilidad | El sistema debe desplegarse sobre contenedores compatibles con cualquier proveedor de nube que soporte Docker, sin dependencias propietarias del entorno. |
| RNF-035 | Reproducción del entorno local | Portabilidad / Instalabilidad | El sistema debe poder reproducirse íntegramente en un entorno local mediante contenedores, incluyendo la base de datos y los servicios de apoyo. |
| RNF-036 | Reemplazabilidad del proveedor de nube | Portabilidad / Reemplazabilidad | El sistema debe poder desplegarse en al menos dos proveedores de nube compatibles con contenedores, sin modificar el código fuente. |
| RNF-037 | Implementación en contenedores | Implementación / Empaquetado | El sistema debe estar implementado en contenedores Docker, garantizando la consistencia entre los entornos de desarrollo, pruebas y producción. |
| RNF-038 | Motor de base de datos relacional | Implementación / Base de datos | El sistema debe utilizar un motor de base de datos relacional (PostgreSQL) con soporte de transacciones ACID y extensiones geoespaciales. |
| RNF-039 | Control de versiones del código fuente | Implementación / Gestión de configuración | El código fuente debe almacenarse en un repositorio Git corporativo (GitHub o equivalente), con control de versiones y revisión de cambios. |
| RNF-040 | Integración y despliegue continuos | Implementación / Despliegue | El sistema debe contar con un pipeline de integración y despliegue continuos (CI/CD) que ejecute las pruebas automatizadas antes de cada despliegue. |
| RNF-041 | Capacidad de almacenamiento por publicación | Almacenamiento / Capacidad | El sistema debe limitar el almacenamiento de archivos por publicación a 50 MB, considerando un máximo de 10 imágenes de 5 MB. |
| RNF-042 | Retención de contratos y evidencias | Almacenamiento / Retención | El sistema debe conservar los contratos firmados y la evidencia fotográfica de las reservas por un plazo mínimo de 5 años. |
| RNF-043 | Retención de los registros de auditoría | Almacenamiento / Retención | El sistema debe conservar los registros de auditoría por un plazo mínimo de 5 años, conforme a los requisitos de trazabilidad del proyecto. |

## 2. Resumen por categoría (mínimo 3 RNF por categoría)

| Categoría | Subcategorías incluidas | Cantidad | RNF |
| :--- | :--- | :---: | :--- |
| **Rendimiento** | Comportamiento temporal, Eficiencia de recursos, Capacidad | 5 | 001, 002, 003, 004, 030 |
| **Usabilidad** | Operabilidad, Reconocimiento de errores, Adaptabilidad visual, Accesibilidad | 4 | 005, 006, 007, 008 |
| **Fiabilidad** | Disponibilidad, Recuperabilidad, Tolerancia a fallos, Integridad | 5 | 009, 010, 011, 012, 028 |
| **Seguridad** | Confidencialidad, Autenticidad, Integridad, Privacidad | 7 | 013, 014, 015, 016, 017, 018, 029 |
| **Flexibilidad** | Escalabilidad, Modificabilidad, Adaptabilidad | 3 | 019, 020, 031 |
| **Mantenibilidad** | Testeabilidad, Analizabilidad, Modularidad | 3 | 021, 032, 033 |
| **Compatibilidad** | Interoperabilidad | 3 | 022, 023, 024 |
| **Portabilidad** | Adaptabilidad, Instalabilidad, Reemplazabilidad | 3 | 034, 035, 036 |
| **Implementación** | Empaquetado, Base de datos, Gestión de configuración, Despliegue | 4 | 037, 038, 039, 040 |
| **Almacenamiento** | Capacidad, Retención | 3 | 041, 042, 043 |
| **Requerimientos Externos** | Estándares (PCI-DSS), Legislativos (Ley 21.719, SII) | 3 | 025, 026, 027 |
| **Total** | — | **43** | RNF-001 – RNF-043 |

## 3. Trazabilidad con los requerimientos funcionales y los módulos

| RNF | Se relaciona con los RF | Módulos / casos de uso afectados |
| :--- | :--- | :--- |
| RNF-001, RNF-030 | RQF-094, RQF-101, RQF-111 | M05 (CU-19), M06 (CU-22) |
| RNF-002, RNF-011, RNF-012, RNF-028 | RQF-114 a RQF-120, RQF-172, RQF-173 | M06 (CU-24, CU-27), M10 (CU-42) |
| RNF-003 | RQF-132, RQF-133 | M07 (CU-29) |
| RNF-004, RNF-041 | RQF-074 a RQF-076 | M04 (CU-16) |
| RNF-005, RNF-006 | RQF-107, RQF-108, RQF-113 | M05 (CU-21), M06 (CU-22) |
| RNF-007, RNF-008, RNF-022 | Todos los flujos de interfaz | Transversal |
| RNF-009, RNF-010 | Funciones críticas (autenticación, búsqueda, reservas) | Transversal |
| RNF-013, RNF-015, RNF-016 | RQF-003 a RQF-006, RQF-011 a RQF-018, RQF-215 a RQF-217 | M01 (CU-01, CU-03, CU-50) |
| RNF-014 | RQF-032, RQF-137 | M02 (CU-08), M07 (CU-30) |
| RNF-017, RNF-043 | RQF-184, RQF-185 | M11 (CU-46) |
| RNF-018, RNF-029, RNF-026 | RQF-034 a RQF-037 | M02 (CU-09) |
| RNF-019, RNF-031 | RQF-068, RQF-114 | M04 (CU-15), M06 (CU-24) |
| RNF-020 | RQF-105, RQF-223 | M05 (CU-21), M04 (CU-18) |
| RNF-021, RNF-032, RNF-033, RNF-040 | — | Transversal (arquitectura y construcción) |
| RNF-023, RNF-024 | RQF-044, RQF-051, RQF-133, RQF-148 | M03 (CU-11, CU-12), M07 (CU-29) |
| RNF-025 | RQF-114, RQF-115 | M06 (CU-24) |
| RNF-027 | RQF-176, RQF-211 | M10 (CU-42) |
| RNF-034 a RNF-039 | — | Transversal (infraestructura y arquitectura TI) |
| RNF-042 | RQF-137, RQF-167, RQF-205 | M07 (CU-30), M08 (CU-33, CU-48), M10 (CU-41) |
