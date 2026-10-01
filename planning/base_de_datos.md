# Plan de Base de Datos

## Línea base

El contrato académico vigente es [ES2 Anexo B](../../Informes/ES2PT/anexos/B_diccionario_datos.md): 43 tablas de diseño, producto completo, PostgreSQL 18 + PostGIS + `btree_gist` como objetivo. El diccionario no es DDL ejecutado y el SQL ilustrativo antiguo no debe tratarse como migración vigente. El modelo físico se implementará módulo a módulo, sin crear desde el inicio todas las tablas por anticipado.

## Perfil de persistencia propuesto (CORE-DB-01)

Estas convenciones versionan una propuesta técnica para revisión; no crean esquema físico, no autorizan DDL y no seleccionan una plataforma productiva.

- **Motor/extensiones:** PostgreSQL 18; PostGIS 3.6 y `btree_gist` 1.8 conforme al objetivo del Anexo B. La evidencia disponible cubre únicamente las versiones locales y la imagen descritas abajo; no se afirma compatibilidad con otros proveedores/patches no probados.
- **Imagen de prueba fijada:** `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`; un tag sin digest no es suficiente para reproducir la prueba. No es una decisión de imagen productiva.
- **Evidencia ambiental preexistente (2026-10-01; no repetida en esta tarjeta):** instalación local PostgreSQL 18.6, PostGIS 3.6.2 y `btree_gist` 1.8; la imagen fijada informó PostgreSQL 18.6, PostGIS 3.6.4 y `btree_gist` 1.8. Las consultas de versiones/extensiones, healthcheck y creación de `btree_gist` en instancia descartable pasaron. La diferencia observada de patch de PostGIS queda registrada; no se extrapola a otros patches ni a Cloud SQL. Evidencia detallada de instancia y pruebas en [migraciones_postgresql.md](migraciones_postgresql.md).
- **Smoke reproducible local/CI:** usar la imagen fijada con `pg_isready`; consultar `SHOW server_version;`, `SELECT postgis_full_version();` y `SELECT extname, extversion FROM pg_extension WHERE extname IN ('postgis','btree_gist') ORDER BY extname;`; en una base descartable ejecutar `CREATE EXTENSION IF NOT EXISTS postgis;` y `CREATE EXTENSION IF NOT EXISTS btree_gist;`. Aceptar solo si PostgreSQL 18.x, PostGIS 3.6.x y `btree_gist` 1.8.x están disponibles. El registro preexistente satisface el smoke para las versiones indicadas; automatizarlo en CI cuando se cree la infraestructura de pruebas.
- **Nombres/constraints:** tablas en singular, identificadores y columnas en `snake_case`, descriptivos y consistentes con el diccionario español. Nombrar constraints e índices de forma estable como `<tabla>_<columnas>_<tipo>`; declarar nombres explícitos para FKs y restricciones.
- **IDs:** usar tipo `uuid` según Anexo B. La política concreta de generación/biblioteca queda por seleccionar antes de la primera migración; UUID no es autorización ni anonimización.
- **Tiempo:** `timestamptz` para instantes UTC; `date` para fechas civiles sin hora. No usar timestamp sin zona para eventos.
- **Rangos:** ocupación con `tstzrange` finito, no vacío y semiabierto `[inicio, fin)`; GiST/`btree_gist` para exclusión de solapes. Otros rangos temporales seguirán `[)` cuando la semántica represente intervalos.
- **Moneda:** alinear con Anexo B: CLP en `numeric(14,0)` (pesos enteros); moneda explícita `char(3)` ISO 4217; porcentajes y cálculos intermedios con precisión/escala declaradas y versionadas. Nunca `float`/`double precision`; no fijar reglas legales de redondeo distintas del contrato sin decisión trazable.
- **Esquema/roles:** propuesta inicial de un único esquema `public`; cualquier partición por módulo requiere decisión previa documentada. Separar rol propietario/migrador del rol runtime de privilegio mínimo; la app no recibe privilegio de crear objetos; credenciales fuera del repositorio.
- **Índices:** añadirlos con una consulta/invariante justificada: B-tree para FK/filtros/orden medidos y GiST para geografía/rangos/exclusiones. No duplicar índices cubiertos por constraints ni crear índices parciales sin predicado trazable. Medir con `EXPLAIN (ANALYZE, BUFFERS)` y datos sintéticos antes de optimizar.
- **Evolución:** DDL solo por migraciones versionadas con rol migrador; cambios compatibles expand/contract y sin edición manual destructiva. Esta tarjeta no crea tablas, migraciones, roles ni archivos de entorno.

## Perfil PostgreSQL verificado para CORE-DB-03

Imagen fijada por digest: `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`. Verificada el 2026-10-01 en una instancia desechable: PostgreSQL `18.6 (Debian 18.6-1.pgdg13+2)`, PostGIS `3.6.4` y `btree_gist` `1.8`. Evidencia de integración y limpieza en [migraciones_postgresql.md](migraciones_postgresql.md). Este perfil es para pruebas; no configura despliegue productivo.

## Modelo conceptual inicial

| Área | Entidades principales | Invariante relevante |
| --- | --- | --- |
| Identidad | usuario, rol, perfil, sesión, token y aceptación de términos | Roles coexistentes; credenciales/token nunca se exponen; revocación auditable. |
| Privacidad/verificación | solicitud de titular, verificación, referencias de cuenta de cobro/proveedor | Finalidad, acceso y retención por clase de dato; no almacenar secretos de terceros. |
| Catálogo | categoría, espacio, tarifas y políticas versionadas | Todas las categorías desde el diseño; cada espacio es una unidad física reservable exclusiva. |
| Cotización/reserva | cotización, reserva, ocupación, transiciones | Instantánea de precio/condiciones; una sola ocupación activa por unidad e intervalo. |
| Finanzas | pago, evento de proveedor, movimiento, garantía, liquidación, documento tributario | Intento remoto separado del hecho confirmado; idempotencia y conciliación; no presumir custodia. |
| Contrato/operación | contrato, firmas, operación de arriendo, disputa, documentos | Evidencia vinculada al recurso/propietario; historial no se borra en cascada. |
| Comunicación | mensajes, reseñas, reportes, notificaciones/entregas | Acceso por participantes, reserva y estado; contenido con retención/finalidad. |
| Gobierno/analítica | auditoría, outbox, promociones/derechos de reportes, NPS | Outbox en la misma transacción local; analítica minimizada fuera del OLTP. |

El Anexo B tiene exactamente 43 tablas, aunque agrupa varias bajo ciertos encabezados: `usuario`, `rol_usuario`, `perfil_usuario`, `sesion`, `token_accion`, `version_terminos`, `aceptacion_terminos`, `verificacion`, `cuenta_cobro`, `vinculo_proveedor_vendedor`, `categoria_espacio`, `espacio`, `politica_cancelacion`, `tramo_cancelacion`, `regla_tarifa`, `regla_comision`, `cotizacion`, `reserva`, `ocupacion`, `reserva_transicion`, `pago`, `evento_proveedor`, `movimiento_financiero`, `garantia`, `liquidacion`, `documento_tributario`, `contrato`, `firma_contrato`, `operacion_arriendo`, `disputa`, `documento`, `mensaje_reserva`, `resena`, `reporte_resena`, `notificacion`, `entrega_notificacion`, `evento_auditoria`, `solicitud_titular`, `outbox_evento`, `campana`, `orden_promocion`, `derecho_reporte`, `respuesta_nps`.

## Secuencia física

El contrato operativo de naming, checksum, serialización, transacciones, detección de deriva y reconstrucción vacía está en [migraciones_postgresql.md](migraciones_postgresql.md). El runner ejecutable (`cmd/dbmigrate`) y sus seis pruebas de integración pasaron contra la imagen PostgreSQL/PostGIS fijada por digest; PR #3 fue aprobado y fusionado el 2026-10-01. CORE-DB-03 queda `done`. AUTH-DB-02 no se inició y sigue pendiente de sus dependencias; el merge del runner no sustituye CORE-DB-02 ni sus revisiones.

1. **Revisión de modelo global:** cardinalidades, dueños lógicos, clasificaciones personales/restringidas, plazos por finalidad, estados y claves. Registrar cambios antes de escribir DDL.
2. **Convenciones de persistencia:** PG18/extensiones requeridas, UUID/timestamps/moneda, naming, esquema, roles de migración/API/operación, migraciones versionadas, rollback/forward-fix, test fixtures y política de cambios compatibles.
3. **Fundación mínima:** base local reproducible, extensiones autorizadas, cuentas/roles mínimos, migrador y health/readiness. No cargar datos reales; seeds solo para catálogos públicos controlados.
4. **Migración por módulo:** modelo específico → DDL y restricciones → pruebas de migración vacía/repetida → revisión → aplicar en entorno de desarrollo. Cada cambio posterior va en migración nueva.
5. **Índices guiados por consultas:** definirlos junto a endpoints/planes de consulta; incluir geoespacial, unicidad e índices parciales solo con consulta y cardinalidad documentadas. Medir antes de añadir índices costosos.
6. **Verificación de integridad:** FK con borrado restrictivo en hechos históricos, `CHECK` de dominios, exclusiones y pruebas de concurrencia/SQLSTATE; las invariantes de workflow también se verifican en servicios.
7. **Datos de prueba y protección:** fixtures sintéticos deterministas; sin PII de producción en desarrollo/mock. Probar backup/restore, solicitudes de derechos y retención en una fase de entorno.

## Diseño de calendario y transacciones

`ocupacion` unifica retenciones de reserva y bloqueos manuales; rango finito no vacío semiabierto `[inicio, fin)`. La propuesta `EXCLUDE USING gist (espacio_id WITH =, intervalo WITH &&) WHERE (activo)` previene solapes entre solicitudes concurrentes. La FK compuesta debe asegurar que reserva y espacio coincidan. La expiración libera ocupación en la misma transacción que cambia el estado de reserva. Añadir esta restricción antes de habilitar la reserva; probar solape, adyacencia, expiración y carreras.

La reserva conserva snapshots de tarifa/comisión/impuestos/condiciones; cotización no retiene inventario. Importes CLP se modelan en entero/decimal exacto, nunca `float`; tarifa externa, comisión, IVA, total de comprador y neto observado siguen separados.

## Seguridad y privacidad desde la primera migración

- La Ley 21.719 es criterio de diseño del primer incremento, por decisión del usuario; no equivale a cumplimiento probado ni a vigencia legal adelantada.
- Asignar finalidad, acceso, mínimo dato, encargado/ubicación, retención y acción de cierre por clase y tratamiento.
- UUID sigue siendo identificador vinculable; minimizar registros y payloads. No persistir URLs firmadas, credenciales ni secretos de proveedor.
- Usar autorización por recurso en backend; roles PostgreSQL no reemplazan autorización de negocio.
- No usar cascadas que destruyan pagos, contratos, evidencias, auditoría o solicitudes de derechos. Resolver baja/desidentificación mediante flujo explícito validado.
- No fijar bloqueos irreversibles ni plazos productivos de retención hasta resolver política y aprobación correspondiente.

## Qué se aplaza

No implementar todavía la base de datos física completa ni crear tablas por anticipado. Promociones, derechos de reportes, NPS y analítica se diseñan para el producto completo, pero su creación se agenda después del flujo transaccional, cuando reglas comerciales y finalidades estén resueltas. No transportar actividad de alto volumen a una fila OLTP por evento.
