# Preparación del flujo nativo del wizard por HTTPS

El harness `frontend/tooling/wizard-native-https.mjs` sirve el Next standalone ya construido en `https://127.0.0.1:4206`, proxyando solo hacia `127.0.0.1:4209`; Next apunta al API local existente `http://127.0.0.1:18086/api/v1`. No crea ni inicia una base de datos ni un servicio API.

## Certificado local

Proveer `TLS_CERT` y `TLS_KEY` existentes bajo `.local/testing/wizard-improvements/tls/`. El harness valida que certificados/clave sean archivos regulares, sin symlinks, propiedad del UID actual; directorios TLS `0700` y clave `0600`. No genera certificados ni imprime su contenido.

## Ejecución

Desde `frontend/`, con el standalone existente actualizado:

```sh
TLS_CERT=/ruta/al/cert.pem TLS_KEY=/ruta/a/key.pem node tooling/wizard-native-https.mjs
```

Debe estar libre el puerto público `4206` y el interno `4209`; ambos listeners se atan exclusivamente a loopback. El proceso hijo de Next recibe un entorno limpio y explícito, sin heredar variables de PostgreSQL, credenciales API u otros secretos. La señal SIGINT/SIGTERM cierra el proxy y termina únicamente el hijo Next que lanzó este harness.

Es un transporte HTTPS local para pruebas del navegador, no evidencia de autenticación ni de integración nativa Go/base de datos. El flujo verificado del API 18086 y sus precondiciones se mantienen independientes.
