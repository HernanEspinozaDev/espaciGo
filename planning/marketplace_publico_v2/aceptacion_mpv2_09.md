# Demo local y guía de aceptación MPV2-09

**Estado:** **NO aceptada; el usuario solicitó cambios.** La fusión técnica de MPV2-09 sigue válida como historial, pero no es aceptación de producto. Esta guía/evidencia conserva el baseline de aplicación de MPV2-08; la mejora autorizada se desarrolla por separado en MPV2-10–13.
**Código de aplicación:** `6075b7800b45f15e779d11c3ab5853dadd26238e` (fusión de MPV2-08).
**Demo local:** `http://127.0.0.1:18782`. Solo loopback; no usar túnel ni exponer los puertos.

## Arranque y datos de acceso

La pila está iniciada y se conserva para esta revisión. Desde el worktree de MPV2-09, comprobarla con:

```bash
export LOCAL_DOCKER_PROJECT_NAME=espacigo_mpv2_09_acceptance
export LOCAL_DOCKER_IMAGE_PREFIX=espacigo_mpv2_09_acceptance
export LOCAL_DOCKER_STATE_DIR="$PWD/.local/testing/mpv2-09-demo/runtime"
export LOCAL_DOCKER_API_PORT=18780
export LOCAL_DOCKER_MAILPIT_PORT=18781
export LOCAL_DOCKER_ANGULAR_PORT=18782
export LOCAL_DOCKER_PGDATA_VOLUME=espacigo_mpv2_09_acceptance_pgdata
bash scripts/local-docker-stack.sh status
```

Si está detenida, iniciar y esperar los healthchecks con `bash scripts/local-docker-stack.sh up`. No seleccionar el proyecto Compose `espacigo` ni el volumen `espacigo_pgdata`.

| Superficie | Dirección local | Uso |
| --- | --- | --- |
| Sitio Angular/SSR | [http://127.0.0.1:18782](http://127.0.0.1:18782) | Recorrido de aceptación |
| Go readiness | [http://127.0.0.1:18780/health/ready](http://127.0.0.1:18780/health/ready) | Debe responder HTTP 200 |
| Angular health | [http://127.0.0.1:18782/healthz](http://127.0.0.1:18782/healthz) | Debe responder HTTP 200 |
| Mailpit | [http://127.0.0.1:18781](http://127.0.0.1:18781) | Buzón sintético local; puede mostrar enlaces de verificación, no compartir capturas ni mensajes |

El visitante anónimo no necesita cuenta. Los accesos de ensayo ya creados por los endpoints normales de registro/verificación de Go están en `.local/testing/mpv2-09-demo/accounts.json`, modo `0600`; ese archivo no se versiona. Sus claves son `renter`, `host`, `admin` y `thirdParty`. No copiar emails ni contraseñas a tickets, chats, capturas o logs. La tercera persona tiene rol de arrendatario, sin permiso de anfitrión. Rol de anfitrión, aprobación/elegibilidad KYC local y verificación de la persona arrendataria son **datos sintéticos de esta base**, no una verificación real ni un resultado de un proveedor.

La base conserva tres publicaciones normales creadas en el editor y guardadas/publicadas por Go:

| Título | Categoría | Tarifa pública | Zona |
| --- | --- | --- | --- |
| Demo sintética MPV2-09 Oficina | Oficina | CLP 18.000 / hora | Providencia, Metropolitana de Santiago |
| Demo sintética MPV2-09 Bodega | Bodega | CLP 35.000 / día | Providencia, Metropolitana de Santiago |
| Demo sintética MPV2-09 Quincho | Quincho | CLP 490.000 / mes | Providencia, Metropolitana de Santiago |

Las fotos son PNG sintéticos generados por Go; cada una fue declarada pública desde la galería del anfitrión. No hay fotos reales ni direcciones públicas. La demo retiene una solicitud local en estado «Pendiente de pago fake» para comprobar el recorrido; el aviso de ensayo indica que no se cobra ni se mueve dinero real. Si ese intervalo ya está ocupado, elige otra fecha futura.

## Recorrido de aceptación

1. **Exploración anónima:** abrir `/`, entrar a Explorar, buscar «Demo sintética MPV2-09 Oficina» y filtrar por Oficina. Confirmar que la URL lleva los criterios, la unidad y el precio aparecen en la tarjeta, y que Atrás/Adelante del navegador restauran el estado. Repetir a 390×844 y navegar con teclado.
2. **Ficha pública:** abrir la tarjeta. Revisar PNG, reglas, categoría, zona aproximada y CLP/hora. Confirmar que la dirección privada y la identidad del anfitrión no aparecen. En una pestaña sin ejecutar JavaScript, la respuesta SSR de catálogo/ficha debe contener título/metadatos; una ficha inexistente responde 404.
3. **Identidad contextual y reserva:** desde la ficha pulsar «Iniciar sesión»/«Ver disponibilidad y reservar», entrar con `renter`, comprobar que se vuelve a la misma ficha y no se crea ninguna escritura automática. Elegir una fecha futura en «Fecha local», seleccionar «Horario disponible», pedir la cotización y revisar monto/unidad. Solo al pulsar «Solicitar reserva» Go crea la solicitud local. La página declara «ensayo local», «pago fake» y ausencia de dinero real. No se usó overlay de reloj ni se cambió el reloj del sistema: se eligió una fecha futura real desde el control de la aplicación.
4. **Anfitrión y permisos:** entrar con `host` y revisar «Anfitrión» → «Mis espacios», ubicación, datos y galería. La cuenta `thirdParty` al abrir directamente una ruta de edición de otro anfitrión vuelve a «Mi cuenta»; no puede editar ese espacio. `admin` existe para revisar los controles administrativos locales; no representa una cuenta operativa.
5. **Retirada:** la regresión conectada FE-14, ejecutada en otro proyecto temporal, retiró una publicación sintética y comprobó 404 en ficha SSR e imagen PNG, además de su ausencia del sitemap. No oculté una de las tres ofertas de esta pila retenida para conservar la demo navegable.
6. **Caída/restauración:** la misma regresión temporal observó 503 de catálogo/sitemap mientras Go estaba detenido, HTTP 200 después de restaurarlo y sesión en la misma pestaña. No se detiene el backend de la pila retenida durante una revisión humana.

Para repetir de forma aislada los pasos automatizados de FE-14 (crea cuentas/base efímeras en `.local/testing`, usa otro proyecto/volumen/puertos y los retira al terminar):

```bash
TMPDIR="$PWD/.local/testing" \
FE14_PORT_BASE=18880 \
FE14_PUBLIC_ONLY=1 \
FE14_EVIDENCE_OUTPUT_DIR="$PWD/.local/testing/mpv2-09-demo/regression-evidence/mpv2_06" \
bash scripts/test-frontend-e2e.sh
```

La ejecución de este corte terminó **7/7** escenarios FE-14, incluidos identidad contextual, cotización y reserva explícitas, retirada de imagen y caída/restauración SSR. También comprobó readiness antes de E2E y ausencia de emails/contraseñas sintéticos en logs del backend. El fixture temporal fue retirado; su volumen no es el de la demo ni el persistente de desarrollo.

## Evidencia y reproducibilidad

Las cuatro capturas públicas de esta revisión incluyen viewport CSS, alto/ancho del documento, hora, estado sintético, rol anónimo, commit de aplicación y SHA-256 en sus sidecars JSON y en [evidencia MPV2-09](evidencia/mpv2_09/README.md). Los viewports capturados son 1365×900 y 390×844 CSS px; no hubo desbordamiento horizontal. La captura de confirmación de reserva queda solo bajo `.local/testing/mpv2-09-demo/artifacts/`, porque muestra el estado de una reserva privada sintética.

La pila usa proyecto Compose `espacigo_mpv2_09_acceptance`, volumen exclusivo `espacigo_mpv2_09_acceptance_pgdata` y puertos `18780`–`18782`, todos ligados a `127.0.0.1`. La configuración resuelta no referencia `espacigo_pgdata`; las credenciales runtime están bajo `.local/testing/mpv2-09-demo/runtime/secrets/` con modo `0600`. Tras reiniciar PostgreSQL se conservaron V56 y las tres publicaciones, y Go volvió a readiness HTTP 200. No se creó migración ni cambió OpenAPI en MPV2-09.

## Criterio humano y límites

El usuario revisó la demo y pidió cambios concretos: ordenamiento usable que compare nombre y precio con su modalidad, controles de categoría inmediatamente después de Buscar, filtros propios completos por categoría y una jerarquía explícita entre borradores y filtros aplicados; además, pidió cuadrícula/densidad y cantidad visible. Esos cambios están autorizados y se distribuyen en [MPV2-10–13](plan_perfeccionamiento_publico.md). La decisión final de producto continúa pendiente. Cuando se presente una nueva demo tras MPV2-13, comprobar explícitamente: (a) catálogo y ficha legibles sin cuenta; (b) filtros/URL/orden e historial coherentes; (c) fotografía y tarifa/unidad visibles, sin datos privados; (d) identidad solicitada al reservar, retorno a la ficha y confirmación deliberada; (e) retirada, permisos y estados de error; (f) filtros tipados de las ocho categorías; (g) lista/cuadrícula responsive, cantidad y teclado. Esta guía no registra aceptación en nombre del usuario.

Fuera de este corte: fotos reales, mapas/proveedores externos, pagos reales, GCP y visitas/postulaciones. No hay cuota por IP ni respuesta 429/`Retry-After` para lectura pública; debe definirse antes de exposición externa. Los tiempos de EXPLAIN de fixtures de MPV2-08 no son rendimiento de producción. No es una certificación legal ni WCAG, ni prueba de indexación/ranking en Google.

## Conservación y retiro

**Responsable:** tarea MPV2-09 / HernanMEC, para revisión local del usuario. **Motivo:** dejar una demo navegable hasta que el usuario decida la aceptación. Los contenedores, volumen y credenciales se mantienen; no se ejecutó su retiro.

Cuando el usuario solicite retirarla, desde el mismo worktree ejecutar estas órdenes exactas; `down` no borra el volumen, luego se elimina solo el volumen y directorio identificados para MPV2-09:

```bash
export LOCAL_DOCKER_PROJECT_NAME=espacigo_mpv2_09_acceptance
export LOCAL_DOCKER_IMAGE_PREFIX=espacigo_mpv2_09_acceptance
export LOCAL_DOCKER_STATE_DIR="$PWD/.local/testing/mpv2-09-demo/runtime"
export LOCAL_DOCKER_API_PORT=18780
export LOCAL_DOCKER_MAILPIT_PORT=18781
export LOCAL_DOCKER_ANGULAR_PORT=18782
export LOCAL_DOCKER_PGDATA_VOLUME=espacigo_mpv2_09_acceptance_pgdata
bash scripts/local-docker-stack.sh down
docker volume rm espacigo_mpv2_09_acceptance_pgdata
rm -rf -- .local/testing/mpv2-09-demo
```

No ejecutar `docker volume prune`, `down --volumes` sobre otro proyecto ni una eliminación amplia bajo `.local/testing`.
