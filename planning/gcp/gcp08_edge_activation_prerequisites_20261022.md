# GCP-08 — preparación de edge y DNS (2026-10-22)

## Estado comprobado

- El reviewer reportó el build Cloud Build `dd3c588f-e3ad-4ddc-9b61-a4fa85747253` exitoso para `main` `cfdf44f8f63261a8b4273960daa40735a6b7098d`, y las revisiones Ready de API, web público y web privado.
- La lectura segura de la revisión privada confirmó `espacigo-test-private-00001-nlp`, imagen web por digest `sha256:6fc2bfc70046a837b4c10f732bb9171b305a5548263c8025c5540ef12b399b3d`, subred `espacigo-testprod-serverless` y egress `private-ranges-only`.
- No hay reglas de forwarding, backend services, URL map, IP global ni certificado; tampoco hay registros A resolubles para los dos hostnames elegidos. La raíz Cloudflare conserva `zone_id=""`, `manage_dns_records=false` y `confirm_dns_inventory=false`; el state/prueba local no contiene un ID o snapshot real de la zona ni autorización para modificarla.

## Preflight real del endpoint de claves IAP

El repositorio anterior tenía solo una justificación de topología, sin prueba de conectividad desde Cloud Run. Para cerrar esa brecha se ejecutó un Job temporal con la misma imagen, VPC, subred y `private-ranges-only` de la revisión privada. El proceso hizo un GET sin credenciales a `https://www.gstatic.com/iap/verify/public_key`, validó HTTP 200 y la forma del key set en memoria y registró únicamente `status=200 valid_keyset=true`. No se guardó ni emitió el cuerpo ni las claves. El Job se eliminó después de terminar. Evidencia sanitizada: [gcp08-iap-key-egress-preflight-20261022.json](evidence/gcp08-iap-key-egress-preflight-20261022.json).

Este preflight comprueba la ruta actualmente desplegada. Con `private-ranges-only`, solo los destinos privados usan Direct VPC; el host público `www.gstatic.com` sale por el egress administrado de Cloud Run. Aunque la subred tiene Private Google Access y una ruta por defecto, esta ejecución no prueba una ruta `ALL_TRAFFIC` a través de PGA. Si el egress cambia a `all-traffic`, se requiere un preflight PGA independiente antes de confirmar el gate. [Cloud Run Direct VPC egress](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc) documenta el alcance de ambos modos; [Private Google Access](https://docs.cloud.google.com/vpc/docs/configure-private-google-access) incluye `gstatic.com` como dominio elegible.

## Gate de edge

`enable_edge` sigue siendo falso por defecto. El gate ahora requiere `enable_foundation`, que conserva las confirmaciones de inventario y costo, la confirmación de los servicios Cloud Run externos y el preflight IAP para la configuración vigente. Los tests negativos impiden habilitarlo sin Foundation o sin evidencia de egress. Al activarlo en un plan revisado, Terraform crea NEG serverless, backends, Cloud Armor para el backend público, IP global, certificado administrado, URL map, proxy HTTPS y forwarding rule. Esto no abre el ingress directo de Cloud Run ni cambia IAP.

El nombre de entrada `confirm_pga_iap_key_preflight` se conserva para no romper tfvars protegidos existentes, pero su descripción aclara que, con la configuración actual `private-ranges-only`, el preflight confirma el endpoint desde la ruta pública real y no afirma uso de PGA.

## DNS y límites

DNS permanece fuera de este corte: no hay inventario autorizado de zona/registros ni `zone_id` revisado. No se agregó ningún registro de Cloudflare. Para gestionarlos después, el owner debe revisar la zona exacta y los registros existentes, confirmar autorización, y aportar el IPv4 del plan de edge revisado; entonces `manage_dns_records` podrá habilitarse en la raíz Cloudflare separada, solo con registros A DNS-only para los dos hostnames.

No se ejecutó Terraform plan/apply remoto ni se cambió Cloud Run, Cloud SQL, Secret Manager, IAM, DNS, forwarding, certificado o Cloud Armor. El preflight temporal fue la única mutación y su Job se retiró. El siguiente paso es que el revisor evalúe y ejecute el plan exacto de edge; Cloudflare requiere además inventario/autorización del owner.
