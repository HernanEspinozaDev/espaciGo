# Pendientes de ES2PT

Este archivo responde a una pregunta: **qué falta y quién puede cerrarlo**. Está agrupado por el hecho que desbloquea cada tarea, porque lo que depende de un tercero, del equipo o del producto no avanza por más redacción que se haga. El [plan de cierre](plan_cierre_pendientes.md) ordena dependencias, fechas objetivo y evidencia; el detalle de lo cumplido está al final.

**Regla:** un `[x]` exige evidencia registrada y una meta no demuestra un resultado. `validar ES2PT --final` bloquea la entrega mientras queden marcadores de borrador o tareas abiertas.

**Estado de revisión:** el contenido está redactado de punta a punta y los anexos están ordenados por primera cita (A = evaluación económica, B = diccionario de datos, C = catálogo de casos de prueba). INV-033 cerró tres decisiones documentales; el 29-09-2026 se sustituyó el núcleo parcial del Anexo B por el diccionario de diseño de **43 tablas** y se sincronizó la propuesta backend con el producto completo. Quedan **13 marcadores `PENDIENTE`** en secciones/anexos (10 + 1 en A + 2 en B). El Anexo A y 2.1 ya están recalculados con 3 % neto, tarifa del vendedor e instancia API mínima; los datos siguen siendo supuestos. El **29-09-2026** se generó una nueva copia Word de lectura del informe y los anexos A/B/C con TIH184; `validar ES2PT` terminó sin errores y conservó tres avisos por pendientes. Esta salida aún requiere actualizar los índices en Word y revisión visual final cuando cierre el contenido. La revisión de 91 páginas del 24-09-2026 es histórica y no valida los cambios recientes. La validación final depende de evidencia de terceros, del equipo y del producto. El plan por sección está en [plan_de_trabajo.md](plan_de_trabajo.md) y el avance vivo en la [bitácora de coordinación](coordinacion/bitacora.md).

El detalle operativo preparado antes de esta actualización —decisiones, consultas a terceros, producto, economía y reparto— está en [`investigacion/investigacionmanual/`](investigacion/investigacionmanual/README.md). Su mapa anterior de 14 marcadores es histórico. La [auditoría vigente de los 17 criterios](investigacion/investigacionmanual/08_auditoria_avance_es2_2026_09_29.md) y el [reparto propuesto para usuario, Shiva y Tajamon](investigacion/investigacionmanual/09_reparto_equipo_es2_2026_09_29.md) organizan los **13 marcadores actuales** y el inicio del código local.

## 1. Falta con terceros

Ninguna de estas tareas se cierra con trabajo documental: requieren una cotización, una credencial, una entrevista o una medición real. [INV-027](investigacion/INV-027_gestion_terceros.md) reúne el estado de las diez filas, las consultas preparadas y la evidencia precisa que falta. [INV-028](investigacion/INV-028_sensibilidad_costos_terceros.md) cuantifica escenarios de firma y asesoría; **no reemplaza cotizaciones**. Las consultas no se han enviado y ninguna fila se marca como cumplida.

| Qué falta | Quién lo cierra | Evidencia que lo cerraría |
| --- | --- | --- |
| Confirmar la tarifa contractual, su IVA y receptor de documento, el saldo real de cada parte, reembolsos, garantía y liberación condicionada; el escenario vigente asigna la tarifa al vendedor según la guía pública | Proveedor de pago, cuenta de empresa y entorno de pruebas | Liquidación detallada de un caso real y un reembolso con vendedor sin saldo (INV-018) |
| **Mercado Pago, parcialmente confirmado en INV-026 (24-09-2026).** Confirmado con documentación oficial: el reparto 1:1 está disponible en Chile para Checkout Pro o API, exige cuenta con KYC 6, OAuth por arrendador y cuentas de prueba; la comisión de Mercado Pago se descuenta **primero al vendedor**; el reporte de ventas separa tarifa del marketplace, tarifa de Mercado Pago y neto. **Sigue abierto:** (a) si «dinero en cuenta» restringe el **medio de pago** del comprador — es la pregunta que más afecta al producto; (b) la **tarifa contractual** de Split, porque la página de costos del proveedor ya no responde; (c) si la **cuenta de empresa** de la SpA cumple KYC 6; (d) el nivel de identificación exigido a **cada arrendador** vinculado; (e) el ensayo en cuentas de prueba, aún sin ejecutar | Ejecutivo comercial de Mercado Pago, cuenta de empresa y contador | Respuesta escrita a las diez preguntas de INV-026 y acta del ensayo en las cuentas de prueba con cobro, reparto, reembolso y contracargo |
| Cotizar la firma electrónica por documento y tipo jurídico, y la verificación de identidad por usuarios nuevos y revalidaciones | Proveedor de firma e identidad | Cotización o contrato formal, comparable con la tarifa pública de INV-012 |
| Confirmar el IVA y el crédito fiscal de los servicios de Google Cloud y el tipo de cambio aplicado | Contador y comprobantes de facturación | Primera factura pagada y su registro contable; hoy el presupuesto usa USD/CLP = 1.000 y 19 % supuestos |
| Confirmar la propuesta de giro **631200 principal y 731001 complementario**, régimen, documentos y capital pagable de la SpA; consultar si la comisión exige 682000 u otra actividad | Contador y SII | Respuesta particular según contrato y prestaciones. **682000 no integra la propuesta actual**; los destaques aún no se venden. Determinar IVA, DTE y verificación de actividad para cada servicio |
| Verificar el domicilio Oficina Express de Ahumada 131, la **única patente comercial** de la SpA, eventuales trámites DOM y aseo municipal, y cotizar honorarios legales y contables | Municipalidad de Santiago, Oficina Express y profesionales | Contrato/derecho de uso, zonificación, exigencias DOM, liquidación de patente/aseo y presupuestos; la oferta pública y el mínimo legal son solo referencias (INV-029) |
| Medir oferta, demanda, precio final, duración, capacidad, ocupación y aceptación de la comisión en **cada** tipo de arriendo | Trabajo de campo con arrendadores y arrendatarios | Fichas del protocolo de INV-016 con fuente, consentimiento y muestra por categoría |
| Decidir si el destaque pagado entra en ES2 o queda para una fase futura, con precio, zona, cupos y política de pausas y reembolsos | Equipo y clientes piloto | Prueba con tráfico y oferta aprobada, con aceptación y canibalización medidas (INV-013) |
| Ejecutar una campaña de Meta Ads y medir el costo por publicación y por reserva pagada | Equipo, cuenta publicitaria y el presupuesto ya supuesto | Panel de campaña y eventos verificados; hoy no hay campaña ni retorno atribuido (INV-015) |
| Habilitar el acceso a Registro Civil, SII y pasarelas | Los organismos y proveedores | Credenciales y una primera consulta registrada, sin secretos en el repositorio (INV-003) |

## 2. Falta con el equipo

Acuerdos internos: nadie más puede tomarlos.

| Qué falta | Evidencia que lo cerraría |
| --- | --- |
| Fijar la capacidad semanal, los responsables por bloque y la rotación de jefatura | Acuerdo registrado en el cronograma y en esta lista |
| Registrar el avance realmente implementado para el 3 de noviembre dentro del producto completo ya definido | Evidencia de funciones construidas, limitaciones y entorno; el diseño ya no se reduce a una categoría |
| Revisar los cuatro BPMN, las siete vistas de casos de uso y el modelo de estados y permisos | Acta de revisión con las observaciones resueltas |
| Validar las interfaces entre componentes, la ejecución de la conciliación y las decisiones de red | Conformidad del equipo técnico sobre 3.3 y 3.5 |
| Resolver el mecanismo de inmutabilidad de RNF-017 (cadena de hash, retención bloqueada o destino de solo anexado) | Decisión con costo y retención, más una prueba de alteración fallida |
| Acordar aportes, pacto de socios, propiedad intelectual y poderes antes de los estatutos | Acuerdo escrito; el capital hoy es una propuesta |
| Confirmar los metadatos académicos restantes: sección, académico y fechas formativas. El usuario confirmó **TIH184** para la asignatura y ya consta en `informe.json` | Instrumento oficial o confirmación del docente para los demás campos (INV-025) |
| Confirmar con el instrumento oficial el contraste de INV-025 y registrar la revisión docente de los 17 criterios | Retroalimentación recibida y registrada con fecha |

## 3. Falta con el producto

Todo esto exige implementación y un entorno de pruebas.

| Qué falta | Evidencia que lo cerraría |
| --- | --- |
| Escribir y aplicar migraciones PostgreSQL 18 derivadas del diccionario de 43 tablas; ejecutar los trece ensayos del modelo (MD-01 a MD-13), incluida atomicidad y recuperación del Outbox | Base desplegada y resultados fechados, incluida la prueba concurrente del calendario |
| Ejecutar los dieciséis casos de prueba del Anexo C (PT-01 a PT-16) | Informe por caso con entorno, datos y resultado |
| Ejecutar los ensayos comparativos de tecnologías: latencia y concurrencia del flujo seleccionado y costo de Cloud Run frente a VM con SKU comparables en Santiago | Registro de ensayo con entorno, versión, carga y resultados, más la ratificación del equipo ([INV-033](investigacion/INV-033_cierre_borrador.md)) |
| Medir los once KPI y los cinco SLA y llenar el registro de mediciones | Serie de mediciones con fuente y responsable; hoy está vacío |
| Ensayar respaldo, restauración y continuidad (RPO de 4 h y RTO de 6 h) | PT-11 con tiempos medidos y evidencia de restauración |
| Probar carga y escalado (200 usuarios concurrentes y 500 con 100 escrituras por segundo) | PT-09 y PT-10 con percentiles y trazas |
| Demostrar la portabilidad de RNF-034–036 | Recorrido reproducido en contenedores locales y con un proveedor alternativo |
| Ensayar la alteración de RNF-017 con retención bloqueada y acordar el plazo productivo con la matriz de tratamiento | Prueba de alteración fallida con evidencia fechada y política de retención aprobada (SUP-13) |
| Levantar el inventario físico de las estaciones de trabajo y fijar los parches e imágenes efectivamente instalados | Inventario con sistema operativo, CPU, memoria, almacenamiento y conectividad, más el archivo de dependencias (SUP-20) |
| Auditar por SKU y factura el presupuesto ya recalculado de Cloud Run con facturación por instancia y mínimo 1; fijar máximo de instancias y pool de conexiones | Exportación comparable del calculador, configuración real y factura, sin confundir cálculo documental con gasto observado |
| Desplegar monitoreo y alertas y acordar la ventana de mantención | Sondeos activos, umbral de dos fallos consecutivos y acuerdo de excluir la ventana |
| Implementar los procedimientos de continuidad, cambios e incidentes | Simulacro ejecutado y registro del incidente |

## 4. Cierre documental

Se resuelve al cerrar lo anterior; no requiere evidencia externa nueva.

- [ ] Resolver con evidencia los **13 marcadores `PENDIENTE`** vigentes: 10 en secciones, 1 en el Anexo A y 2 en el Anexo B. El conteo anterior de INV-033 (14) corresponde a una versión previa; los pendientes actuales dependen de terceros, del equipo o de producto desplegado.
- [ ] Repetir la generación y la revisión visual cuando se cierren los 13 marcadores: la salida del 24-09-2026 fue revisada, pero el nuevo diccionario y la sincronización de backend la dejaron desactualizada.
- [ ] Registrar el año de las fuentes web cuando se identifique: el estilo ya emite «s. f.» y la lista cumple APA 7, así que es una mejora, no un defecto.
- [ ] Sustituir los supuestos económicos —ticket de 100.000 CLP, reparto sintético por categoría, 0/720/1.800 reservas, aceptación de la comisión del 3 %, tarifa contractual de pasarela y firma— por evidencia medida.
- [ ] Ejecutar los ensayos comparativos de tecnologías y el inventario físico: la comparación ponderada, las fichas de versiones y licencias y el alojamiento ya están redactados con supuestos trazables (SUP-19 y SUP-20).
- [ ] Reponer en el Anexo A el escenario que resulte de las cotizaciones de pago, firma e identidad.

## 5. Cumplido, con su evidencia

| Bloque | Evidencia |
| --- | --- |
| Línea base, brechas y trazabilidad | `INV-001` con las brechas B-01 a B-11 y la matriz de los 17 criterios |
| Capítulo II: tecnologías, factibilidad y economía | `02_01` y `02_02` con investigación por capa, tarifas de Santiago aplicadas y simulación reproducible |
| Capítulo III: diseño completo | Siete vistas de casos de uso (41 de 52 CU), cuatro BPMN con mensajes y temporizadores, diccionario vigente de 43 tablas y contrato backend sincronizado; migraciones PG18 aún por escribir/aplicar, además de interfaces, red, infraestructura y fallos previstos |
| Capítulo IV: KPI y SLA con método | Once fichas y cinco fichas con ventana, entradas, exclusiones y evidencia (`INV-005`) |
| Capítulo V: pruebas y normas | Plan, dieciséis casos y matriz normativa con la Ley 21.719 como criterio de diseño (`INV-006`, `INV-022`) |
| Capítulo VI: operación | Disponibilidad, continuidad y mantención con herramientas y umbrales propuestos (`INV-007`, `INV-023`) |
| Capítulo VII: cronograma | Siete hitos con evidencia de cierre, dependencias, esfuerzo relativo y seis riesgos, con la diferencia de incrementos de ES1 documentada |
| Introducción y conclusiones | Redactadas a partir del cuerpo, sin anunciar implementación ni cumplimiento |
| Región Santiago decidida y aplicada | Tarifas de `INV-021` en el presupuesto, provisión del 35 % eliminada y flujo recalculado: piloto USD 307,72 con API mínima. La salida de 4.889.743 CLP y el VAN −1.970.676 de `INV-019` corresponden a la versión **histórica con LOF**; el escenario posterior de INV-029 con comisión del 12 % también es antecedente. El vigente de 3 % figura en el Anexo A |
| **Orden de anexos corregido** | Los anexos siguen ahora el orden de primera cita del cuerpo: A = evaluación económica, B = diccionario de datos, C = catálogo de casos de prueba. Se renombraron los tres archivos, se actualizó `informe.json` y se rotaron las 129 referencias en mayúscula del cuerpo, los anexos, los registros y los tableros; las referencias en minúscula a los anexos de ES1 quedaron intactas y la convención está documentada en `AGENTS.md`. El índice ensamblado se verificó y las 52 pruebas pasan |
| **Mercado Pago confirmado en lo documental** | `INV-026` reúne la evidencia oficial: reparto 1:1 disponible en Chile para Checkout Pro o API con KYC 6, OAuth por arrendador y cuentas de prueba; comisión de Mercado Pago descontada primero al vendedor; reembolso proporcional condicionado al saldo del vendedor; reporte de ventas con tarifa del marketplace, tarifa de Mercado Pago y neto. El estudio de códigos SII no equivale a inscripción. Lo confirmado no agrega ingresos; la incidencia se modela en el vendedor y requiere conciliación contractual/documental |
| Domicilio y patente contrastados documentalmente | `INV-029` registra Oficina Express: 50.000 CLP/año más 5.000 CLP por firma de contrato, IVA incluido; **una patente** para la SpA en el domicilio, mínimo ilustrativo 1 UTM/año; DOM y aseo sujetos a liquidación. El escenario histórico con 12 % arrojaba salida del año 1 de 4.815.743 CLP y VAN −1.791.437 CLP. El Anexo A vigente conserva el domicilio y recalcula 5.314.863 CLP de salida y VAN −15.140.998 CLP; sin contrato ni permiso obtenidos |
| Registros de investigación | Registros `INV-001` a `INV-033` con pregunta, hallazgo y límite, indexados en `INV-024`; INV-027 prepara consultas de terceros, INV-028 conserva sensibilidades históricas, INV-029 registra el domicilio y la patente, INV-030 audita los pendientes, INV-031 reúne fuentes para cerrar decisiones, INV-032 selecciona tecnologías y INV-033 aplica todo al cuerpo sin dar por obtenidas respuestas externas |
| Sensibilidad documental de terceros | `INV-028` conserva la base histórica LOF y simula asesoría inicial ±25 % y firma neta de 2.500 por firmante. La brecha de financiamiento del primer año con Oficina Express, API mínima y colchón del 20 % es 3.377.835 CLP, según el Anexo A. Las cotizaciones siguen abiertas |
| Metadatos y rúbrica | `INV-025`: la guía omite el criterio 2.1.5.15 y su tabla suma 96; la rúbrica suma 60 con 17 criterios |
| Código de asignatura | El usuario confirmó **TIH184** el 29-09-2026; `informe.json` lo incorpora en la portada. La grafía TIHI84 permanece únicamente en el nombre/contenido histórico de la plantilla y la guía |
| Bibliografía | Auditoría del `.bib`: 115 entradas, 90 citadas, ninguna cita sin fuente ni fuente incompleta |
| Revisión Word de la salida vigente | `INV-020` (§24-09-2026): informe de 91 páginas y anexos A/B/C generados con el perfil aprobado, índices actualizados y render revisado en `build/revision/`; se corrigió un defecto real de anchos de tabla y `validar ES2PT` termina en 0 errores. La revisión del 23-09 queda como antecedente de una salida anterior |
| Citas y referencias | Convención aplicada en Markdown: solo fuentes primarias. Se retiraron las menciones a registros internos y las autocitas de la línea base; la salida Word anterior también se revisó, pero la nueva requiere comprobación final |
| Calidad técnica de borrador | `ensamblar ES2PT` y auditoría de fuentes: 0 errores, 3 avisos por pendientes; 52 pruebas `unittest` correctas y ES1 conservado: 192 archivos, 0 alterados. `validar` de los Word actuales queda para el cierre |
| **Cierre documental con supuestos trazables** | `INV-030` auditó las 32 tareas originales y su destino; `INV-031` reunió fuentes primarias para pagos, firma, nube y trámites; `INV-032` seleccionó tecnologías con versiones, licencias y hardware supuesto; `INV-033` aplicó los tres al cuerpo: valoración ponderada por capa (SUP-04/SUP-19), fichas de versiones y licencias verificadas (SUP-05/SUP-20) y decisión de inmutabilidad de RNF-017 por retención bloqueada (SUP-13). Los marcadores bajan de 17 a 14 |
| **Escenario segmentado sintético** | `investigacion/supuestos_segmentado.json`, resumido en 2.1 y el Anexo A: mismo volumen hipotético 0/720/1.800 repartido en cinco categorías con los precios de lista de `INV-011`; la contribución del año 3 pasa de 0 a −3.617.400 CLP y el VAN de caja de −15.140.998 a −19.515.672 CLP. Es una sensibilidad declarada, no demanda medida |

Marcar `[x]` únicamente con evidencia. `validar --final` bloquea tareas abiertas en este archivo.
