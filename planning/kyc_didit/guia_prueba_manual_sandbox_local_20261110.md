# Guía de prueba manual Didit Sandbox local

## Alcance

Ensayo manual local autorizado: registro → correo capturado localmente → verificación de correo → acceso → guardar identidad → acción separada Didit Sandbox → retorno y consulta de estado. No es aceptación Live ni cierre de R2/R3/R4. No se han creado sesiones Didit reales desde el agente ni se ha validado una aprobación real del proveedor.

**Estado de entrega:** frontend, proxy, buzón y API arrancados y disponibles para la prueba manual. El binario API ya fue reemplazado con las cuatro rutas de registro/verificación del perfil manual. Readiness200 y términos200 por HTTPS BFF; las cuatro rutas responden400 a JSON deliberadamente incompleto, sin crear cuentas. El recorrido con una cuenta y la decisión real Didit siguen pendientes de ejecución humana, no se presentan como aprobados.

## Direcciones

- Aplicación: **https://127.0.0.1:4219/registro**.
- Buzón de ensayo: **http://127.0.0.1:11080/**. No hay entrega a un correo externo; solo captura local en memoria.
- Verificación de cuenta tras ingresar: **https://127.0.0.1:4219/mi-cuenta/verificaciones**.
- API interna: `http://127.0.0.1:18090`; Next interno4201. No abrirlos como sustituto del frontend HTTPS.
- Webhook fijo: `https://didit.testingpage.store/api/v1/verifications/didit/webhook`. Cloudflare configuraciónv2 apunta a18090 y conserva catch-all404. No hay que cambiar otra vez la URL en Didit.

Usar siempre **127.0.0.1**, no localhost. Certificado HTTPS local autofirmado: el navegador puede mostrar una advertencia; aceptar únicamente la excepción para esta dirección loopback y el certificado local preparado. No desactivar la verificación TLS global ni importar una CA de terceros. El agente no instala confianza global ni abre el navegador del usuario.

## Recorrido manual

1. Abrir `/registro` y crear una cuenta sintética autorizada por la política local privada. La primera pantalla dice «Crea tu cuenta» y solicita correo/contraseña; avanzar por los pasos visibles y aceptar los documentos legales vigentes cuando aparezcan. No registrar una identidad arbitraria ni reutilizar cuentas ajenas.
2. Abrir el buzón local. Los mensajes permanecen solo en RAM, con máximo20 y caducidad15min; reiniciar el buzón pierde los mensajes. Abrir «Abrir verificación en desarrollo»: el token viaja en el fragmento `#`, no en query. Confirmar explícitamente la verificación en la pantalla.
3. La verificación de correo no inicia sesión. Ir a `/acceso` e ingresar con esa misma cuenta.
4. Abrir Mi cuenta → Verificaciones. Completar identidad sintética autorizada y consentimiento, **guardar** y esperar la confirmación/readback. Guardar no crea una sesión Didit.
5. Revisar el aviso/consentimiento independiente y pulsar la acción para verificar identidad con Didit. Solo esta acción solicita una sesión Sandbox externa.
6. Usar únicamente los documentos/medios de ensayo permitidos. **Sandbox guarda las capturas de verdad**: no usar documento real, selfie/cámara de una persona ni datos biométricos por inferencia de esta guía. La autorización de desarrollo sintético no los habilita.
7. Al volver a `/verificaciones/retorno`, la UI elimina el query de la barra y consulta el intento. Esperar o usar la consulta explícita si queda pendiente. El retorno del navegador por sí solo no aprueba nada: Go valida webhook y consulta autoritativa al proveedor.
8. Una aprobación Sandbox no debe mostrar una cuenta verificada para uso real. «En revisión» puede ser el resultado honesto del guard Sandbox≠Live; no se retira ese control para que el ensayo parezca aprobado. Un error de proveedor o de comparación se reporta, no se sustituye por una respuesta simulada.

## Configuración fijada

Perfil `local-kyc-didit-sandbox-manual`, sin workers generales; DB nativa `127.0.0.1:5432/espacigo_dms_stage_20261020`, login API separado y SET ROLE runtime. Aplicación Sandbox `806fe3f4-4c9b-4567-bf6f-ce8c3e688fce`; workflow publicado `a4b973d4-4578-44fc-8580-7554e39821ce`, versión1. El archivo histórico apunta a otro workflow borrador: el lanzador fija explícitamente el publicado inspeccionado sin editar proveedor ni claves. API base primaria `https://verification.didit.me`.

Credenciales Didit preexistentes se cargan privadamente en el proceso API; no se imprimen, no se copian a Next y no se rotan por cambiar URL. Keyrings locales AES/HMAC distintos en `.local/testing/kyc-manual/`, versión `manual-20261110`, propios0600 y directorio0700. **No regenerarlos ni borrarlos mientras haya registros dependientes**, aunque se reinicie la aplicación. No se reemplazaron claves históricas autorizadas.

Privacidad Didit: el usuario informó retención de1año, consentimiento automático y eliminación de la plantilla con la sesión; el objetivo30d/siempre no consta guardado independientemente. Esta guía no lo presenta como ya aplicado y no autoriza medios reales.

## Reinicio reproducible

Desde `/home/nandev/Documentos/projects/espaciGo-main-sync`, con los servicios propios previamente detenidos:

```sh
# Solo antes del arranque: comprueba artefactos, archivos privados y puertos libres.
python3 scripts/kyc_manual_local.py check
```

Abrir cuatro terminales y ejecutar una orden por terminal, desde ese mismo directorio:

```sh
python3 scripts/kyc_manual_local.py run-mailbox
python3 scripts/kyc_manual_local.py run-api
python3 scripts/kyc_manual_local.py run-frontend
python3 scripts/kyc_manual_local.py run-proxy
```

Cada comando corre en primer plano y se detiene con Ctrl+C. No arrancar duplicados sobre los procesos que Hermes mantiene para este ensayo. El túnel es un servicio usuario separado:

```sh
systemctl --user is-active espacigo-didit-tunnel.service
systemctl --user restart espacigo-didit-tunnel.service
```

Reiniciar el túnel conserva hostname y credencial; no crea otro Quick Tunnel. El equipo y los procesos API/túnel deben permanecer encendidos para recibir webhooks. Una sesión de Hermes cerrada puede retirar sus procesos locales: usar las órdenes anteriores para levantar de nuevo, sin tocar las claves.

Los binarios ya construidos están en `.local/testing/kyc-manual/bin/`. Si cambia código, detener primero únicamente los procesos propios; después compilar y repetir gates. No compilar sobre un standalone en ejecución. El lanzador no instala paquetes ni cambia DB por sí mismo.

## Evidencia y límites

- Backup previo `.local/testing/kyc-manual/pre-manual-sandbox-20261110.dump`,0600, SHA256 `e2893deccc92c77bddcfebcf08ea87e5708038824a8c0bfde6247d5b784cca49`; listado/parse completo PASS.
- Se corrigió el permiso del directorio raíz del worktree0775→0755 para satisfacer el guard de ancestros del passfile, sin debilitarlo ni cambiar credenciales. Preflight real con el entorno del lanzador: perfil/config/keyrings/construcción de servicios y transacción READ ONLY con login/rol/base exactos PASS.
- Race/vet Go combinados PASS; Python11 y Node proxy2 PASS; contratos/aislamiento frontend28PASS; build producción PASS. El test nativo opt-in de preflight se omite por defecto; no convertir su SKIP en evidencia de conexión.
- API ready200, términos200 directamente y por BFF HTTPS, identidad anónima401. Buzón HTTP/SMTP y Host421 comprobados. Playwright aislado contra4219 verificó solo el primer paso visible del registro, sin rellenar ni enviar formulario ni solicitudes externas.
- Las cuatro rutas nuevas POSTregister/verification/status/reissue están montadas solo en el perfil manual; pruebas con sentinel conservan404 en fake y denegación de rutas ajenas. Tras el reemplazo del binario, probes reales HTTPS BFF con cuerpo JSON incompleto devolvieron400 en las cuatro, la misma mutación sin Origin403 y el perfil ajeno404. No se envió una solicitud válida de registro ni se consumió un token. API final PID120498/handleproc_8ddc1ddc41cf; receipts bajo `.local/testing/kyc-manual/`, junto a los de frontend105137/proxy105185/buzón103733.
- Cloudflare probado con curl: raíz404 y webhook sin firma401; el rechazo sin firma prueba enrutamiento y protección, **no** prueba webhook firmado de Didit. urllib obtuvo403 en ese mismo ensayo; se registra la discrepancia de cliente, no se reemplaza un resultado por otro sin decirlo.
- Callback service tiene pruebas de construcción de URL con attemptID; falta prueba conductual runtime→clienteHTTP que se había solicitado al delegado. El cableado fue leído y la regresión pasó; no presentar esa brecha como prueba completa.
- No cuenta nueva ni login ni declaración persistente ni sesión/aprobación real creados por el agente en esta preparación. El recorrido completo queda para la prueba manual del usuario, y las operaciones deben seguir la política privada.
- Sin migraciones nuevas, Cloud SQL, workers generales, despliegue, commit, push o integración main. V1–V94 se preservan. Detener/limpiar solo los recursos propios cuando el usuario termine; no borrar la DB, cuentas o datos retenidos por inferencia.
