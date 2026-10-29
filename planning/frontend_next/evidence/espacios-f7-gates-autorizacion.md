# F7 — Compuerta de autorización cloud y release

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, rama `main`, HEAD base `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`.  
**Alcance:** consulta remota read-only de SHA/build/revisiones Cloud Run, metadata Cloud SQL e HTTP público. No se creó backup, no se leyó Cloud SQL, no hubo migración, Terraform/IAM, build, despliegue ni cambios de tráfico.

## Hallazgos de la inspección remota read-only

- `git ls-remote origin refs/heads/main` devolvió `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`, igual al HEAD local base.
- El trigger `espacigo-main-rollout` existe en `espacigo-testprod/southamerica-west1`, conectado a `^main$` y `cloudbuild.yaml`, con la identidad Cloud Build `espacigo-cloud-build-deployer`.
- El build padre `9b426354-629a-4c1e-acb2-04a04edddae3` terminó `SUCCESS` el 27 de octubre de 2026 06:03:43Z para ese SHA, scope `full`; build hijo `ef21332d-ef89-4529-a0ab-4031591e7eed` también `SUCCESS` y publicó imágenes API/Web.
- Las revisiones API, web pública y web privada etiquetan ese SHA y build; sus condiciones `Ready=True`, y los servicios reportan 100% de tráfico a esas revisiones. Digests API `sha256:1b55e2c785a8e1924b315c216fe8da6ea182ab2f01350ece99a178c86eec923a` y web `sha256:6910f1b723887cfa85cbc92f0a052f51973d06ae3ed903d1d9ef230958b1e4ed`.
- La raíz pública `https://espacigo.testingpage.store/` respondió HTTP 200. No se probó el recorrido privado IAP/autenticado ni flujos de cuenta/media; el HTTP 200 no valida esas funciones.
- Metadata de `espacigo-testprod-db`: `RUNNABLE`, región `southamerica-west1`, IPv4 pública deshabilitada, `requireSsl=false`, `sslMode=ALLOW_UNENCRYPTED_AND_ENCRYPTED`. Solo se consultó metadata; no se abrió conexión PostgreSQL ni se inspeccionaron ledger/datos.
- Estas observaciones corresponden al estado consultado el 28 de octubre de 2026 y al último full rollout reportado el 27 de octubre. El repo actual contiene cambios sin commit posteriores al SHA de despliegue; por tanto, ese release no contiene el corte F4 que se está desarrollando ahora.

## Acciones remotas fuera del alcance autorizado

- **Cloud SQL:** lectura del ledger/perfiles/permisos, backup recuperable, captura de evidencia y aplicación de migraciones. La inspección presente solo obtuvo metadata de instancia; no se leyó la base.
- **Terraform/IAM:** `plan` remoto y cualquier cambio de bucket, retención o permisos. No se autoriza bucket público, lifecycle, CORS ni grants nuevos.
- **Release:** no iniciar otro Cloud Build, migrar destino remoto, desplegar, cambiar tráfico/IAP ni validar servicios autenticados bajo la instrucción general de completar el plan. El deployment actual no prueba los cambios locales no publicados; F4/6 aún requieren gates.

## Preparación local que sí puede continuar

Se permiten gates locales con fixtures sintéticos, revisión de migraciones ya aplicadas al target local autorizado y artefactos de pre/postcheck sin conectarse a GCP. Esa preparación no equivale a backup remoto, migración, despliegue o verificación funcional cloud.

## Salida veraz

F7 tiene **inspección read-only parcial** completada; la entrega de F7 permanece **bloqueada por autorización** para backup/migración/release. No proponer ni iniciar ese release antes de cerrar F6 y obtener autorizaciones concretas para operaciones remotas. La inspección no modifica los recursos.
