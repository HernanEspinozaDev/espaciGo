# Guía de revisión — demo LOCAL-CLOSE-02

Fecha: 2026-10-14 (America/Santiago). Esta es una demo local, aislada y sintética para revisión; la verificación técnica no constituye aceptación del producto.

## Dirección, versión y datos

- Angular (incluye SSR): [http://127.0.0.1:21442/explorar](http://127.0.0.1:21442/explorar); el smoke de SSR independiente usó temporalmente el puerto `21443` durante la prueba y fue detenido al retener el demo. API: `http://127.0.0.1:21440`; Mailpit: [http://127.0.0.1:21441](http://127.0.0.1:21441).
- Proyecto Compose: `espacigo_localclose02_demo_bb5f703b8c7c`; volumen: `espacigo_localclose02_demo_bb5f703b8c7c_pgdata`. Puertos publicados solo en loopback. Base PostgreSQL sin puerto host; `espacigo_pgdata` no está montado.
- RUN_ID: `bb5f703b8c7c`. Código de aplicación probado: `0da50a3a7c3f201b50c4aa7abcfd475bddbd9052`. El sidecar generado por Playwright registra el HEAD de ejecución de la demo (`0885abf17bd31f57e914b08e48942b4eb4f64b19`); el commit de esta guía añade solo documentación/evidencia.
- Responsable: `HernanMEC`. Motivo: retener una demo LOCAL-CLOSE-02 aislada mientras la aceptación de producto sigue pendiente.
- Cuentas sintéticas recreables: `.local/testing/local-close-02-demo/bb5f703b8c7c/accounts.json`, modo `0600`, excluido de Git. Sus claves son `renter`, `host`, `admin` y `thirdParty`; usar las credenciales del archivo local y no copiarlas a capturas, logs o mensajes. Roles: arrendatario, arrendador, administrador y arrendatario tercero.
- IDs del recorrido: reserva `67db3ab1-7bbe-43ad-b415-8eff5cbba463`; espacio `c93431ac-5980-4a38-80d1-2c2dd452dadf`; contrato `4f9c3c16-0b0b-4679-a60f-3bbc310c1a63`; incidente `f86fe030-c0e2-4951-96f9-7a08e5633609`; reclamo `f6fbebb3-3e4c-4114-be79-802b67287234`; liquidación `974e7850-a547-420d-9c5b-34b7b589a749`; recibo `e5f663da-81e5-4ede-aa13-da8d8944c83f`. El JSON íntegro y sin credenciales está en [local-close-02-demo-bb5f703b8c7c-journey.json](local-close-02-demo-bb5f703b8c7c-journey.json).

## Recorrido visible

Usar el rol arrendatario y los siguientes enlaces:

1. `/reservas` → abrir **Ver solicitud**. Se espera estado **Cerrada** y la entrada correspondiente en el historial. Capturas: [escritorio](fe14_local_close_02_closed_history_renter_desktop_bb5f703b8c7c.jpg) y [móvil 390×844](fe14_local_close_02_closed_history_renter_mobile_bb5f703b8c7c.jpg).
2. `/reservas/67db3ab1-7bbe-43ad-b415-8eff5cbba463/disputas` → pulsar **Actualizar desde Go**. En **Reclamo y descargo** se ve `resuelta`; en **Comprobante de ensayo**, `liquidada` y `comprobante_ensayo`. Capturas: [escritorio](fe14_local_close_02_resolved_receipt_renter_desktop_bb5f703b8c7c.jpg) y [móvil 390×844](fe14_local_close_02_resolved_receipt_renter_mobile_bb5f703b8c7c.jpg). La página se puede volver a cargar desde Go.
3. Como arrendador, la misma ruta de disputa muestra la liquidación y comprobante de ensayo. La vista autenticada desktop está en [cierre financiero](fe14_local_close_02_closed_finance_desktop_bb5f703b8c7c.jpg).

Sidecars JSON junto a cada imagen registran viewport, tamaño del documento y SHA de ejecución. Hashes SHA-256 de imágenes:

| Captura | SHA-256 |
| --- | --- |
| Cierre financiero arrendador desktop | `4c05f2e82eda7bc4a758773da76d81c7b86bcaa19151452a035b1a38c3cbe9bb` |
| Historial cerrado arrendatario desktop | `795afa987048bfd41fe10aebd0c75510d6e2040c5b0b979d3055c0c248ece385` |
| Historial cerrado arrendatario móvil | `189b83a5953738042e242b24da065ec5ceeefc430eb6a0a6c22f13680912ea2b` |
| Reclamo resuelto y recibo desktop | `d2b2b736dc1f037944eefca0353a13bf8b8c33a6c253b2deb3bff76710b1b827` |
| Reclamo resuelto y recibo móvil | `04d93ccee1ca9615746a564cd9b07b7b01a8c5e8b437c66a6970091e5aebde7c` |

## Privacidad y límites observados

Con el rol `thirdParty`, la API contestó HTTP 404 para leer reserva, leer comprobante, crear liquidación y leer liquidación. Los cuatro códigos quedan consignados como `404` en el JSON de recorrido. La lectura del recibo fue 200 para el arrendatario autorizado; el test comprobó también no desbordamiento horizontal a 1365×900 y 390×844. El único comprobante es `comprobante_ensayo`: no se movió dinero real ni se llamó a un proveedor externo. Las acciones posteriores a la firma del arrendatario se ejecutaron por API desde Playwright contra el Go real; Angular verificó lectura, historial y vistas del reclamo/recibo. No se simuló que todas las mutaciones se hicieran con botones de UI.

Prueba de reemplazo: `scripts/prepare-local-close-02-demo.sh`, 7/7 pruebas Playwright PASS; el runner comprobó readiness, logs sin credenciales y fallo/recuperación SSR ante indisponibilidad de Go. La demo continúa encendida para revisión. La aceptación final de producto corresponde al usuario.
