# Cargador del kit `espacigo-1000`

## Alcance del corte 1: cuentas y publicaciones

`cmd/dataset-loader` valida en modo de solo lectura el manifiesto, las asignaciones y referencias cruzadas, el contenido JSONL, los totales por rol/categoría/visibilidad y la distribución de escenarios. No conecta a PostgreSQL.

`scripts/dataset-import.py` implementa el corte de carga de cuentas y publicaciones mediante HTTP contra los servicios Go existentes:

- Registra las 1.000 cuentas con el mismo secreto del archivo privado 0600 de política local, nombres sintéticos y correos deterministas bajo `.invalid`. Acepta términos publicados, verifica por el correo local y actualiza el perfil por la API. La contraseña y los correos no aparecen en stdout, errores ni checkpoints.
- Conserva la asignación de arrendatario por defecto. Un administrador global operador, autenticado desde la política privada, otorga los roles de anfitrión a través de la API auditada. Las diez cuentas administradoras reciben solo el rol legado mediante el nuevo flujo explícito `admin_access.manage`, sesión reciente y auditoría. El cargador no concede ni modifica membresías `administrador_global`.
- Crea y aprueba KYC sintético a través de los servicios de verificación, nunca insertando estados. Crea 1.200 espacios, tarifas CLP `int64`, atributos de categoría, zona IANA y horario semanal para modalidad hora. El horario se guarda por la ruta administrativa auditada, con sesión reciente y control optimista de versión. La migración aditiva V61 elimina la restricción heredada que permitía activar horarios solo en el fixture singleton; habilita horarios para publicaciones por hora no eliminadas. Añade la imagen `synthetic-png-v1`; solo declara pública su foto para espacios efectivamente activos. Publica, oculta o deja en borrador mediante las rutas de dominio y verifica la respuesta guardada.
- Guarda referencias externas a UUID devueltos por Go en un mapa privado bajo `.local/testing/` con modo 0600. Persiste el checkpoint de cada cuenta/publicación con reemplazo atómico. Claves estables y conciliación de espacios existentes permiten reanudar tras interrupciones. El estado privado se liga por hash al contenido generado.
- No crea tratos en este corte. No hay SQL de importación ni mutaciones del reloj de negocio.

Preflight, que no requiere credenciales:

```sh
go run ./cmd/dataset-loader --dataset-dir /ruta/privada/.local/datasets/espacigo-1000
```

El modo piloto limita la ejecución a 20 cuentas balanceadas por rol y sus publicaciones. Requiere servicios de API y Mailpit dedicados, en puertos loopback altos (`>=10000`), conectados a una restauración efímera del backup. `--mode import` recorre el dataset completo; exige confirmar el destino `espacigo_local`, un manifiesto 0600 y el dump cuyo SHA-256 coincide. La carga persistente se ejecuta solo después de fusionar este corte y comprobar el piloto. Para el modo completo se requieren tres procesos API dedicados con `LOCAL_VERIFICATION_IP_LIMIT=1000`; sus cuotas quedan aisladas del backend habitual. No se reinicia ni reconfigura el stack persistente.

## Corte 2: tratos

Pendiente para el corte siguiente: generar calendarios de intervalos compatibles por zona, unidades y modalidad; ejecutar los 1.200 escenarios con APIs Go y proveedores fake vigentes; manejar vencimientos por checkpoints; verificar historias/ledger/contratos únicamente como resultado del servicio. Los pendientes de pago y reclamos abiertos se crean al final cuando corresponda. El reloj lógico se limitará a un proceso/base efímeros del piloto; no habrá avance del reloj compartido.

## Límites y evidencia

El contenido cargado es sintético y no valida identidades reales. Las fotos son el PNG fijo local; no hay archivos aportados por usuarios. Proveedores, firmas y pagos siguen siendo fake/locales. JSONL, cuentas, mapa de UUID y checkpoints se quedan privados y excluidos de Git.

### Evidencia registrada

- Preflight de solo lectura: pasó con 1.000 cuentas, 1.200 publicaciones, ocho categorías y cinco grupos de escenarios; no escribió en PostgreSQL.
- Se restauró el backup previo a una base y volumen exclusivos del piloto. La base de entrada tenía 45 cuentas, 41 espacios, 28 reservas y 60 migraciones. Se aplicó V61 sin reescribir V1–V60. En una segunda base vacía del mismo clúster efímero se ejecutaron las 61 migraciones desde cero.
- Piloto real por API en PostgreSQL 18/PostGIS 3.6/Mailpit: 20 cuentas (12 arrendatarios, 6 anfitriones y 2 administradores legados) y 22 publicaciones; 6 quedaron activas, 8 ocultas y 8 en borrador. Las 22 recibieron foto sintética, las 12 publicaciones por hora recibieron horario activo y las 6 públicas quedaron con la foto visible. Conteos después del piloto: 65 cuentas, 63 espacios, las mismas 28 reservas, 1 administrador global total, 0 administradores globales entre las 20 cuentas importadas, 2 roles legados importados y 12 horarios activos.
- Se repitió la ejecución desde el checkpoint privado 0600. Conservó 20 asignaciones de cuenta y 22 de publicación, sin duplicar filas; el UUID y rol del administrador global original no cambiaron. Las interrupciones durante la primera ejecución se reanudaron con el mismo checkpoint.
- `go test ./internal/adminglobal/... -count=1` pasó contra el servicio de PostgreSQL aislado, incluyendo grant/replay/revoke auditado del rol legado. El test creó y retiró sus bases temporales dentro del piloto.
- La API real se construyó desde el worktree del corte. El backend persistente y su reloj no se modificaron. El trabajo se probó antes de crear el PR; todavía no se cargó el conjunto persistente de 1.000/1.200.

Los correos y contraseñas no aparecen en estos resultados. El mapa UUID, las credenciales y los logs detallados permanecen en `.local/testing/` con permisos privados. Las reservas y los cinco grupos de escenarios siguen para el corte 2.
