# Cargador del kit `espacigo-1000`

## Alcance del corte 1: cuentas y publicaciones

`cmd/dataset-loader` valida en modo de solo lectura el manifiesto, las asignaciones y referencias cruzadas, el contenido JSONL, los totales por rol/categoría/visibilidad y la distribución de escenarios. No conecta a PostgreSQL.

`scripts/dataset-import.py` implementa el corte de carga de cuentas y publicaciones mediante HTTP contra los servicios Go existentes:

- Registra las 1.000 cuentas con el mismo secreto del archivo privado 0600 de política local, nombres sintéticos y correos deterministas bajo `.invalid`. Acepta términos publicados, verifica por el correo local y actualiza el perfil por la API. La contraseña y los correos no aparecen en stdout, errores ni checkpoints.
- Conserva la asignación de arrendatario por defecto. Un administrador global operador, autenticado desde la política privada, otorga los roles de anfitrión a través de la API auditada. Las diez cuentas administradoras reciben solo el rol legado mediante el nuevo flujo explícito `admin_access.manage`, sesión reciente y auditoría. El cargador no concede ni modifica membresías `administrador_global`.
- Crea y aprueba KYC sintético a través de los servicios de verificación, nunca insertando estados. Crea 1.200 espacios, tarifas CLP `int64`, atributos de categoría, zona IANA y horario semanal para modalidad hora. El horario se guarda por la ruta administrativa auditada, con sesión reciente y control optimista de versión. La migración aditiva V61 elimina la restricción heredada que permitía activar horarios solo en el fixture singleton; habilita horarios para publicaciones por hora no eliminadas. Añade la imagen `synthetic-png-v1`; valida bytes PNG/tamaño/SHA-256 en cada proceso API, selecciona portada por `/spaces/{spaceId}/cover` y declara pública la foto solo para espacios activos. Publica, oculta o deja en borrador mediante las rutas de dominio y verifica la respuesta guardada.
- Guarda referencias externas a UUID devueltos por Go en un mapa privado bajo `.local/testing/` con modo 0600. Persiste el checkpoint de cada cuenta/publicación con reemplazo atómico. Claves estables y conciliación de espacios existentes permiten reanudar tras interrupciones. El estado privado se liga por hash al contenido generado.
- No crea tratos en este corte. No hay SQL de importación ni mutaciones del reloj de negocio.

Preflight, que no requiere credenciales:

```sh
go run ./cmd/dataset-loader --dataset-dir /ruta/privada/.local/datasets/espacigo-1000
```

El modo piloto limita la ejecución a 20 cuentas balanceadas por rol y sus publicaciones. Requiere servicios de API y Mailpit dedicados, en puertos loopback altos (`>=10000`), conectados a una restauración efímera del backup. `--mode import` recorre el dataset completo; exige confirmar el destino `espacigo_local`, un manifiesto 0600 y el dump cuyo SHA-256 coincide. La carga persistente se ejecuta solo después de fusionar este corte y comprobar el piloto. Para el modo completo se requieren tres procesos API dedicados con `LOCAL_VERIFICATION_IP_LIMIT=1000`; sus cuotas quedan aisladas del backend habitual. Todos comparten el mismo `M03_EVIDENCE_DIR` privado montado en lectura/escritura. El importador lee cada PNG por cada proceso antes de avanzar. No se reinicia ni reconfigura el stack persistente.

### Reparación de imágenes de una carga persistente

`--mode repair-images` usa el checkpoint privado para identificar las referencias por API. Comprueba y repone las galerías de las 1.200 publicaciones mediante foto sintética, mantiene privadas las fotos de ocultas/borradores y selecciona portada/publicación explícita solo para las 960 activas. Guarda la referencia de sustitución en el checkpoint antes de retirar por API el archivo roto. Confirma PNG, tamaño y SHA-256 desde API autenticada; para activas también confirma portada y GET público desde cada proceso. Requiere una API loopback persistente, `--confirm-target espacigo_local`, checkpoint 0600 y manifiesto/backup 0600. No ejecuta escrituras SQL.

Una carga hecha por procesos API con almacenes efímeros puede conservar metadata de galería en PostgreSQL y haber perdido los bytes al retirar esos procesos. El reparador reconstruye las 1.200 imágenes sintéticas mediante el servicio, sin cambiar la visibilidad de ocultas y borradores. Solo las activas reciben portada y autorización pública. Las referencias antiguas se retiran por API después de verificar la nueva imagen; el worker versionado drena la cola durable de archivos retirados cada 15 segundos y reintenta errores.

```sh
python3 scripts/dataset-import.py --mode repair-images \
  --repo /ruta/al/worktree/main \
  --dataset-dir /ruta/privada/.local/datasets/espacigo-1000 \
  --api-base http://127.0.0.1:8080 \
  --mailpit-base http://127.0.0.1:8025 \
  --policy-file /ruta/privada/.local/testing/test-account-policy.json \
  --state-file /ruta/privada/.local/testing/dataset-import-persistent.json \
  --confirm-target espacigo_local \
  --backup-manifest /ruta/privada/.local/testing/dataset-load/preload-inventory.json
```

## Corte 2: tratos

El cargador de tratos selecciona un piloto balanceado de 10 escenarios y puede reanudarse desde checkpoints privados. Genera intervalos solo a partir de disponibilidad consultada a Go, y ejecuta reserva, pago, garantía, decisión del anfitrión, contrato, firma, check-in/out, recepción, reclamo, liquidación, liberación y comprobante mediante APIs Go y proveedores fake. No inserta estados por SQL. Los pendientes de pago y reclamos abiertos se crean al final para conservar sus estados/plazos. Requiere cuentas/publicaciones previamente cargadas y KYC sintético aprobado para anfitriones y arrendatarios.

El usuario amplió explícitamente el alcance el 2026-10-15: piloto y después importación persistente de los 1.200 escenarios; también autorizó ampliar este plan para usar un reloj aislado con la base persistente. La importación completa solo se inicia después de fusionar el corte y validar el piloto. El proceso API dedicado montará un archivo de reloj exclusivo de esa ejecución en modo de solo lectura; el helper del proceso lo avanza mediante checkpoints y comprueba que API y archivo coincidan. Nunca se cambia el reloj de máquina ni el reloj compartido del backend habitual, y el backend habitual no se reinicia ni se reconfigura. La API dedicada debe desactivar sus sweeps/reconciliadores globales de reservas, garantías, pagos y contratos (incluido EnsureApproved); las rutas de la carga no deben disparar barridos globales por peticiones. Esta protección debe estar fusionada y probada contra regresiones antes de arrancar el proceso dedicado. Las operaciones autorizadas escribirán timestamps de negocio según el avance lógico, que no se presentará como hora real. Antes de cargar se verificará un backup; al terminar se comprobarán conteos, estados, idempotencia y el baseline de filas preexistentes por API/consultas de auditoría. Se detendrá y retirará el proceso/reloj al concluir.

### Piloto aislado de tratos (2026-10-15)

- Compose exclusivo `espacigo_deals_pilot_2b7f622eb59e`, API loopback `21980`, volumen propio `espacigo_deals_pilot_2b7f622eb59e_pgdata`; restaurado desde backup privado verificado. No se modificó la base persistente.
- Se ejecutaron dos selecciones de 10 escenarios en el mismo Compose aislado (20 reservas sintéticas totales, sin duplicar referencias): ambas incluyeron dos casos por cada desenlace. La selección inicial usó 10 escenarios hora; la selección balanceada cubrió 4 hora, 3 día y 3 mes, además de las ocho categorías. Cada checkpoint confirmó `cerrada=2`, `pendiente_de_pago=2`, `cancelada_arrendatario=2`, `rechazada_arrendador=2`, `en_disputa=2`.
- El recorrido usó API Go y proveedores fake. En ambas selecciones se verificaron historia, liquidación, liberación de garantía y comprobante para los dos cierres. Una repetición desde checkpoint procesó los mismos 10 escenarios sin duplicar estados ni volver a firmar contratos ya firmados. El reloj de negocio fue el archivo exclusivo del run `2b7f622eb59e`, modo 0600, y la API confirmó igualdad con él; hora final lógica `2026-10-23T03:00:00Z`.
- Estado y cuentas sintéticas permanecieron privados, permisos 0600. Al registrar esta evidencia se detuvieron y eliminaron solo los recursos y credenciales del ensayo.

### Regresión del reloj aislado y piloto con el guard actual (2026-10-15)

- La integración `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry` ahora crea en PostgreSQL real una reserva pendiente de pago, un contrato con firma parcial y una autorización de garantía pendiente. Avanza el reloj inyectado 48 horas, hace GET/Quote/Create/Pay/Guarantee por HTTP y prueba los tres workers del servicio con contexto cancelado. Los snapshots JSONB completos de cotizaciones, reservas, ocupación, pagos, garantías, contratos, firmas, eventos e historiales quedan idénticos. Luego comprueba que el modo normal sí vence la reserva y concilia la autorización. El API dedicado no inicia los workers globales de contratos, pagos, garantías ni limpieza de operaciones; el modo exige flag de ensayo y el reloj run-scoped montado solo lectura.
- Se ejecutó el test de booking en PostgreSQL 18/PostGIS 3.6 desechable y pasó; la suite `scripts/test-local-qa-backend.sh` pasó con 151 eventos Go, 94/94 integraciones obligatorias, cero skips, `go vet` y cinco repeticiones del test de booking.
- Piloto desde el código del corte con reloj aislado, en Compose/volumen/red/API/Mailpit exclusivos y restaurados desde el dump privado preimport: 20 cuentas sintéticas, 60 espacios y 10 tratos. Los tratos cubrieron los cinco desenlaces con dos casos cada uno, las ocho categorías, 4 modalidades hora, 3 día y 3 mes. Las diez fichas públicas devolvieron HTTP 200. Los dos cierres verificaron firma, check-in/out, recepción, historial, liquidación, liberación de garantía y comprobante.
- Al reejecutar desde el checkpoint, el cargador procesó los mismos diez escenarios; quedaron diez reservas distintas y no se creó ninguna duplicada. Conteos del Compose: 65 usuarios, 101 espacios y 38 reservas (28 previas + 10 sintéticas).
- Se restauró el dump preimport a una segunda base de referencia del mismo clúster efímero y se compararon los 28 IDs de reserva originales con snapshots byte a byte en 12 tablas de reservas, contratos, garantías e historiales. Conteos relacionados: 79 transiciones, 28 ocupaciones, 6 contratos, 12 firmas, 18 eventos de contrato, 1 garantía previa y 3 operaciones previas de garantía; SHA-256 del snapshot `0203304f7a5ee39b0cda1e19ad93fa338580dbb368ec06f8f8d07a2456911c1b`, idéntico antes/después del piloto.
- Pruebas Python finales: 8/8 entre los seis tests de `test_dataset_import.py` y los dos de `test_dataset_deals.py`, más `py_compile`. Compose, sus dos bases, volumen, redes, imágenes, credenciales y checkpoints se retiraron tras conservar evidencia agregada privada modo 0600. No se tocó el volumen persistente.

## Límites y evidencia

El contenido cargado es sintético y no valida identidades reales. Las fotos son el PNG fijo local; no hay archivos aportados por usuarios. Proveedores, firmas y pagos siguen siendo fake/locales. JSONL, cuentas, mapa de UUID y checkpoints se quedan privados y excluidos de Git.

### Evidencia registrada

- Preflight de solo lectura: pasó con 1.000 cuentas, 1.200 publicaciones, ocho categorías y cinco grupos de escenarios; no escribió en PostgreSQL.
- Se restauró el backup previo a una base y volumen exclusivos del piloto. La base de entrada tenía 45 cuentas, 41 espacios, 28 reservas y 60 migraciones. Se aplicó V61 sin reescribir V1–V60. En una segunda base vacía del mismo clúster efímero se ejecutaron las 61 migraciones desde cero.
- Piloto real por API en PostgreSQL 18/PostGIS 3.6/Mailpit: 20 cuentas (12 arrendatarios, 6 anfitriones y 2 administradores legados) y 22 publicaciones; 6 quedaron activas, 8 ocultas y 8 en borrador. Las 22 recibieron foto sintética, las 12 publicaciones por hora recibieron horario activo y las 6 públicas quedaron con la foto visible. Conteos después del piloto: 65 cuentas, 63 espacios, las mismas 28 reservas, 1 administrador global total, 0 administradores globales entre las 20 cuentas importadas, 2 roles legados importados y 12 horarios activos.
- Se repitió la ejecución desde el checkpoint privado 0600. Conservó 20 asignaciones de cuenta y 22 de publicación, sin duplicar filas; el UUID y rol del administrador global original no cambiaron. Las interrupciones durante la primera ejecución se reanudaron con el mismo checkpoint.
- `go test ./internal/adminglobal/... -count=1` pasó contra el servicio de PostgreSQL aislado, incluyendo grant/replay/revoke auditado del rol legado. El test creó y retiró sus bases temporales dentro del piloto.
- Evidencia histórica anterior a la fusión #319: la API real se construyó desde el worktree del corte; el backend persistente y su reloj no se modificaron y todavía no se había cargado el conjunto 1.000/1.200. El estado persistente posterior se registra en la siguiente sección.

### Reparación de portadas, regresión y piloto (2026-10-15)

- Verificación de cierre de la carga persistente en `5f4c51c`: 1.000 cuentas activas, 1.200 publicaciones, 960 activas (120 por cada una de las ocho categorías), 120 ocultas y 120 en borrador. La consulta pública contiene las 960 activas y no las otras 240. El baseline persistente se conserva (45 cuentas y 41 espacios preexistentes, 28 reservas); el maestro global/bootstrap continúa activo. V1–V60 mantiene exactamente los hashes de entrada; V61 está aplicada.
- `scripts/test_dataset_import.py`: 6/6. Incluye reemplazo idempotente de un archivo sintético ausente, verificación de la portada pública, modo privado para ocultas/borradores y restricción del puerto compartido al modo explícito de reparación.
- `go test ./internal/adapters/postgres/spaces -run '^TestLocalPublicationRequiresEffectiveKYCAndRecordsOwnerTransitions$' -count=1`: pasó en PostgreSQL 18/PostGIS 3.6 desechable. Añade regresión de portada escogida y la inclusión de `images[{image_id,is_cover}]` en el DTO de listado. Se retiró la base de test.
- Piloto API recreable en Compose exclusivo `espacigo-dataset-cover-pilot-20261015`, restaurado desde el backup previo; V61 se aplicó por el migrador. 20 cuentas y 22 publicaciones: 6 activas, 8 ocultas, 8 en borrador. El almacenamiento privado fue una carpeta host exclusiva compartida por el API y persistió cuando el contenedor API se eliminó y recreó. Tras la recreación, 22/22 galerías respondieron PNG autenticado con tamaño y SHA-256 correctos; 6/6 portadas activas aparecieron en `cover_image_id` e `images` y sus GET públicos devolvieron PNG 200 con SHA-256 coincidente. Ninguna de las 8 ocultas ni 8 borradores apareció en el catálogo. Las sesiones sintéticas quedaron cerradas.
- Lectura final del piloto: 65 cuentas, 63 espacios, las mismas 28 reservas, 1 administrador global, 5 roles administradores legados y 61 migraciones. Se eliminó el proyecto, su volumen y contenedores al registrar la evidencia; no se guardó una demo.
- Limitación: las imágenes siguen siendo el fixture PNG determinista local; no hay fotos de usuario ni proveedor externo. El reparador persistente de 1.200 galerías aún no se ejecuta: requiere revisar/fusionar este PR y luego reconstruir las referencias por API.

Los correos y contraseñas no aparecen en estos resultados. El mapa UUID, las credenciales y los logs detallados permanecen en `.local/testing/` con permisos privados. Las reservas y los cinco grupos de escenarios siguen para el corte 2.
