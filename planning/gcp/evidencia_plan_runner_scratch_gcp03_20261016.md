# GCP-03: plan aislado del runner scratch

Fecha: 2026-10-16 (UTC). Proyecto explícito `espacigo-testprod` (`447373903153`), región `southamerica-west1`, raíz independiente `infra/terraform/environments/testprod/gcp03-scratch` y prefijo separado `espacigo/testprod/gcp03-scratch`.

## Plan privado generado

- Terraform 1.16.6, Google provider 8.7.0, con refresh contra el proyecto indicado.
- Plan binario privado: `.local/planning/gcp/gcp-03-scratch-20261016/gcp03-scratch.tfplan`; SHA-256 `9f077bfc01c372003fb5236a258685637d689a3ac2095e4a134aa38ff7b3a932`.
- Directorio privado modo 0700 y archivos modo 0600. Plan, JSON/texto, backend y tfvars se excluyen de Git.
- Resultado exacto: **8 crear, 0 cambiar, 0 destruir**. Acciones: instancia Cloud SQL PostgreSQL 18 privada `espacigo-gcp03-pilot01-pg`; base vacía `espacigo_scratch`; service account independiente; grant `roles/cloudsql.client` para esa cuenta; secreto temporal scratch solo como metadata sin versión; `secretAccessor` sobre ese secreto y nada más para esa cuenta; Artifact Registry temporal; y lectura del repositorio para el Cloud Run service agent.
- `enable_job=false`: el plan no incluye un Job invocable porque aún no existe imagen subida/validada ni versión temporal de password. El root sí declara el Job para un plan futuro, gated por digest de este repositorio, confirmación de secret version, identidad operadora y un `run_id` exacto.
- La inicialización/plan remoto creó el objeto backend nuevo `espacigo/testprod/gcp03-scratch/default.tfstate` con cero recursos; quedó serial `1`, lineage `11eb96d4-a301-9029-11d1-69e1a00f6302`, generation `1792130482170124`. Se conservó y descargó solo a `.local/`; SHA-256 `5ad217323cc279b6678a8e22efe95841a0e6915a9ab1894e428c18d7a6694e95`. No se seleccionó ni modificó el state del root persistente.

## Verificaciones de solo lectura y límites

- Proyecto activo: `espacigo-testprod`, ACTIVE, número `447373903153`.
- VPC `espacigo-testprod` y subnet `espacigo-testprod-serverless`, `southamerica-west1`, `10.80.0.0/24`, `private_ip_google_access=true`.
- La única instancia Cloud SQL observada en el proyecto fue la persistente `espacigo-testprod-pg`; el nombre scratch no estaba ocupado. Tampoco se encontraron cuentas, secreto, repositorio o Job con el identificador `pilot01`.
- Terraform leyó proyecto, VPC y subnet durante `plan`. No se ejecutó `apply`, SQL, conexión, migración, creación de usuario/version de secreto, upload de imagen, restore o importación. El state de la raíz persistente no fue seleccionado ni modificado.
- El plan contempla un Cloud SQL `db-custom-N4-2-4096` zonal con 20 GiB PD-SSD sin backup/PITR, sin IPv4 pública, `ENCRYPTED_ONLY`, connector requerido y ambas protecciones de borrado activas. El tier debe revalidarse en el plan inmediatamente anterior a cualquier aplicación futura.

El Cloud Run Job regional sigue sin tarifa validada para Santiago hasta seleccionar su SKU en la calculadora oficial; los montos anteriores son referencias estimadas, no precio facturado. Esta evidencia es de un plan remoto, no de infraestructura creada ni de compatibilidad SQL demostrada.
