# Revisión independiente final y adjudicación

## Resultados recibidos

La revisión independiente acotada posterior de los recibos de fallo, stub legacy y opt-in público devolvió **sin bloqueadores concretos**. El revisor repitió 16 pruebas de preparación con mocks, seis del launcher y dos de CLI/import legacy, más pruebas Go de guards. El padre había ejecutado esos mismos checks por separado. Esto cierra los hallazgos operativos adjudicados abajo, sin convertir sus unitarias en pruebas de recursos reales. La publicación local real y su auditoría se acreditan por separado en `wizard-native-public-positive.md`.

Tres revisores independientes inspeccionaron cambios rastreados y archivos nuevos del worktree de mejoras del wizard. Son revisiones de código, no sustitutos de las pruebas ejecutadas por el padre.

| Área | Veredicto recibido | Alcance/límite |
| --- | --- | --- |
| Frontend wizard, DPA y calendario | Sin bloqueadores concretos | Sólo revisión estática; no repitió los recorridos |
| Backend público, perfiles, migrador y ACL | Sin bloqueadores concretos | También ejecutó paquetes Go afectados; no repitió la prueba PostgreSQL opt-in |
| Preparación y registro de ensayo | Objeciones de fallo parcial, segundo actor y duplicación legacy | Adjudicación abajo; no ejecutó recursos ni modificó archivos |

## Adjudicación del padre contra las fuentes

### Preparación parcial sin recibo de fallo — aceptado con corrección limitada

`create_empty` conserva recursos y material privado al fallar; eso evita destruir datos ante una respuesta ambigua. La retención fail-closed es intencional y no se sustituirá por eliminación automática. Sí falta un recibo durable de fallo que identifique etapa y necesidad de reconciliación sin afirmar que se limpiaron recursos.

Se asignó añadir ese registro y negativos con runner simulado, manteniendo credenciales/backup/recursos intactos. No se ejecutará de nuevo la preparación real. La corrección sigue pendiente de revisión y prueba al emitir esta adjudicación; no se considera resuelta por asignarla.

La primera entrega agregó el recibo pero no los negativos pedidos. El padre detectó además que una nueva constante de ruta quedaba ligada al directorio real y que una prueba con runner simulado había escrito allí un journal mock. Se comprobó su clase/etapa y se conservó en cuarentena privada en `/home/nandev/.hermes/cache/scratch/wizard-rejected-unit-failure-receipt/`; no es evidencia de preparación real. No se alteraron recursos, base ni credenciales. Se devolvió la corrección para resolver la ruta desde la raíz inyectada y añadir guard de escrituras fuera del directorio temporal, más fallos por etapa. No repetir esa suite antes de corregir su aislamiento.

La corrección posterior eliminó la ruta global ligada al checkout real. El padre ejecutó `python -m unittest scripts/test_wizard_improvements_local.py -v`: **16 PASS**. Incluye inyección en seis etapas (segunda credencial, volumen, red, inicio de contenedor, readiness y verificación de backup), retención sin eliminación/replay y conservación de la excepción original si falla el journal. El postcheck independiente confirmó ausencia de un journal en la ruta real del ensayo. Sólo se usaron fixtures temporales y runner simulado; no se ejecutó preparación real. Se solicitó una revisión independiente acotada de estas correcciones y del nuevo opt-in público; su veredicto sigue pendiente.

### Segundo actor — limitación operativa, no permiso para ampliar altas

El plan y la autorización establecen **como máximo dos cuentas**, no la obligación de crear dos ni de proporcionar un aprovisionador multiusuario. Una cuenta existente basta para los tres borradores y sus recorridos. El registrador actual se detiene si hay inventario o recibo, evitando nuevas altas ante estado previo. Se conserva esa política conservadora; no se retirará el guard para satisfacer un número máximo ni se registrará otro actor.

Se documenta que ese entrypoint sirve para la primera cuenta nueva con estado vacío y que una segunda cuenta requiere un procedimiento específico de reconciliación/autorización. No presentar como implementada una ruta automática para dos actores. Si un corte posterior exige otro propietario, ese flujo deberá resolverse antes de su prueba.

### Registro duplicado bajo frontend/scripts — aceptado

La copia legacy consultaba otra forma de política y no era el entrypoint canónico del ensayo. Ya se convirtió en una entrada inactiva que señala `scripts/wizard-improvements-register-account.py`, sin redirección automática, lectura de secretos ni nuevas altas. El padre leyó el stub y ejecutó directamente sus dos pruebas de CLI e importación: **2 PASS**, sin instalar pytest. Se conservó el archivo, sin eliminación por patrón.

## Scan y pruebas del padre

El scan de literales produjo tres candidatos; el padre comprobó que eran una fixture sintética, una aserción contra secretos en argumentos y un prefijo de complejidad concatenado con generación criptográfica aleatoria. No se identificaron credenciales reales en esos candidatos. Este resultado acotado no es una garantía universal de ausencia de secretos.

Los resultados de build, browser, Axe, CAS, unitarias y Go se registran en `wizard-final-local-gates.md`. La revisión no autoriza publicar ni elimina el gate HTTP público positivo pendiente.

## Diagnóstico preciso del gate público

El perfil actual del ensayo es `local-space-wizard`. `localPublicCatalogEnabled` reconoce sólo el perfil worker-free F5 y su destino histórico; el launcher wizard no habilita ese contrato HTTP. No se cambiará de destino ni se activará una bandera que el guard actual rechaza.

Se revisó la publicación normal existente: `spaces.Service.SetPublicationState*` separa publicación de identidad contractual, y el repositorio verifica propietario activo, revisión, timezone IANA, estado de fixture y conflictos de devolución. No es correcto afirmar que toda publicación exige KYC o fotos por inferencia: deben respetarse las condiciones reales, no inventarse ni saltarse.

El usuario eligió después **«Autorizar ese ensayo público local por el flujo normal»** para habilitar el contrato exclusivamente en este perfil/destino y activar el borrador sintético horario mediante la operación normal con CAS y auditoría. Se implementó el opt-in con guard exacto, seis pruebas aisladas del launcher y pruebas Go repetidas por el padre. La API reconstruida respondió 200 al catálogo de categorías y 404 al borrador antes de publicar. La prueba de publicación sigue en revisión antes de su única mutación. No se autorizaron SQL para forzar estado, bypass de elegibilidad, nuevas cuentas, GCP ni navegación online.
