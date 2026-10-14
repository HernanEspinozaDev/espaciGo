# Guía de revisión técnica MPV2-13

**Estado:** demo preparada para revisión; aceptación de producto pendiente.
**Código de aplicación:** merge de MPV2-12 `c797f0eb734e2c7eda37eb70a50d4d509e82677d` (PR #306). MPV2-13 consolida pruebas y documentación; no agrega funcionalidad de marketplace.
**Responsable del ensayo:** HernanMEC. **Motivo:** demo aislada solicitada para revisión del catálogo público.

## Abrir la demo

El navegador local puede abrir `http://127.0.0.1:19902/explorar`. La API está publicada solo en loopback en `127.0.0.1:19900`; Mailpit usa `127.0.0.1:19901`. PostgreSQL no publica un puerto al host. El proyecto Compose es `espacigo_mpv2_13_demo_316ee26df160` y su único volumen de base de datos es `espacigo_mpv2_13_demo_316ee26df160_pgdata`.

La demo tiene más de 24 publicaciones ficticias en las ocho categorías, tarifas CLP por hora/día/mes y valores de filtros tipados. Se puede explorar sin iniciar sesión: cambiar categoría, aplicar filtros, ordenar, cambiar cuadrícula y tamaño de página, abrir fichas y consultar las fotos sintéticas. Para revisar el límite de acceso, usa **Reservar** en una ficha; el acceso privado debe requerir autenticación. La reserva es simulada y no procesa pagos.

Las cuatro cuentas sintéticas están en `.local/testing/mpv2-13-demo/316ee26df160/accounts.json`, con permisos `0600`. Consulta ese archivo localmente para las credenciales; no copies su contenido en capturas, logs ni archivos compartidos. El archivo de configuración operativo es `.local/testing/mpv2-13-demo/316ee26df160/runtime.env`, también `0600`.

## Estado y ciclo de vida

Para consultar los contenedores de esta demo, desde la raíz del repositorio:

```sh
set -a
. .local/testing/mpv2-13-demo/316ee26df160/runtime.env
set +a
bash scripts/local-docker-stack.sh status
```

Para detener solo sus servicios conservando datos y volumen, ejecuta `bash scripts/local-docker-stack.sh down-frontend` con las mismas variables cargadas. Para volver a levantarlos, carga de nuevo `runtime.env` y ejecuta `bash scripts/local-docker-stack.sh up-frontend`.

Al terminar la revisión y una vez que el usuario decida retirar esta demo, elimina exclusivamente su proyecto Compose y volumen nombrado. No usar comandos globales de limpieza de Docker. La demo y sus cuentas sintéticas quedan retenidas hasta entonces. La pila MPV2-09 y `espacigo_pgdata` son recursos distintos, conservados intactos.

## Recorrido y límites

1. Entra anónimamente a `/explorar`; comprueba ocho categorías, filtros, orden y navegación.
2. Verifica tamaños de página 12/24, disposición de cuadrícula y retorno con Atrás/Adelante.
3. Abre una ficha y pulsa **Reservar** para comprobar el límite de autenticación.
4. Ten presente que fotos y cuentas son fixtures recreables; no hay fotos reales, pagos reales ni despliegue GCP.

La fusión técnica de MPV2-12 y el recorrido de esta demo no equivalen a aceptación del producto. La decisión final corresponde al usuario.
