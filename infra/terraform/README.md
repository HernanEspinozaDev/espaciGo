# Terraform para testprod de EspaciGo

La configuración es local y no se ha aplicado. La raíz `bootstrap` crea tres buckets de state separados (`bootstrap`, `gcp`, `cloudflare`); `gcp` y `cloudflare` usan backends GCS distintos. Cada raíz cloud requiere un `backend.hcl` privado fuera de Git. Ninguna raíz importa automáticamente state, crea el proyecto, cambia billing ni lee el state de la otra.

## Topología de entrada e IAP

Un Application Load Balancer externo global con una IP sirve ambos hosts HTTPS y enruta cada host a su Cloud Run SSR mediante serverless NEG. El backend público tiene Cloud Armor. El servicio Cloud Run privado habilita IAP **directamente en Cloud Run** (`iap_enabled = true`); el backend service del balanceador no tiene IAP. Cloud Run IAP protege también el acceso que llega por el balanceador; `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER` bloquea el acceso externo directo por `run.app`. Ambos SSR usan Direct VPC egress con `ALL_TRAFFIC`; la subred habilita Private Google Access, para que la llamada al API privado `run.app` recorra la VPC y se clasifique como interna. Esa salida no incluye Cloud NAT: cualquier requisito futuro hacia destinos públicos no-google exige diseñar y cotizar NAT/rutas antes de habilitarlo. La audiencia IAP es el servicio Cloud Run:

```text
/projects/PROJECT_NUMBER/locations/REGION/services/SERVICE_NAME
```

No usar `/projects/PROJECT_NUMBER/global/backendServices/SERVICE_ID` en `IAP_JWT_AUDIENCE`; esa audiencia solo correspondería si IAP se activara en el backend service. No combinar ambas modalidades. La salida `private_iap_audience` está diseñada para coincidir con la validación actual en Express. Acceso de evaluadores queda vacío por defecto: se agregan principales IAP explícitos tras resolver OAuth/acceso externo manualmente.

## Separación y bootstrap del state

1. Revisar el inventario del proyecto existente. El primer `apply` de `bootstrap` usa state local bajo permisos 0600 solo después de confirmar proyecto, nombres únicos y tres principales operadores. No usar un bucket previo/ajeno ni habilitar locking en un state compartido.
2. Bootstrap crea tres buckets GCS independientes, UBLA, prevención de acceso público, versionado y `prevent_destroy`; aplica IAM de objetos al bucket específico. Outputs y configuración de backend se copian privadamente.
3. Migrar el state bootstrap a su bucket dedicado en una segunda inicialización revisada. GCP y Cloudflare reciben sus propios `backend.hcl` privados fuera del repositorio. Comparar el plan guardado y revisar cada destino; no copiar state a Cloudflare ni conectar raíces con `terraform_remote_state`.
4. Para Cloudflare, copiar manualmente solo la IP pública de GCP a variables privadas. No proporcionar state GCP al operador/token Cloudflare.

El primer state bootstrap es local y temporal; una vez creados los buckets, migrarlo explícitamente al bucket `bootstrap` con `terraform init -migrate-state`, verificación de identidad y revisión del plan. Esta raíz no contiene un backend remoto implícito.

## Gates de aplicación

Los toggles permanecen falsos por defecto (`create_state_bucket`, `enable_foundation`, `enable_database`, `enable_services`, `enable_edge`, `manage_dns_records`). `enable_foundation` exige proyecto/número existentes y confirmaciones de inventario aislado, política organizacional y estimación completa; `manage_dns_records` exige revisar la zona/registros y una IPv4 válida del plan GCP. Aplicar requiere plan revisado y gates humanos de facturación y OAuth/IAP. `enable_services` exige confirmación explícita de que GCP-04 resolvió replay compartido y leases; hoy la validación debe impedir activarlo.

Terraform crea el contenedor de Secret Manager, nunca el valor. Antes de desplegar API deben cargarse por canal privado y validar las versiones activas de `database-password`, `contract-encryption-key` y `payment-webhook-secret`; los datos no entran en variables, planes ni state. También falta que GCP-04 cree/verifique el usuario PostgreSQL de runtime y grants. El API fija `cpu_idle = false` y mínimo una instancia mientras esté habilitado, por lo que CPU/memoria se facturan continuamente durante su vida; incluirlo explícitamente en la calculadora regional. Mailpit queda fuera de esta raíz hasta definir imagen y operación de VM/disco en el corte de operación correspondiente; `mailpit_private_address` debe venir de una instalación privada aprobada.

Cloud Armor limita el listado de catálogo a un bucket de 60 solicitudes/minuto/IP y otro bucket a 120/minuto/IP para otras rutas públicas SSR/API/imagen/sitemap/readiness. Las reglas se evalúan por prioridad: una solicitud al catálogo que coincide con prioridad 100 incrementa ese contador, no el contador independiente de prioridad 200; no se suman ni comparten contadores. Los valores son configurables por variables dentro de rangos y deben comprobarse/tunearse con métricas y costos reales. La política se adjunta solo al backend público.

Cloudflare solo declara dos registros A DNS-only (`proxied = false`) en la zona existente. No gestiona zone settings, nameservers, Workers, Pages, túneles ni registros no listados. Antes de habilitarlos hay que leer los registros existentes; si alguno existe, importarlo a este state tras confirmar propiedad y contenido, nunca duplicarlo.

## Validaciones locales

Con Terraform instalado y acceso al registry para descargar providers:

```sh
terraform fmt -check -recursive infra/terraform
```

Inicializar/validar/planificar cada raíz con backend desactivado desde su directorio. El plan por defecto es intencionalmente vacío y solo demuestra que los gates están cerrados; no es inventario remoto ni plan de despliegue:

```sh
terraform init -backend=false -input=false
terraform validate
```

`validate` usa los schemas fijados sin inicializar backend remoto. `terraform test` usa `mock_provider` para probar planes con foundation, database, services, edge y DNS activos sin credenciales ni llamadas cloud; comprueba el grafo/configuración, no un plan real, inventario ni integración GCP. Un `plan` remoto real requiere backend privado, identidad cloud, inventario y variables aprobadas, y no se ejecuta en este corte. El plan vacío solo demuestra gates apagados. Ningún comando ejecutado en este corte hizo `apply`, `import`, login, habilitación de APIs, llamadas a Cloudflare, IAM o DNS.
