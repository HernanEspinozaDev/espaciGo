Actualización 2026-10-11 — LOCAL-1 aceptado por el propietario dentro del alcance sintético ratificado; #246 quedó registrado como Hecho en Projects. Los padres con criterios productivos o expresamente diferidos permanecen abiertos. Se inicia LOCAL-DOCKER; sus comandos aislados están en [`local_docker.md`](local_docker.md). DOCKER-01 solo renderiza configuraciones y compila imágenes, sin levantar servicios ni tocar el volumen de desarrollo.

# Guía vigente de arranque local — EspaciGo

Actualizada después del merge de PR #266: `main` está en `0a47cafc24f0570b53e19c7d81d7ed8a806b50a1`; su código ejecutable coincide con el commit validado `03e9c77`. V53 añade exportación/retención y V54 completa la retención para cancelación por firma. Ambas están aplicadas incrementalmente al entorno persistente. La evidencia post-merge está en [`evidence/local-close-01-post-merge-20261011.md`](evidence/local-close-01-post-merge-20261011.md). Se preservaron `espacigo_pgdata`, datos sintéticos y secretos; no se adelantaron vencimientos ni se ejecutaron purgas.

## Levantar y comprobar el prototipo

Desde la raíz del repositorio, con Docker Compose instalado:

```sh
bash scripts/dev-env.sh up -d
bash scripts/dev-env.sh verify-http
```

`up -d` inicia servicios y aplica solo migraciones pendientes. V53/V54 ya están aplicadas en el entorno local actual; futuras migraciones se aplicarán incrementalmente, nunca reiniciando desde cero. No ejecutar `clean`, `down --volumes`, ni eliminar volúmenes o archivos bajo `.local/secrets`. Las integraciones PostgreSQL siguen usando bases desechables.

Abrir:

- Mock: <http://127.0.0.1:8081>
- Mailpit para verificación/avisos sintéticos: <http://127.0.0.1:8025>

Usar cuentas sintéticas activas existentes o registrarse mediante el flujo del mock con correos de prueba `@ejemplo.invalid`; completar la verificación desde Mailpit. No guardar credenciales en el repositorio ni compartir tokens de correo.

El mock recorre publicación/búsqueda, reserva y pagos fake, contrato sintético, operación, conversación, disputas y controles administrativos autorizados. Cada resultado fake se identifica como ensayo. LOCAL-CLOSE-01E amplía el ZIP propio con contrato minimizado y PDF privado disponible; sus migraciones V53/V54 también cubren retención por firma vencida.

## Pruebas y límites de aislamiento

El runner integral ejecutó sobre `03e9c77`: 100/100 pruebas del mock, 4/4 controles del manifiesto, `go test -p 1 ./...`, `go vet ./...`, 89/89 integraciones obligatorias PostgreSQL/PostGIS+Mailpit (127 eventos), cero omitidas y 5/5 repeticiones del check-in. La V52→V54 y pruebas nuevas de exportación/retención pasaron también en PostgreSQL desechable. Después del merge se reutilizó esa regresión por equivalencia de código y se ejecutó la prueba bilateral de exportación en PostgreSQL desechable.

Las nuevas pruebas PostgreSQL deben utilizar el runner/base desechable del proyecto y nunca `espacigo_pgdata`. No restaurar backups sobre el volumen activo.

## Estado de cierre

- La base de trabajo está migrada hasta V54. La prueba posterior al merge confirmó sus filas/checksums y permisos efectivos del rol runtime.
- El ZIP exporta contrato propio para ambos participantes autorizados: snapshot y firmas/historial minimizados, PDF privado cuando existe; si fue retirado, el manifiesto lo dice y no lo reconstruye.
- Owner-stores especializados se ratifican como adaptación local de MAP-02/DB02-02. Contrato/firmas/PDF: 24 meses calendario desde el último cierre terminal relacionado; historiales de publicación/eliminación: 24 meses desde eliminación definitiva. Obligaciones abiertas difieren la purga; consultas, reintentos y exportaciones no reinician el plazo.
- La matriz no identifica brechas locales obligatorias para LOCAL-1. #246 fue aceptada y cerrada por el propietario; #185/#40 y padres generales conservan criterios abiertos. No se afirma supresión integral ni anonimización.
- No hay trabajo, pruebas ni despliegue GCP autorizados por el corte actual.

Véanse [`plan_cierre_backend_local.md`](plan_cierre_backend_local.md), [`backlog_cierre_backend_local.md`](backlog_cierre_backend_local.md), [`matriz_cierre_backend_local.md`](matriz_cierre_backend_local.md), [`trazabilidad_requisitos_backend_local.md`](trazabilidad_requisitos_backend_local.md), [`clasificacion_privacidad_local.md`](clasificacion_privacidad_local.md) y [`evidence/local-close-01-contract-export-20261011.md`](evidence/local-close-01-contract-export-20261011.md).
