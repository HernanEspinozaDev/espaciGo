# G03O V64: release de imagen y prevalidación read-only

Fecha: 2026-10-20. Esta evidencia documenta una autorización del usuario para publicar la imagen G03O y ejecutar una sola prevalidación de solo lectura en Cloud, sin transferencia ni cambios de permisos.

## Imagen

- Código: commit integrado `810240faca40bc0a296db0a322b485f4239d77ef` (`docs(gcp): plan local dataset transfer and CI/CD`). El árbol de trabajo estaba limpio antes de construir.
- Artefacto OCI local, guardado en `.local/testing/g03o-1017-image-release-20261020/transfer.oci` con permisos `0600`: digest de manifiesto `sha256:f13ec70403c04edd7aee498bc49146cc90036169f3f4a760dcbd0c193dcdf694`, SHA-256 del archivo `74b7ba0860d855f64412de7010f9c4442e215692442948100ebca66698b7186f`, tamaño 260.943.360 bytes.
- Artifact Registry ya contenía ese mismo digest en `southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/g03o-v64-transfer`, con tag `g03o-1017-owner-fix-810240f`. La lectura independiente por digest coincidió; no se volvió a subir el artefacto.
- El Job estaba fijado al digest anterior `sha256:b2972f4501296aeef17b2f6719b87a9ecb6e7f5eeb2cd787d4f9997c73141e45`. Se actualizó solo la imagen del contenedor `transfer` al digest verificado. El Job pasó de generación 10 a 11. La plantilla conservó `GCP03_V64_TRANSFER_EXECUTE=NO`, destino de confirmación fijo, una tarea, paralelismo 1, cero reintentos y la misma cuenta de servicio. El cliente gcloud añadió las anotaciones de cliente/nombre y versión al metadata del template. No se cambiaron IAM, ACL, secretos, buckets, state Terraform ni servicios.

## Ejecución única

- Proyecto/región/Job: `espacigo-testprod`, `southamerica-west1`, `espacigo-g03o-v64-transfer`.
- Ejecución: `espacigo-g03o-v64-transfer-xvwfg`; imagen fijada al digest anterior; `EXECUTE=NO`; una tarea y cero reintentos.
- Resultado: falló con `SOURCE_OWNER_PREFLIGHT_FAILED` en `source_preflight`; `transaction_outcome=not_started`, `retry=false`. El informe saneado indicó una relación requerida ausente, más de un propietario en el grupo inspeccionado, acceso incompleto del login y ninguna ruta segura de lectura. No se publicaron nombres de relaciones o propietarios.
- El flujo valida el informe del origen antes de consultar el destino o revisar objetos; por eso se detuvo antes de `target_preflight`, comprobaciones de objetos, transacción de transferencia y escrituras. No se transfirieron filas ni archivos. No se concedieron permisos y no hubo reintento ni segunda ejecución.
- Esta lectura confirma que la ruta llega al gate de propietario/acceso y se detiene de forma segura. No acredita compatibilidad del origen completo, estado del destino, permisos de Cloud SQL, importación, ni una transferencia exitosa.

## Estado y siguiente paso

La transferencia G03O→V64 sigue pendiente. La brecha de relación/propiedad requiere resolución local y revisión del inventario antes de cualquier nuevo preflight remoto; no se deriva una solicitud de grant de este resultado. Cualquier otra ejecución, lectura Cloud adicional, cambio de permisos o transferencia necesita autorización separada.
