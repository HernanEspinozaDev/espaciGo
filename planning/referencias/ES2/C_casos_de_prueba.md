# Catálogo de casos de prueba de ES2

Todos los casos siguientes están **planificados y sin ejecutar**. Proceden de los requisitos, casos de uso e historias de ES1 y del diseño propuesto en ES2. Usar datos sintéticos, código y esquema versionados y un reloj controlable cuando se prueben plazos. Un simulador de proveedor solo demuestra la lógica local.

Para cada ejecución se deberá agregar un registro con ID de caso, fecha/hora, versión de código y datos, entorno, ejecutor, resultado observado, evidencia (log/captura/reporte), defecto y reejecución. Ningún resultado se anticipa en este catálogo.

## PT-01 — Validación de intervalo

- **Tipo y traza:** unitaria; RQF-108–112, CU-22/23, KPI-10.
- **Precondición y datos:** función de validación disponible; espacio sintético con un intervalo ocupado. Probar inicio pasado, fin igual o anterior al inicio, intervalo solapado y uno adyacente al fin de la ocupación.
- **Pasos:** invocar la regla con cada combinación y consultar la decisión y su motivo.
- **Resultado esperado y aprobación:** los tres casos inválidos se rechazan sin crear reserva; el adyacente se trata según la convención semiabierta propuesta, después de aprobarla. Todas las salidas deben coincidir con la tabla de expectativas revisada.
- **Entorno y evidencia:** prueba unitaria con reloj fijo; reporte de casos y aserciones. Estado: planificada.

## PT-02 — Doble reserva concurrente

- **Tipo y traza:** integral/concurrencia; RQF-111/112, RNF-030, CU-22/23, KPI-10.
- **Precondición y datos:** PostgreSQL con esquema de ocupación y un espacio libre; dos solicitudes sintéticas simultáneas para el mismo intervalo. Repetir frente a un bloqueo manual.
- **Pasos:** sincronizar el inicio de ambas transacciones, confirmar resultados y consultar reserva y ocupación activa después de cada carrera.
- **Resultado esperado y aprobación:** a lo sumo una reserva incompatible se confirma; la otra recibe rechazo consistente. Ningún fallo deja reserva sin ocupación o bloqueo ignorado. Repetir el ensayo con semillas documentadas.
- **Entorno y evidencia:** integración con PostgreSQL real de ensayo, no solo mock; transacciones, consultas de integridad y reporte de concurrencia. Estado: planificada.

## PT-03 — Pago pendiente y confirmación tardía

- **Tipo y traza:** integración/tiempo; RQF-120/200, RNF-012/028, CU-27.
- **Precondición y datos:** reserva «Pendiente de Pago», reloj controlable, adaptador financiero de ensayo con respuesta configurable.
- **Pasos:** observar el estado antes de 15 minutos; avanzar hasta el vencimiento sin pago confirmado; después inyectar una confirmación tardía o una respuesta incierta.
- **Resultado esperado y aprobación:** no se cancela antes del plazo; se aplica la regla de cancelación cuando el estado financiero es conocido. Una respuesta incierta queda correlacionada para conciliación y no libera automáticamente la ocupación ni inicia otro cargo. Se documenta el resultado de la confirmación tardía antes de cerrar el caso.
- **Entorno y evidencia:** API, BD, reloj y simulador; historial de estados y eventos. La compatibilidad del proveedor real queda aparte. Estado: planificada.

## PT-04 — Idempotencia y webhook

- **Tipo y traza:** integración/fallo; RNF-012/024/028, CU-24/25, KPI-11.
- **Precondición y datos:** misma reserva y clave de idempotencia; evento válido, duplicado, fuera de orden y con firma inválida.
- **Pasos:** repetir la solicitud de pago; entregar dos veces el mismo evento, después el fuera de orden y el no autenticado; ejecutar conciliación.
- **Resultado esperado y aprobación:** una sola operación local por clave, ningún evento sin firma válida aplicado, duplicados sin efectos repetidos y respuesta incierta conservada para conciliación. No se declara ausencia de cargo real usando solo simulación.
- **Entorno y evidencia:** adaptador de ensayo, API y BD; contador de solicitudes, tabla de eventos, logs sin secretos y estado final. Estado: planificada.

## PT-05 — Respuesta del arrendador y vencimiento

- **Tipo y traza:** integración/tiempo; RQF-123–129, CU-26/28.
- **Precondición y datos:** solicitud pagada pendiente de decisión; escenarios de aprobación, rechazo con motivo, rechazo sin motivo y silencio durante 24 horas.
- **Pasos:** enviar cada decisión y avanzar reloj hasta el límite; consultar reserva, ocupación y operación de devolución.
- **Resultado esperado y aprobación:** aprobación habilita contrato; rechazo sin motivo no se procesa; rechazo válido o vencimiento activa cancelación y reembolso conforme a estado financiero confirmado. Una devolución incierta queda visible para conciliación.
- **Entorno y evidencia:** API, BD y proveedor simulado; transiciones y correlación de reembolso. Estado: planificada.

## PT-06 — Firma completa y falta de firma

- **Tipo y traza:** integración/tiempo; RQF-130–142, CU-29–32.
- **Precondición y datos:** reserva aprobada, contrato versionado y dos partes; respuestas de firma individuales y reloj hasta fecha de inicio.
- **Pasos:** registrar primera firma, intentar check-in; registrar segunda y repetir; en otra reserva llegar al inicio con firma incompleta.
- **Resultado esperado y aprobación:** primera firma deja estado parcial e impide ingreso; dos firmas confirmadas almacenan contrato final y permiten el estado de ingreso; vencimiento sin firmas causa cancelación y reembolso conforme a ES1. Respuesta de firma incierta no se interpreta como confirmación.
- **Entorno y evidencia:** API, archivos de prueba y adaptador de firma simulado; versión de contrato, estados, hashes y eventos. Estado: planificada.

## PT-07 — Permiso de resolución de disputa

- **Tipo y traza:** seguridad funcional/integración; RQF-166–171, CU-41.
- **Precondición y datos:** disputa abierta, cuentas sintéticas de arrendatario, arrendador y administrador con permisos separados.
- **Pasos:** intentar consultar y resolver con cada rol; registrar un fallo con cuenta administrativa válida.
- **Resultado esperado y aprobación:** las partes solo acceden a las acciones y evidencias que les correspondan; ninguna puede emitir fallo administrativo. El administrador autorizado registra resolución y motivo con trazabilidad. Las denegaciones no modifican datos.
- **Entorno y evidencia:** API y BD de ensayo; respuestas HTTP, auditoría y estado anterior/posterior. Estado: planificada.

## PT-08 — Reclamo y liquidación

- **Tipo y traza:** integración; RQF-159–177/208–211, CU-39–42.
- **Precondición y datos:** reserva finalizada, ventana de 24 horas, garantía y fondos simulados; escenarios con reclamo dentro de plazo y sin reclamo. Para el ensayo del proveedor, disponer de vendedor de prueba con y sin saldo.
- **Pasos:** abrir reclamo con evidencia, intentar liquidar; agregar descargos y resolución; conciliar y liquidar según fallo. En otra reserva dejar pasar el plazo sin reclamo. Registrar total del comprador, tarifa del procesador, comisión y saldo del arrendador; intentar reembolso con saldo insuficiente y conciliar el resultado externo.
- **Resultado esperado y aprobación:** un reclamo abierto bloquea liberación; el cierre solo ocurre tras operación financiera confirmada. Sin reclamo se aplica el flujo previsto después del plazo; evento repetido no duplica liquidación. El reembolso fallido o parcial permanece pendiente y visible, sin afirmar devolución completa ni usar la garantía como caja de EspaciGo.
- **Entorno y evidencia:** API, BD, archivos y adaptador financiero de ensayo; más sandbox del proveedor cuando se habilite. Guardar auditoría, estados, tarifas y saldos antes/después y operación correlacionada. Una simulación local no acredita capacidad de retención o reembolso del proveedor. Estado: planificada.

## PT-09 — Rendimiento de búsqueda

- **Tipo y traza:** carga; RNF-001, KPI-09.
- **Precondición y datos:** catálogo geográfico y mezcla de filtros versionados; entorno, red y herramienta identificados.
- **Pasos:** establecer referencia sin carga y ejecutar búsquedas con 200 usuarios concurrentes durante una ventana acordada; registrar cada latencia y error.
- **Resultado esperado y aprobación:** cada respuesta válida cumple ≤ 2 segundos conforme a RNF-001; informar también máximo, percentiles y tasa de errores. No sustituir la meta por un promedio favorable.
- **Entorno y evidencia:** entorno de carga reproducible; configuración, semillas, series de tiempo y reportes. Estado: planificada.

## PT-10 — Capacidad, estrés y escalado

- **Tipo y traza:** carga/estrés; RNF-019/030.
- **Precondición y datos:** base de ensayo con catálogo y reservas sintéticas; límites de instancias y recursos documentados.
- **Pasos:** aumentar hasta 500 usuarios concurrentes y 100 escrituras/s; aplicar un incremento del 200 % respecto de la carga de referencia acordada y observar instancias y latencia.
- **Resultado esperado y aprobación:** capacidad objetivo sin violar latencias o integridad; nuevas instancias útiles en ≤ 10 minutos si se adopta RNF-019. Si el entorno no permite esa carga, registrar la limitación y no declarar aprobación.
- **Entorno y evidencia:** despliegue de carga separado, métricas de cómputo y BD, errores y consulta de integridad. Estado: planificada.

## PT-11 — Respaldo y recuperación

- **Tipo y traza:** recuperación; RNF-010, SLA-05.
- **Precondición y datos:** respaldo verificable, conjunto de transacciones con marcas de tiempo y ambiente de restauración aislado.
- **Pasos:** confirmar operaciones, simular pérdida controlada, restaurar copia y ejecutar pruebas de humo e integridad.
- **Resultado esperado y aprobación:** diferencia de datos recuperados ≤ 4 horas (RPO) y funciones críticas restablecidas ≤ 6 horas (RTO), medidas desde disparadores registrados. No usar la frecuencia de respaldo como sustituto del ensayo.
- **Entorno y evidencia:** infraestructura de ensayo; manifiesto de respaldo, cronología, hashes y resultados de humo. Estado: planificada.

## PT-12 — PDF contractual

- **Tipo y traza:** rendimiento/integración; RNF-003, CU-29.
- **Precondición y datos:** reserva aprobada con datos sintéticos completos y versión de plantilla contractual definida.
- **Pasos:** solicitar generación, cronometrar hasta disponibilidad, medir bytes y revisar correspondencia de campos y versión.
- **Resultado esperado y aprobación:** PDF correcto disponible en ≤ 10 segundos y tamaño < 2 MB. La validez jurídica de la firma requiere verificación separada.
- **Entorno y evidencia:** generador y almacenamiento de ensayo; tiempos, archivo, tamaño y validación de contenido. Estado: planificada.

## PT-13 — Humo del recorrido mínimo

- **Tipo y traza:** humo; CU-01/22/24/29/30/33 y dependencias.
- **Precondición y datos:** versión desplegada y componentes del recorrido identificados; usuarios y espacio sintéticos.
- **Pasos:** registrar o acceder, buscar, solicitar reserva, procesar pago de ensayo, aprobar, firmar e intentar ingreso. Marcar explícitamente cada etapa aún no construida.
- **Resultado esperado y aprobación:** los componentes disponibles responden y conservan estado coherente; no se declara recorrido completo mientras una etapa crítica dependa de un placeholder o simulador sin indicarlo.
- **Entorno y evidencia:** ensayo integrado; versión, pasos, capturas y transiciones. Estado: planificada.

## PT-14 — Alfa y aceptación interna

- **Tipo y traza:** alfa/aceptación; HU y criterios de aceptación del alcance que apruebe el equipo.
- **Precondición y datos:** versión candidata, guion por HU, participantes internos y datos sintéticos; responsables por confirmar.
- **Pasos:** recorrer tareas de arrendatario, arrendador y administrador según el alcance real; registrar observación y defecto contra cada criterio.
- **Resultado esperado y aprobación:** criterio solo se marca aceptado con evidencia y revisor identificado; defectos críticos bloquean aprobación. No se infiere aprobación docente.
- **Entorno y evidencia:** entorno alfa; acta de sesión, guion, capturas sin datos sensibles y lista de defectos. Estado: planificada.

## PT-15 — Beta y aceptación externa

- **Tipo y traza:** beta/aceptación; HU del alcance demostrable y metas de experiencia por acordar.
- **Precondición y datos:** prototipo estable, participantes externos autorizados, consentimiento y tratamiento de datos definidos.
- **Pasos:** asignar tareas observables, registrar terminación, errores y comentarios; contrastar resultados con criterios acordados antes de la sesión.
- **Resultado esperado y aprobación:** resultados trazables por tarea y participante, sin convertir opiniones aisladas en cumplimiento general. Si no hay acceso a usuarios o servicio real, declarar beta no ejecutada y explicar la limitación.
- **Entorno y evidencia:** entorno beta controlado; protocolo, acta anonimizada, métricas y defectos. Estado: planificada.

## PT-16 — Derechos de titulares y ciclo de datos

- **Tipo y traza:** integración/seguridad; RQF-034–037, RNF-018/026/029/042/043, matriz del Anexo B de ES2 y Ley 21.719 como criterio de diseño.
- **Precondición y datos:** políticas de finalidad, acceso y conservación revisadas por categoría; cuentas sintéticas con y sin reservas activas, pagos pendientes y disputa abierta; datos y documentos identificables en varios módulos.
- **Pasos:** consultar y rectificar datos autorizados; solicitar cierre, supresión o bloqueo; intentar acceso con otro rol; repetir con cada obligación pendiente; revisar historial, documentos, registros y exportación cuando se implemente portabilidad.
- **Resultado esperado y aprobación:** se verifica identidad y autorización; cada solicitud recibe estado y decisión trazable; las restricciones RQF-035–037 impiden un cierre improcedente; los datos eliminables se suprimen o anonimizan según política aprobada, sin destruir hechos que deban conservarse; no aparecen PII en respuestas o logs de otro usuario. Medir el tiempo frente a la meta interna RNF-026 sin llamarla plazo legal demostrado. Si falta una regla de retención aprobada, el caso queda bloqueado, no aprobado.
- **Entorno y evidencia:** API y BD con datos sintéticos; matriz aprobada, solicitudes, decisiones, permisos, datos antes/después y reporte de prueba depurado. Estado: planificada.

El catálogo ha sido validado contra las herramientas (GitHub Actions, k6 y GCP Staging) y la Matriz de Tratamiento de Privacidad consolidada (Anexo B). La ejecución de este plan se registrará iterativamente en los reportes de calidad, vinculando cada ejecución con los rastreadores de incidentes en GitHub.
