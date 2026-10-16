# GCP-03: evidencia de aplicación de controles Cloud SQL

Fecha: 2026-10-16. Proyecto objetivo: `espacigo-testprod`. Instancia: `espacigo-testprod-pg`, región `southamerica-west1`.

## Autorización y plan aplicado

PR #339 fue aprobado y fusionado por `HernanEspinozaDev` en `62f8dc16dbc724bb48f2045a9f5dbcb2f3ca783b`. El PR #337 relacionado fue fusionado por la misma cuenta en `5b04606044c331e698518cbefcee4fe87f6385b2`. El plan binario aprobado y aplicado fue `cloudsql-security.tfplan`, SHA-256 `4c4c90120115a93d7fec83ad316d62e8844ec7036d9595cdc3cd342e96eb26c3`. Los artefactos privados están bajo `.local/planning/gcp/gcp-03-cloudsql-security-20261016/`, con directorio 0700 y archivos 0600; no se versionan.

El preflight confirmó el proyecto `espacigo-testprod` activo, la instancia `RUNNABLE`, su IP privada `10.189.0.3` y ninguna IPv4 pública. El estado Terraform conservó lineage `95bafa42-5d1a-bef2-909b-7ce4b6a92096`, serial 10 antes del cambio. Se aplicó el binario revisado (Terraform 1.16.6, Google provider 8.7.0): **0 añadidos, 1 actualización in-place, 0 destruidos**, únicamente `google_sql_database_instance.postgres[0]`.

## Resultado observado

| Control | Antes | Después |
| --- | --- | --- |
| Protección nativa contra borrado | `false` | `true` |
| `connector_enforcement` | `NOT_REQUIRED` | `REQUIRED` |
| `ssl_mode` | `ALLOW_UNENCRYPTED_AND_ENCRYPTED` | `ENCRYPTED_ONLY` |
| Estado de instancia | `RUNNABLE` | `RUNNABLE` |
| IPv4 pública | deshabilitada | deshabilitada |
| Estado Terraform | serial 10 | serial 11, mismo lineage |

La verificación posterior confirmó esos valores por API. `terraform plan -detailed-exitcode` posterior finalizó con código 0: cero cambios planificados y cero entradas `resource_drift`.

La comparación de refresh previa al apply mostró siete normalizaciones de representación/etag (mapas vacíos, alias/SAN vacíos y etags IAM). Las siete aparecían de igual forma en el plan aprobado y en el refresh nuevo; no añadieron acciones ni alteraron el plan aplicado. El refresh posterior quedó sin drift.

## Límites y evidencia privada

No se ejecutó SQL, no se establecieron conexiones de base de datos, no se crearon usuarios ni versiones de secretos, y no se ejecutaron migraciones, dumps, restores o importaciones. Tampoco se desplegaron Cloud Run, DNS, buckets de staging ni otros recursos. No se modificaron Paid mode ni facturación. Esta evidencia registra controles de infraestructura aplicados; no acredita compatibilidad de migraciones ni preparación de datos.

Los JSON de API/estado, plan, salida detallada y logs quedan privados bajo `.local/planning/gcp/gcp-03-cloudsql-security-20261016/`; no incluirlos en commits, PR, capturas o logs compartidos.
