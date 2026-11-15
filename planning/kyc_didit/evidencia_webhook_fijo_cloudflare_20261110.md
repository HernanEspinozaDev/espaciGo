# Webhook Sandbox con hostname fijo — 2026-11-10

## Autorización y alcance

Usuario: «ya crealo con el que usa espacigo.testingpage.store no se ponele didit crealo en cloudflare asi no lo tengo que andar cambiando a cada rato asi lo configuro de nuevo».

Se creó únicamente `didit.testingpage.store` dentro de la zona existente `testingpage.store`; no se modificó `espacigo.testingpage.store`, la aplicación pública, su backend ni Didit. No hubo despliegue GCP, SQL, creación de sesión de verificación ni uso de identidad/biometría.

## Recursos creados y lectura independiente

- Cuenta Cloudflare: `039658d8d8ea5e8cc90953d362f61a10`.
- Zona: `a33ba541ab27cd4d4987c28889085ee7`.
- Túnel remoto nombrado: `espacigo-didit-sandbox`, ID `34945f81-2a52-4813-8de9-f30efb9925e7`, `config_src=cloudflare`.
- DNS CNAME: `didit.testingpage.store` → `34945f81-2a52-4813-8de9-f30efb9925e7.cfargotunnel.com`, proxied=true, TTL automático; record ID `7ec788e5b461a98d13c7c4b7b273de12`.
- GET independientes de túnel/configuración/DNS: correctos. Estado remoto `healthy`, cuatro conexiones.

Ingress exacto:

1. Host `didit.testingpage.store`, path regex `^/api/v1/verifications/didit/webhook$`, servicio `http://127.0.0.1:8080`.
2. Cualquier otra ruta/host: `http_status:404`.

No se publica toda la API. La firma y frescura del webhook siguen correspondiendo al backend; la ruta pública no sustituye esos controles.

## Conector local persistente

- Servicio systemd de usuario: `espacigo-didit-tunnel.service`.
- Unidad: `/home/nandev/.config/systemd/user/espacigo-didit-tunnel.service`.
- Binario copiado fuera de la caché npm: `/home/nandev/.local/share/espacigo-cloudflare-didit/cloudflared`.
- Credencial únicamente en `tunnel.token` dentro de ese directorio privado0700, archivo0600; token pasado por `--token-file`, no por argumentos con su valor ni logs.
- Transferencia de credencial cifrada con clave pública temporal y AES-GCM; clave privada temporal retirada tras instalación. Ningún secreto en este documento/repositorio/chat.
- `systemd-analyze --user verify` verificó la unidad; emitió advertencia de unidad ajena `spice-vdagent`, no del túnel. `systemctl --user is-active`=active, `is-enabled`=enabled; `loginctl show-user` confirmó Linger=yes ya existente, sin cambiarlo.
- Restart=on-failure/5s y arranque con target por defecto del usuario. No implica disponibilidad si el equipo está apagado, sin red o la API detenida. El hostname nombrado no depende de la URL aleatoria de un Quick Tunnel.
- Quick Tunnel propio `proc_f6635a45ddd1` detenido tras verificar el nombrado; no se tocaron otros túneles de la cuenta.

## Pruebas y límite actual

- GET HTTPS a `/`: HTTP404, como exige el catch-all.
- GET HTTPS a `/api/v1/verifications/didit/webhook`: HTTP502.
- Listener8080 ausente, confirmado por `ss`: el502 refleja origen local apagado, **no prueba de receptor webhook funcional**.
- No se levantó API estándar ni se sustituyó respuesta con un servidor ficticio. No se enviaron POST ni webhooks sintéticos hacia Didit.

URL estable para configurar en el destino Sandbox existente o en su reemplazo:

`https://didit.testingpage.store/api/v1/verifications/didit/webhook`

Falta levantar el perfil local de API compatible con Sandbox real bajo sus guards autorizados y configurar el destino en Didit. Si se crea un destino nuevo, su secreto de firma se debe inyectar privadamente al backend; el token del túnel, la clave API Didit y ese secreto son credenciales distintas. No se ha modificado ninguna credencial Didit.

## Operación

- Estado: `systemctl --user status espacigo-didit-tunnel.service`.
- Reiniciar mismo túnel/hostname: `systemctl --user restart espacigo-didit-tunnel.service`.
- Detener solo este conector: `systemctl --user stop espacigo-didit-tunnel.service`.
- Configuración de rutas remota: gestionar solo el ID de túnel indicado; preservar catch-all404 y no ampliar a toda la API por inferencia.
