# N08-B — retorno propio/ajeno y retry nativos (2026-11-09)

## Baseline y alcance

- Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty preservado.
- Probe durable `frontend/tooling/kyc-native-flow.mjs`, SHA-256 `f9ef7a692949479002c996cd01fd8a3667a886c2a73e95dc699873ee17ba0632`.
- Mismo build N07 aislado `espacigo-kyc-n07-fixed-k653aibr`, origenHTTPS4219, BFF/Next4201, API harness18090 y DB nativa `espacigo_dms_stage_20261020`/OID16388/motor180006. Dos cuentas sintéticas existentes autorizadas; sin cuentas nuevas ni credenciales reales.
- Se conserva el recorrido completo N07 y se agregan subcriterios C02/C07. No se prueba aquí decisión Approved real, error503/FAILED, webhook ni todos los estados de retorno.

## Candidato y revisión parental

`deleg_87f80902` entregó fuente y checks estáticos, sin ejecución. Padre leyó el archivo completo, conservó el relay intacto del POST BFF y corrigió antes del ensayo: rechazo opt-in/argumentos sin stack, captura segura de errores al leer receipts, guard de red a nivel contexto/origen exacto, saneamiento URL verificado tras UI montada y comprobación de todos los campos sintéticos seleccionados al denegar el retorno ajeno. Metadatos privados own0600/parent0700 y política validada mediante preflight nativo previo. No se modificó producto ni gateway.

`node --check`, Prettier y ESLint focalizados PASS. Subprocess sin opt-in y con argumento inesperado bajo opt-in: exit2 antes de leer fixtures.

## Preparación y ejecución

- Preflight READ ONLY nativo PASS.
- Backup nuevo `.local/testing/kyc-native/pre-n08-return-20261109.dump`, 0600, 8,321,941bytes, SHA-256 `fcea9ea7aa46931fb19127525d2fd914562b04e957b6eebc797939ff44a7d2a2`. TOC y consumo completo por pg_restore PASS. No restore.
- Prepare/finalize versionados ejecutados como comandos sudo directos parentales. Orquestador `run_kyc_n07_bff.py return` inicia Go harness y ejecuta `KYC_NATIVE_FLOW=1 node frontend/tooling/kyc-native-flow.mjs` con entorno browser allowlist; stop/cleanup por journal antes de finalizar permisos, incluso ante fallo. La invocación aislada del probe requiere que esos prerrequisitos ya estén preparados.

## Resultado real

**probe_exit=0, harness_exit=0**. `TestNativeKYCAPIWithBFF` PASS11.70s, paquete12.788s con `-race`.

| Subcriterio nuevo | Resultado |
| --- | --- |
| Retorno de A con intento propio y query `status=Approved`/token sintético | GET200 real, no aprobación desde URL |
| Saneamiento de query propia | Vacía al observar despacho GET y tras montar UI |
| Resultado propio visible | «Verificación pendiente», sin badge exacto «Identidad verificada» |
| Retorno de B con intento de A | GET404 real, query limpia, alerta segura «No pudimos consultar el estado» |
| Retry explícito de B | Segundo GET404 real, mantiene error recuperable |
| Privacidad del error de B | Sin badge de aprobación/rechazo y sin nombres/RUT/fecha sintéticos de A |

También volvieron a pasar las aserciones N07 originales: login200/cookies seguras, declaración404→guardar201→readback200→recarga200, consentimiento Didit separado sin llamada previa, intento201/status200, navegación al proveedor falso interceptada sin petición externa, A→B→A con declaración privada y resumen recuperado, logout204/sesión401/cookies ausentes. Cero peticiones externas inesperadas y sin campos sintéticos seleccionados en localStorage/sessionStorage/URL.

No se emitió webhook ni se fabricó una decisión. La única respuesta sustituida es la página de captura de proveedor; el relay BFF devuelve íntegra la respuesta verdadera de Go/DB. La declaración única del ensayo se registró antes de la mutación y se retiró por su journal al terminar.

## Cleanup y readback independiente

- Cleanup del harness y finalizador directo COMMIT.
- Readback READ ONLY posterior: audit DELETE=false, outbox DELETE=false, owner CREATE=false, audit SELECT=false, ledger90.
- Outbox Didit global50, pendientes0; el harness verificó que el total de su baseline pre-test no cambió tras su cleanup. El total global observado en cortes previos fue46; no se interpreta como permiso para borrar/reconciliar filas ajenas ni se atribuye ese cambio entre cortes al ensayo sin prueba. El ámbito propio quedó limpio.
- Preflight fixture posterior PASS (ambas cuentas autorizadas activas y sin artefactos propios), journal ausente, ready ausente, stop propio retirado tras validar metadata, API18090 sin listener.
- Los recursos propios Next4201/proxy4219/fakeprovider19091 continúan retenidos solo para N08; no se tocaron demos ni colas ajenas, no se arrancó worker general.

## Estado

**N08-B completa para retorno propio/ajeno y retry404.** C07 continúa parcial por estados técnicos503/FAILED y otros casos no ejercitados. C02 agrega denegación nativa de intento ajeno al aislamiento de declaración ya demostrado. N08 no se cierra.

Siguiente slice N08-C: auditoría completa Axe/copy/overflow en escritorio y móvil con capturas solo del formulario sintético, sin correo/credenciales de política ni auth screens. Delegación source-only en curso; aún no ejecutada. Sin proveedor externo, personas reales, aprobación legal/humana, main, publicación ni despliegue.
