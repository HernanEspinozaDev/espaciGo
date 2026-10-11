# Guía vigente de arranque local — EspaciGo

Actualizada para `main` `6164ba0a929bd088e94a8914287f3bffe7899f94` (PR #264), esquema V52. El entorno persistente conserva `espacigo_pgdata`, datos sintéticos y secretos locales. Esta guía no declara LOCAL-1 aceptado: el ZIP todavía omite contratos propios y hay una decisión de equivalencia sobre el modelo `documento` y decisiones de retención pendientes descritas en la matriz.

## Levantar y comprobar el prototipo

Desde la raíz del repositorio, con Docker Compose instalado:

```sh
bash scripts/dev-env.sh up -d
bash scripts/dev-env.sh verify-http
```

`up -d` inicia servicios y aplica solo migraciones pendientes. No ejecutar `clean`, `down --volumes`, ni eliminar volúmenes o archivos bajo `.local/secrets`. El entorno integrado registra V52; no es necesario reiniciarlo para usar la app.

Abrir:

- Mock: <http://127.0.0.1:8081>
- Mailpit para verificación/avisos sintéticos: <http://127.0.0.1:8025>

Usar cuentas sintéticas activas existentes o registrarse mediante el flujo del mock con correos de prueba `@ejemplo.invalid`; completar la verificación desde Mailpit. No guardar credenciales en el repositorio ni compartir tokens de correo.

El mock recorre publicación/búsqueda, reserva y pagos fake, contrato sintético, operación, conversación, disputas y controles administrativos autorizados. Cada resultado fake se identifica como ensayo. El PR de consolidación no modifica código ni datos.

## Pruebas y límites de aislamiento

El runner integral publicado ejecutó sobre el commit probado `056205d`: 100/100 pruebas del mock, 4/4 controles del manifiesto, `go test -p 1 ./...`, `go vet ./...`, 83/83 integraciones obligatorias PostgreSQL/PostGIS+Mailpit, cero omitidas y 5/5 repeticiones del check-in. `main` #264 tiene código ejecutable idéntico; los commits posteriores solo cambian documentación. Por eso esta actualización documental reutiliza esa evidencia y no repite la suite.

Las nuevas pruebas PostgreSQL deben utilizar el runner/base desechable del proyecto y nunca `espacigo_pgdata`. No restaurar backups sobre el volumen activo.

## Estado de cierre

- V52 está aplicada incrementalmente; PostgreSQL, Backend, Mailpit y mock quedaron saludables y `verify-http` pasó.
- Existe una brecha funcional local pendiente: el ZIP de datos propios no incluye los contratos sintéticos del titular, sus firmas/historial ni el PDF privado disponible. No se afirma que la evidencia integral cubra esta omisión.
- Está pendiente ratificar si los owners especializados satisfacen MAP-02 para `documento`. Tampoco tienen plazo local ratificado el contrato/firmas/PDF sintéticos ni `espacio_publicacion_historial_local` y `espacio_eliminacion_historial_local`; los datos se conservan y no se purgan automáticamente por analogía.
- Issues #246, #185/#40 y los padres generales siguen abiertas. No marcar LOCAL-1 como Hecho hasta cerrar la brecha, ratificar decisiones necesarias y obtener aceptación explícita.
- No hay trabajo, pruebas ni despliegue GCP autorizados por el corte actual.

Véanse [`plan_cierre_backend_local.md`](plan_cierre_backend_local.md), [`backlog_cierre_backend_local.md`](backlog_cierre_backend_local.md), [`matriz_cierre_backend_local.md`](matriz_cierre_backend_local.md), [`clasificacion_privacidad_local.md`](clasificacion_privacidad_local.md), [`evidence/local-close-01-matrix-20261011.md`](evidence/local-close-01-matrix-20261011.md) y [`evidence/local-close-01-final-20261011.md`](evidence/local-close-01-final-20261011.md).
