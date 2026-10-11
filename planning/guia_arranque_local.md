# Guía vigente de arranque local — EspaciGo

Actualizada para la entrega LOCAL-CLOSE-01E basada en `main` #265 (`6358b26cc891977b8503f96bece033d34ebc631b`) y commit candidato `db01c00`. V53 añade exportación/retención de contratos e historiales de publicación; fue validada desde V52 solo en PostgreSQL desechable. El entorno persistente conserva `espacigo_pgdata`, datos sintéticos y secretos locales; no se aplicó V53. LOCAL-1 se presenta para aceptación explícita y #246 permanece abierta.

## Levantar y comprobar el prototipo

Desde la raíz del repositorio, con Docker Compose instalado:

```sh
bash scripts/dev-env.sh up -d
bash scripts/dev-env.sh verify-http
```

`up -d` inicia servicios y aplica solo migraciones pendientes. Al integrar este corte, V53 se aplicará incrementalmente en el entorno local; nunca reiniciar desde cero. No ejecutar `clean`, `down --volumes`, ni eliminar volúmenes o archivos bajo `.local/secrets`. La validación de este PR usó exclusivamente bases desechables.

Abrir:

- Mock: <http://127.0.0.1:8081>
- Mailpit para verificación/avisos sintéticos: <http://127.0.0.1:8025>

Usar cuentas sintéticas activas existentes o registrarse mediante el flujo del mock con correos de prueba `@ejemplo.invalid`; completar la verificación desde Mailpit. No guardar credenciales en el repositorio ni compartir tokens de correo.

El mock recorre publicación/búsqueda, reserva y pagos fake, contrato sintético, operación, conversación, disputas y controles administrativos autorizados. Cada resultado fake se identifica como ensayo. LOCAL-CLOSE-01E amplía el ZIP propio con contrato minimizado y PDF privado disponible; su V53 tiene pruebas en base desechable.

## Pruebas y límites de aislamiento

El runner integral ejecutó sobre `db01c00`: 100/100 pruebas del mock, 4/4 controles del manifiesto, `go test -p 1 ./...`, `go vet ./...`, 88/88 integraciones obligatorias PostgreSQL/PostGIS+Mailpit (126 eventos), cero omitidas y 5/5 repeticiones del check-in. La V52→V53 y pruebas nuevas de exportación/retención pasaron también en PostgreSQL desechable.

Las nuevas pruebas PostgreSQL deben utilizar el runner/base desechable del proyecto y nunca `espacigo_pgdata`. No restaurar backups sobre el volumen activo.

## Estado de cierre

- En la base de trabajo previa V52 estaba aplicada. V53 se probó de forma incremental en PostgreSQL desechable; no se tocó el volumen persistente.
- El ZIP exporta contrato propio para ambos participantes autorizados: snapshot y firmas/historial minimizados, PDF privado cuando existe; si fue retirado, el manifiesto lo dice y no lo reconstruye.
- Owner-stores especializados se ratifican como adaptación local de MAP-02/DB02-02. Contrato/firmas/PDF: 24 meses calendario desde el último cierre terminal relacionado; historiales de publicación/eliminación: 24 meses desde eliminación definitiva. Obligaciones abiertas difieren la purga; consultas, reintentos y exportaciones no reinician el plazo.
- La matriz no identifica brechas locales obligatorias para LOCAL-1. #246 permanece abierta hasta que el propietario acepte explícitamente; #185/#40 y padres generales conservan criterios abiertos. No se afirma supresión integral ni anonimización.
- No hay trabajo, pruebas ni despliegue GCP autorizados por el corte actual.

Véanse [`plan_cierre_backend_local.md`](plan_cierre_backend_local.md), [`backlog_cierre_backend_local.md`](backlog_cierre_backend_local.md), [`matriz_cierre_backend_local.md`](matriz_cierre_backend_local.md), [`trazabilidad_requisitos_backend_local.md`](trazabilidad_requisitos_backend_local.md), [`clasificacion_privacidad_local.md`](clasificacion_privacidad_local.md) y [`evidence/local-close-01-contract-export-20261011.md`](evidence/local-close-01-contract-export-20261011.md).
