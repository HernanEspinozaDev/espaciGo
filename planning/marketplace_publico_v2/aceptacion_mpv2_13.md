# Guía de revisión técnica MPV2-13

**Estado:** producto aceptado por el usuario para el estado UX/demo/código indicado abajo el 2026-10-14; revisión técnica y eventual aprobación/fusión del PR #307 continúan pendientes.
**Fuente y alcance de aceptación:** mensaje explícito del usuario en este chat el 2026-10-14: revisó y aprueba el estado actual, con futuros cambios a solicitar después. Aplica a la presentación y estado de esta demo/código solamente; no implica aceptación de MPV2-09, otras capacidades, fotos/pagos reales o GCP. Cualquier cambio posterior será una tarea nueva.
**Decisión UX registrada:** 2026-10-14 — escritorio fijo a 4 columnas, móvil en lista de 1 columna; selector 12/24/48 junto al orden y visible en móvil.
**Código de aplicación:** `736f81a367eed79bd87950e2dc64716adc6469db`, cambio MPV2-13 en PR #307 (base `c797f0eb734e2c7eda37eb70a50d4d509e82677d`).
**Responsable del ensayo:** HernanMEC. **Motivo:** demo aislada solicitada para revisión del catálogo público.

## Abrir la demo

El navegador local puede abrir `http://127.0.0.1:20002/explorar`. La API está publicada solo en loopback en `127.0.0.1:20000`; Mailpit usa `127.0.0.1:20001`. PostgreSQL no publica un puerto al host. El proyecto Compose es `espacigo_mpv2_13_demo_a2bf4ff1f4ef` y su único volumen de base de datos es `espacigo_mpv2_13_demo_a2bf4ff1f4ef_pgdata`.

La demo tiene más de 48 publicaciones ficticias en las ocho categorías, tarifas CLP por hora/día/mes y valores de filtros tipados. Se puede explorar sin iniciar sesión: cambiar categoría, aplicar filtros, ordenar, elegir 12/24/48 resultados por página y abrir fichas con fotos sintéticas. El catálogo muestra cuatro columnas en escritorio y tarjetas en una lista de una columna en móvil. Para revisar el límite de acceso, usa **Reservar** en una ficha; el acceso privado requiere autenticación. La reserva es simulada y no procesa pagos.

Las cuatro cuentas sintéticas están en `.local/testing/mpv2-13-demo/a2bf4ff1f4ef/accounts.json`, con permisos `0600`. Consulta ese archivo localmente para las credenciales; no copies su contenido en capturas, logs ni archivos compartidos. El archivo de configuración operativo es `.local/testing/mpv2-13-demo/a2bf4ff1f4ef/runtime.env`, también `0600`.

## Estado y ciclo de vida

Para consultar los contenedores de esta demo, desde la raíz del repositorio:

```sh
set -a
. .local/testing/mpv2-13-demo/a2bf4ff1f4ef/runtime.env
set +a
bash scripts/local-docker-stack.sh status
```

Para detener solo sus servicios conservando datos y volumen, ejecuta `bash scripts/local-docker-stack.sh down-frontend` con las mismas variables cargadas. Para volver a levantarlos, carga de nuevo `runtime.env` y ejecuta `bash scripts/local-docker-stack.sh up-frontend`.

Al terminar la revisión y una vez que el usuario decida retirar esta demo, elimina exclusivamente su proyecto Compose y volumen nombrado. No usar comandos globales de limpieza de Docker. La demo y sus cuentas sintéticas quedan retenidas hasta entonces. La pila MPV2-09 y `espacigo_pgdata` son recursos distintos, conservados intactos.

## Recorrido y límites

1. Entra anónimamente a `/explorar`; comprueba ocho categorías, filtros, orden y navegación.
2. Verifica tamaños de página 12/24/48, cuatro columnas en escritorio, lista móvil y retorno con Atrás/Adelante. Una URL antigua con `page_size=20` se conserva; los parámetros viejos de vista/columnas se eliminan al normalizar la navegación.
3. Abre una ficha y pulsa **Reservar** para comprobar el límite de autenticación.
4. Ten presente que fotos y cuentas son fixtures recreables; no hay fotos reales, pagos reales ni despliegue GCP.

La demo previa `espacigo_mpv2_13_demo_316ee26df160` en 19900–19902 se conserva sin cambios como evidencia histórica; esta demo nueva usa un proyecto y volumen separados. La pila MPV2-09 y `espacigo_pgdata` también permanecen intactos. No se reinició ni borró ninguno de esos recursos.

El usuario aceptó el 2026-10-14 el estado de producto documentado para UX/demo/código `736f81a367eed79bd87950e2dc64716adc6469db`; su instrucción indica que solicitará futuros cambios después si los necesita. Esta decisión de producto no sustituye la revisión técnica, aprobación o fusión del PR #307, que siguen pendientes, ni amplía la aceptación a otras capacidades/cortes. La demo permanece retenida para que el usuario pueda seguir usándola.
