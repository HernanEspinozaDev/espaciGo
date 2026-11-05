# Pendientes de galería — cola2 y alfa exacto

Fecha local: 2026-11-04 (-03). Continuación autorizada del cierre funcional. Backend base `ae09d5d5b1627bfcba1a4cdfac76853eceed8ab8`; frontend base `bd4b940065037d442f6f15e96a18261695be0b95`. Mismas ramas de tarea y mismo ensayo `127.0.0.1:15434/espacigo_f5_gate_20261101`, sin main/despliegue/F7.

## Cambios verificados

### Cola de dos solicitudes

- `GalleryUploadQueue` separa decisiones de carga de React/transporte. Selección múltiple; máximo2 uploads y un POST multipart por archivo. Conserva File y clave por intención/reintento. Spinner/estado por elemento, sin porcentaje inventado ni filename/IDs expuestos.
- Cancelación queued evita POST y no suma éxito. Cancelación enviada aborta la señal hook→fachada→adaptador→fetch, no declara rollback remoto: tras settle consulta por misma clave. pending/deleting/consulta fallida conserva incertidumbre. failed/not_found ofrece solo retry explícito original; ready necesita readback.
- Consultas manuales deduplicadas, sin consulta competidora mientras el upload está pendiente; fences de item/dispose impiden sobrescrituras tardías. El resumen inicial no anuncia cargas completadas; solo ready cuenta como éxito. Mutaciones/CAS y publicación se deshabilitan mientras hay cargas sin resolver.
- Capacidad frontend cuenta selección contra fotos existentes y bloquea nuevo lote mientras el anterior no está resuelto; Go mantiene autoridad transaccional del cupo30.
- Dos E2E mock ejercitan4 archivos,2uploads retenidos, cancelación queued, despacho del tercero solo al liberar slot,3POST/3ready/1cancelled, resumen3, rechazo de selección28 sobre3 existentes sin POST; y202→lookup pending/not_found→retry explícito con misma clave y bytes. No se atribuye integración real a los mocks.

### Codec: raíz del exceso de peso con transparencia

La dependencia fijada `github.com/deepteams/webp v1.2.8` interpreta explícitamente los ceros de AlphaCompression y AlphaQuality. El literal anterior `{Quality:80, Method:4}` desactivaba compresión alfa y cuantizaba alfa a calidad0. No equivalía a todos los defaults de la biblioteca.

RED ejecutado antes de tocar producción:

- `TestEncodeVariantPreservesAlphaExactly`: alfa7 se decodificaba0.
- Caso constante1600×1200 con alfa96: display1923302bytes, sobre meta614400.

Corrección mínima: mantener Quality80/Method4 y fijar AlphaCompression1, AlphaFiltering1 y AlphaQuality100. No bajar calidad RGB, tamaño máximo ni ampliar límites. Los bytes opacos se mantienen exactamente iguales a la configuración anterior (regresión ejecutable); alfa conserva sus valores exactos por píxel. No se recodifican objetos históricos: aplica a nuevas cargas/reemplazos.

Corpus sintético después de corregir:

| Escenario | Display bytes | Thumbnail bytes | Metas614400/102400 |
|---|---:|---:|---|
| interior_edges |18042|4588|Ambas cumplen |
| exterior_texture |1044618|98496|Display excede; thumbnail cumple |
| portrait_orientation |19148|4972|Ambas cumplen |
| transparent_plan |14862|4142|Ambas cumplen; antes1934686/176864 |
| small_image |846|846|Ambas cumplen |
| webp_texture |1190300|91266|Display excede; thumbnail cumple |

Las12 variantes mantienen decodificación WebP, dimensiones/orientación, no ampliación y ausencia de metadatos/animación del corpus. Dos display de ruido pseudoaleatorio siguen excediendo la meta; no representan un corpus fotográfico ni justifican degradar calidad arbitrariamente.

Experimento opt-in `ESPACIGO_MEDIA_EFFORT_EXPERIMENT=1 go test -v -count=1 ./internal/media -run '^TestCodecEffortExperiment$'`: seis escenarios×dos tamaños, variantes q80m4 anterior, q80m4 alfaexacto, q80m6 y losslessm6. Method6 no redujo las muestras respecto a m4; lossless puede crecer mucho en ruido. No se adoptaron esos cambios de esfuerzo/modo por defecto. Para transparencia, alfaexacto dejó MAE alfa0; thumbnail anterior3.6806→0, con mismo error RGB2.2205. Display RGB1.5642 permanece igual. Log reproducible `gallery-codec-effort-final.log`.

## Gates del padre

| Gate | Resultado real |
|---|---|
| Tooling global |110PASS/0FAIL/0SKIP; incluye8 tests del motor de cola y propagación de señal |
| TypeScript/ESLint/Prettier/build producción |PASS; un warning no bloqueante por img privada,0errores |
| Contrato generado/inventario |PASS,187 operaciones; sin cambios de API en este corte |
| Playwright global |71PASS/49SKIP/0FAIL, `gallery-queue-global-final.log` |
| Nativo HTTPS con Go/DB/storage reales |1PASS/0FAIL/0SKIP, `gallery-queue-native-acceptance.log`, fixture NUEVO20261104-queue3 |
| Lectura/AXE posteriores |Activa/30fotos/una portada; imágenes decodificadas; panel real desktop1365×900/mobile390×844 sin violaciones ni overflow |
| Go completo/vet sin DSN |PASS; logs `gallery-queue-go-final.log` y `gallery-queue-vet-final.log` |
| Codec regresiones y experimento opt-in |PASS, alfa exacto y bytes opacos invariantes |
| ACL read-only opt-in separado |FAIL por fingerprint PUBLIC/default OID-dependent; no se oculta entre los skips de Go global |

El nativo ahora envía tres archivos juntos desde UI, observa máximo2 POST en vuelo y tres201. Uno es PNG RGBA con alfa96; canvas sobre la variante privada real comprueba alfas `[96,255,255]`. Luego conserva todo el recorrido anterior: edición/reload, portada/orden/reemplazo fallido conserva DTO anterior/reemplazo exitoso/retirada,30/31,AXE,publicación200→activa y otra cuenta403/logout204.

Fallos diagnósticos preservados: E2E mock inicial1FAIL por selector alert ambiguo con anunciador Next (no fallo de cuota); acotado al region de cola y suite finalPASS. El primer probe de alfa trató de vincular el archivo mediante postData multipart que Playwright no expuso como se esperaba; falló sin afirmar codec defectuoso. Se sustituyó por comprobación directa del canal alfa de las tres variantes reales; native finalPASS. Nuevos tests de consulta deduplicada comenzaron RED y quedaron GREEN tras corregir la consulta pública. La revisión fuente inicial señaló fence/summary/medición; correcciones revisadas y reejecutadas.

Re-review GPT-6 Luna medium `local-9891181c28fefdecef72280236659d30d52299beeb4a0cd3821d1f35d3947089`: PASS sin hallazgos residuales, source-only. Conteos/servicios/ejecución verificados separadamente por padre. Delegación inicial cola6/6 fue ampliada por padre; no confundirla con el total final.

## ACL: diagnóstico cerrado, aceptación aún bloqueada

Se reejecutó `TestVerifyRuntimeACLReadbackF5Gate` con el passfile administrativo documentado, sin contraseñas en URI y en transacción READ ONLY, contra el target exacto. Falla en baseline PUBLIC/default; ningún SQL de cambio/grant/migración.

Revisión independiente source-only `local-e3a87830e4fd2c654e6d4ff676587f4500588aacdb19be5ae5370c1fe99d82aa`: serializar classid/objid/grantor OID impide estabilidad tras restore. La igualdad de pg_dump schema/owners/ACL prueba el alcance archivado, no cubre por sí sola todas las ACL de sistema/PUBLIC que incluye el gate. No se agregó tercer hash observado ni se sustituyó el gate por el checker schema.

Para resolver correctamente hace falta una referencia semántica independiente, íntegra y con cobertura PostgreSQL18.6 completa, no derivada del target bajo prueba. La normalización deberá conservar clase/objeto/esquema/firma/columna/rol/grantor/privilegio/grant option, ownership, default ACL vacío e inventario de catálogos, con negativos de grants añadidos/faltantes/elevación/cambio de servidor/base. Preparar una instancia de referencia restaurada o ampliar la política requiere una autorización específica posterior; no se creó otra base/instancia ni se tocaron las ACL existentes.

## Demo y respaldo

Mismo acceso `https://127.0.0.1:4203/anfitrion/espacios`, fixture20261104-queue3 editado activa30fotos. Ruta precisa en `.local/testing/space-gallery-demo/review-target.json`; inventario sintético existente sin nuevas cuentas ni cambios de contraseña. API worker-free recompilada con el fix alfa y frontend standalone del build final, servicios propios retenidos para revisión del usuario. Mantener PostgreSQL tmpfs y datos ajenos.

Ledger read-only83/83 nombres/checksums idénticos. Backup fuera de tmpfs,0600, TOC/decodificación completa pg_restore PASS:

- `gallery-accepted-20261105T013910Z.dump`,733524bytes, SHA256 `be93c2b379087e6e37a0842484ffefd707be2c388689de9ab0c30dc84d2bd82e`.
- `gallery-storage-accepted-20261105T013910Z.tar.gz`,139035bytes/1062entradas, lectura integralPASS, SHA256 `e3b6b9b21770e72e06bd3db7e47e154f33f67ec29e423d7391c3003783e61a05`.

Ambos en `/home/nandev/.local/share/espacigo-f5-backups/`. Timestamp UTC05 corresponde a noche local04. No se afirma restauración del backup nuevo.

## Pendientes que no se convierten en implementados

Quedan las dos metas display de textura y decisión de corpus/piso visual, referencia completa del gateACL, republicación/desocupación, lectura documental por revisores y contratación/cobros por sus contratos separados. Sin main, PR, F7/cloud, despliegue ni aceptación humana inventada. Los SHA publicados de cada rama se verifican en el recibo final, sin hash autorreferencial en esta evidencia.
