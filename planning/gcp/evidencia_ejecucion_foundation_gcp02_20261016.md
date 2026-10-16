# Evidencia de ejecución de GCP-02 Foundation

Fecha: 2026-10-16 (UTC). Proyecto `espacigo-testprod` (`447373903153`), región `southamerica-west1`. El plan binario revisado en PR #333 se aplicó desde el HEAD `56211aaae37181d1fe876af70069f070df8af346`, con Terraform 1.16.6. Esta evidencia registra solo el root Terraform `gcp` y el plan exacto; no completa GCP-02 ni acredita aceptación de producto.

## Aplicación exacta

- SHA-256 del plan aplicado: `11cf459fe59669d4ad7747cc24b68e3c53ce9ad873574c3211450f6c3f3f9e8c`.
- Resultado: **41 recursos creados, 0 modificados, 0 destruidos**. El estado remoto conserva su lineage; serial `1 → 7`, 41 direcciones y 41 instancias. SHA-256 del state exportado después del apply: `ebb50b0e3a772ce68c68a92065fce9502bf7c5fd0565f499c3f826e07ea67770`.
- La verificación `terraform plan -refresh=true -detailed-exitcode` posterior terminó con código `0`: **sin cambios**.
- Plan, state, respuestas completas y logs permanecen bajo `.local/planning/gcp/foundation-plan-20261016/`, con permisos privados; no se versionan. Los hashes permiten identificar los artefactos sin publicar su contenido.
- Tras el apply y el plan de verificación, el único objeto vigente del backend es `default.tfstate`. El bucket versionado retiene cinco generaciones no vigentes de `default.tflock`; no existe lock actual. No se purgaron versiones históricas.

Los 41 recursos corresponden a las 13 declaraciones de API (incluyendo dos ya habilitadas antes), red VPC/subred, cinco cuentas de servicio, Artifact Registry, bucket privado, tres metadatos de secretos, Workload Identity Federation y los grants del plan. Las 14 vinculaciones IAM revisadas son: 3 en el proyecto (`roles/run.developer` y dos `roles/cloudsql.client`), 5 en cuentas de servicio (cuatro `roles/iam.serviceAccountUser` y `roles/iam.workloadIdentityUser`), una `roles/artifactregistry.writer`, una `roles/storage.objectAdmin` y cuatro `roles/secretmanager.secretAccessor`. No se almacenaron valores de secretos ni se crearon versiones.

## Verificaciones posteriores

- IAM: las 14 vinculaciones planeadas existen en los destinos previstos; el principal propietario observado antes y después no cambió. La política de proyecto no contiene `roles/editor`, `allUsers` ni `allAuthenticatedUsers`. La política del bucket tampoco contiene principales públicos. Sus cuatro bindings legacy de Storage se reportan como roles/pseudoprincipales asociados al proyecto; no se añadieron como grants del plan.
- Cuentas y credenciales: existen las cinco cuentas de runtime planeadas, incluida la deployer. Las cinco y la cuenta Compute predeterminada tienen cero llaves de usuario administradas. La cuenta Compute predeterminada apareció al habilitar Compute API; no es una de las 41 direcciones Terraform y su política directa no tiene bindings. No se eliminó.
- Secret Manager: los tres secretos existen como metadatos y tienen cero versiones.
- Bucket `espacigo-test-files-447373903153-gcp02`: región `SOUTHAMERICA-WEST1`, UBLA activo, prevención de acceso público `enforced` y versioning activo.
- WIF: pool activo; proveedor OIDC de GitHub Actions limitado a `HernanEspinozaDev/espaciGo` y `refs/heads/main`.
- Sin instancias Cloud SQL ni servicios Cloud Run. DNS no se consultó ni modificó. No se habilitó billing ni se cargaron datos de aplicación.
- Ocho políticas organizacionales efectivas se releen después del apply y permanecen sin cambios. Se conserva el rol Owner y no se relajaron las políticas de organización.
- El root `inventory-apis` y sus APIs `cloudasset.googleapis.com`/`orgpolicy.googleapis.com` permanecen separados. El state bootstrap migrado sigue separado; esta operación no lo migró ni lo editó.

## Delta completo de APIs habilitadas

La comparación usa la lista completa del proyecto antes y después, no solo los servicios declarados en Terraform. El total pasó de 25 a 39; hashes SHA-256 de las capturas: antes `a3e1563a532c4f227e9957b1f2a07e2252de847ec6bd00ab70b0e62a60ba9c14`, después `04342c4532fa6da771a85098a2e9d2fb72e58b4ecc80b04db058b6d929c5d943`. No se deshabilitó ningún API.

Además de los 11 APIs planeados que estaban deshabilitados, el proyecto habilitó `containerregistry.googleapis.com`, `oslogin.googleapis.com` y `pubsub.googleapis.com` como dependencias/efectos colaterales del aprovisionamiento. Se conservaron habilitados y se documentan aquí; no se hizo limpieza ni se atribuyen como recursos Terraform independientes de este plan.

## Alcance pendiente

El toggle de base de datos, servicios y edge permaneció deshabilitado. No se crearon Cloud SQL, Cloud Run, balanceador, Cloud Armor, DNS ni versiones secretas; tampoco se importaron usuarios, espacios, reservas o archivos de la aplicación. GCP-02 sigue incompleto hasta que se revisen y autoricen esos cortes y se verifiquen sus gates, costos y datos de prueba por separado.

## Aclaración de agentes de servicio observados

En la inspección posterior usada para preparar GCP-03 se observaron seis roles de agentes de servicio gestionados por Google, asociados a Artifact Registry, Compute, Compute Instance Group Manager, Container Registry, Pub/Sub y Cloud Run. Son bindings creados por Google al habilitar APIs y no forman parte de las 14 vinculaciones IAM explícitas del plan de Foundation ni de su manifiesto. Se preservan; no se revocaron ni se intentó “limpiar” la política del proyecto.
