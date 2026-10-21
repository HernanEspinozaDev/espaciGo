# GCP-08 — restauración inicial de Cloud Run con V67 (2026-10-21)

## Estado y alcance

Este cambio prepara un flujo manual, primero con vista previa, para crear los tres servicios de Cloud Run de `testprod` usando imágenes fijadas por digest publicadas desde `main`. No ejecuta `terraform apply` ni afirma que exista un servicio desplegado. Cuando los tres servicios existan y el control de versión base V67 pase, el flujo automático existente podrá actualizar imágenes.

La restauración inicial permite únicamente la creación de `google_cloud_run_v2_service.api[0]`, `google_cloud_run_v2_service.web["public"]` y `google_cloud_run_v2_service.web["private"]`, además de ocho bindings IAM exactos requeridos para invocación, despliegue y lectura del secreto runtime. Rechaza cualquier otro cambio, incluidos recursos SQL, secretos, redes, buckets, cuentas, roles nuevos o cambios de IAM fuera de esa lista. No crea ni modifica contraseñas, logins PostgreSQL o grants SQL.

El destino está fijado a proyecto `espacigo-testprod`, región `southamerica-west1`, instancia Cloud SQL `espacigo-testprod-db`, base `espacigo_dms_stage_20261020`, usuario de conexión `espacigo_api_login`, rol de aplicación `espacigo_runtime` y secreto `espacigo-test-database-password-v67`, versión `1`. El archivo de contraseña nunca entra a Terraform, GitHub Actions ni el artefacto de vista previa.

La migración V67 está en `main` por PR #473. La restauración no copia ni vuelve a ejecutar migraciones. Los manifiestos bloquean imágenes cuya base de esquema no incluya V67.

## Evidencia V67 y corrida más reciente

La verificación local protegida por túnel confirmó que el login conecta a la instancia y a la base V67, puede asumir `espacigo_runtime`, leer las 1.241 publicaciones y tiene permisos de escritura de aplicación; no tiene permisos para crear bases, roles o esquemas. La prueba no dejó cambios persistentes. La instancia usa codificación `UTF8` y colación `es_ES.utf8`.

El secreto runtime está habilitado en la versión `1`. El registro V67 queda completo, pero el estado operativo del flujo sigue pendiente porque falta revisar y aprobar el nuevo PR de esta restauración antes de ejecutar la vista previa privilegiada.

| Corrida | SHA | Resultado | Uso |
| --- | --- | --- | --- |
| [Application release #39551584696](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/39551584696) | `b1b38d1883a34096f9027375dd2d5737c0abb043` | Éxito | Publicó las imágenes API y web fijadas por digest para V67. |
| [Application quality #39551584704](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/39551584704) | `b1b38d1883a34096f9027375dd2d5737c0abb043` | Éxito | Calidad backend del release V67. |
| [Frontend architecture and quality #39551584698](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/39551584698) | `b1b38d1883a34096f9027375dd2d5737c0abb043` | Éxito | Calidad frontend del release V67. |
| [Application quality #39557422812](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/39557422812) | `20003d550cac9ee9e0c8d87a70b4460074b58ccb` | Falló | La prueba asumió que la primera mutación siempre empezaba antes de un deadline de 25 ms; en el runner expiró antes de registrar llamadas. La prueba se corrigió en este PR para aceptar ambos casos seguros y prohibir siempre avanzar a la siguiente mutación. |
| [Application rollout #39557443643](https://github.com/HernanEspinozaDev/espaciGo/actions/runs/39557443643) | `20003d550cac9ee9e0c8d87a70b4460074b58ccb` | Éxito con trabajos omitidos | No desplegó; los gates dejaron los trabajos de rollout en `skipped`. |

El registro de preflight usa la aprobación de `HernanEspinozaDev` sobre el HEAD de PR #473 (`0637ca0dd2b239c44f4f38236c82328594d7152c`) y sus tres corridas exitosas sobre ese mismo SHA. La publicación de las imágenes ocurrió luego en el merge commit `b1b38d1…`; ambas evidencias se distinguen para no atribuir la publicación al commit equivocado.

## Flujo Terraform y controles

1. El workflow `Application rollout` se ejecuta manualmente desde `main` en operación `initial-restore / preview`.
2. El job privado inicializa Terraform con el estado remoto canónico y obtiene las imágenes publicadas por release; el plan permanece en el runner y nunca se publica.
3. `verify_gcp07_plan.py` acepta solo los tres servicios exactos y estos ocho bindings: los dos invokers API de los frontends, tres permisos del deployer sobre cada servicio, acceso público al frontend público, invocación del frontend privado por el agente IAP del proyecto y acceso del API al secreto V67.
4. El resumen descargable contiene solo direcciones permitidas, acciones, imágenes por digest, miembros/roles IAM exactos, SHA, corrida y huella. El validador rechaza bindings omitidos, añadidos o alterados.
5. Una persona revisa el resumen y la huella. `initial-restore / apply` vuelve a calcular el plan, verifica la aprobación y la frase exacta de confirmación y aplica solo el plan privado recién calculado.
6. Tras verificar servicios saludables y el acceso de API a Cloud SQL, los futuros cambios de aplicación en `main` publican imágenes nuevas y el workflow automático hace rollout de cambios de imagen solamente.

Los inputs Terraform protegidos deben llegar por `GCP07D_TFVARS_JSON`; el workflow fuerza los valores V67, fija la versión del secreto y desactiva creación de Cloud SQL y selección de candidatos GCP-06. Antes del preview deben existir las variables `GCP07D_ROLLOUT_ENABLED=true`, `GCP08_V67_DATABASE_PASSWORD_VERSION=1` y `GCP08_V67_RUNTIME_PREFLIGHT_SHA=a776af14a29fcce0860ff6b344a27ecd35044b7f99b0ebffafaf5f61fe76e8d7`, más el secreto `GCP07D_TFVARS_JSON` con los inputs Terraform protegidos. El hash corresponde exactamente a `gcp08_v67_runtime_preflight.json`; si cambia el registro, debe recalcularse. Los usuarios de IAP para el demo privado siguen sujetos a `iap_evaluator_members`; el primer bootstrap no inventa identidades personales.

## Estado observado y trabajo pendiente

- No hay servicios Cloud Run desplegados en la región objetivo; el último rollout no ejecutó un apply.
- `GCP07C_PUBLISH_ENABLED=true`; falta terminar de configurar y verificar los inputs protegidos para restauración inicial, incluida la habilitación explícita del workflow.
- El registro preflight V67 refleja los checks y la versión del secreto verificados, y enlaza la aprobación y las corridas del corte de esquema. Su uso para el nuevo despliegue no reemplaza la revisión del plan Terraform.
- La corrida fallida de Application quality permanece como evidencia histórica; el ajuste a la prueba pasó localmente 20 ejecuciones consecutivas del paquete. El check de GitHub de este PR debe pasar antes de considerar validado el pipeline.
- No se ha generado ni aprobado una vista previa Terraform de los tres servicios y ocho bindings; no se conoce aún el plan real de Cloud.
- No se ha probado el servicio Cloud Run contra la base, ni el acceso público/privado desde navegador.

## Validación de este cambio

La validación de este corte ejecuta las pruebas de política del bootstrap y del resumen, 20 ejecuciones del paquete Go cuya prueba falló, formato y validación Terraform y `git diff --check`. Estas pruebas validan localmente la lista exacta de recursos permitidos; no prueban IAM efectivo, conectividad Cloud SQL, estado Terraform remoto ni un despliegue real. No se ejecuta un plan remoto ni un apply como parte de este PR.
