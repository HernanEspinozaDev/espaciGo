# Cierre funcional local — wizard y galería, 2026-11-04 (-03)

Base frontend: `cc2fdc39670477b5c051ea74c853bb905604fb4d`, rama `codex/frontend/fix/local-close-02-f6-native-e2e`. Backend base y API compilada: `93b59c8352960d900b4e62bb42bced9e05dcad8c`, rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`. El commit de esta entrega registra las fuentes probadas; SHA final remoto en recibo, sin hash autorreferencial.

## Alcance entregado

- Comuna explícita por código de cinco dígitos coherente con región; el catálogo no está disponible en el runtime elegido. Se retira promesa/header de idempotencia de POST spaces.
- Galería privada real, máximo30, ETag fuerte/revisión seleccionada, portada/orden/reemplazo/retirada y variantes WebP autenticadas. Sin panel sintético desconectado.
- Sesión BFF Secure/HttpOnly y restauración; Bearer opaco solo servidor. Compatibilidad interna del cliente no expone tokens al navegador.
- Pending202 no es éxito: lookup por misma clave y readback; sin replay automático, retry explícito conserva File/key/photo/ETag. 422 validation_error de codec es rechazo definitivo y mantiene la foto anterior. Se bloquean acciones competidoras inciertas.
- Fences de generación para galería y respuesta tardía tras clear/selección/desmontaje. Sin portada/orden/retirada optimistas antes de confirmar servidor. Conservación de snapshot ante fallo de readback.

## Matriz de pruebas realmente ejecutadas por el padre

| Gate | Resultado |
|---|---|
| Tooling `node --test tooling/*.test.mjs` |100PASS/0FAIL/0SKIP |
| TypeScript, ESLint, Prettier global, build producción |PASS; ESLint dos warnings no bloqueantes en galería (ref y img privada),0errores |
| `node tooling/api-check.mjs` |PASS,187 operaciones, tipos/allowlist/inventario actuales |
| Playwright global controlado |69PASS/49SKIP/0FAIL, log `gallery-controlled-final3.log` |
| Playwright nativo HTTPS completo |1PASS/0FAIL/0SKIP, log `gallery-native-acceptance.log` |
| Lectura independiente posterior y AXE galería real |Activa,30 fotos, una portada; todas decodificadas. Desktop1365×900/mobile390×844 sin violaciones del panel ni overflow |
| Go completo/vet del backend sin DSN; checker schema |PASS; checker12/12. No se atribuyen tests documentales live no repetidos |
| Revisión fuente GPT-6 Luna medium |PASS sin hallazgos, sesión `local-cf665180159ae72e91723d78542191855059491ed487746a5b7e1386935e1e6a`; ejecución verificada aparte por padre |

### Corrida nativa final reproducible

Desde `frontend/`, usando solo el ensayo sintético autorizado existente:

```sh
PHASE_SPACE_GALLERY_NATIVE_E2E=1 \
PHASE_SPACE_GALLERY_FIXTURE_SUFFIX=20261104-end2204 \
PHASE6_PRIVATE_TESTING_DIR=/home/nandev/Documentos/projects/espaciGo/.local/testing \
PHASE6_ACCOUNT_INVENTORY_FILE=/home/nandev/Documentos/projects/espaciGo/.local/testing/f5-gate-20261101-f5a3/f5-a3-accounts-20261104T074455Z.json \
PW_TEST_NO_SNAPSHOT=1 \
./node_modules/.bin/playwright test --config=playwright.space-gallery.config.ts
```

El selector consumido no se reutiliza para acreditar creación: usar un sufijo sintético fresco con el formato validado al repetir. La corrida creó borrador (201), configuró zona (200), recargó/editó/recargó, subió tres fotos por UI, seleccionó portada/ordenó, verificó que reemplazo inválido conservara el DTO completo anterior y reemplazo válido201, aceptó confirmación de retirada y leyó ausencia tras DELETE200. Preparó30 por BFF real, obtuvo409 en foto31 sin cambiar conteo, recargó30 imágenes decodificables, comprobó AXE en ambos tamaños, publicó desde UI (200) y leyó estado activa. Otra cuenta existente recibió403/forbidden sin items; logout204. Sin SQL writes, cuentas nuevas, mocks, traces/video/capturas de privados ni destinos externos.

## Fallos corregidos, no omitidos

- Global inicial58PASS/49SKIP/11FAIL: mocks8/9 tenían expires_at vencido; corregido plazo fijo al crear fixture (no renovación por request). Guard loopback confundía blob privado con red externa; ahora valida origen del blob. AXE encontró contraste insuficiente en Publicar y Eliminar sobre portada; colores corregidos y gates repetidos.
- Test nativo adjuntaba archivos antes de que terminase reorder porque setInputFiles no espera enabled. Se exige enabled. Retirada acepta confirm y exige readback.
- Selector histórico final6 ya existía sin zona horaria. Publicación se bloqueó correctamente; no se cambiaron permisos/gates. El test ahora exige fixture fresco para validar creación.
- Dos ejecutores no completaron correcciones; el padre implementó y ejecutó GREEN. Un exit0 CLI no se usó como prueba de tarea terminada.

## Demo y respaldo

Demo retenida: `https://127.0.0.1:4203/anfitrion/espacios`, proxyTLS4203→Next4204→Go18082 worker-free→127.0.0.1:15434/espacigo_f5_gate_20261101. Ruta exacta e inventario solo bajo `.local/testing/` del backend (0600); no copiar credenciales a esta evidencia. Responsable de retención: usuario para revisión. No detener PostgreSQL tmpfs por limpieza. No se reiniciaron scanner/Mailpit ni se tocaron recursos ajenos.

Readback V1–V83:83/83 nombres/checksums exactos. Backup dump externo `gallery-accepted-20261105T010447Z.dump`, SHA256 `5b0539408db4789dc423bcc2a83bd573b256cd46b2aa88edeacb9f3c3a4c2257`,717956bytes; TOC/decodificación completa pg_restore a dev/null PASS. Storage externo `gallery-storage-accepted-20261105T010447Z.tar.gz`, SHA256 `27b668c0cd0cf6cf1b466faf5fdfd63d39970ec7ecbcf907498ee0d88967052c`,110189bytes/852entradas, lectura completa PASS. Se guardan en `/home/nandev/.local/share/espacigo-f5-backups/` (0600), fuera de Git/tmpfs. UTC05 corresponde a noche local04. No se afirma restore del backup nuevo.

Evidencia matriz/capturas y mediciones completas en la rama backend: `planning/frontend_next/evidence/espacios-wizard-galeria-local-20261104.md`.

## Límites pendientes separados

- Peso: display texturaJPEG1044618, texturaWebP1190300, transparencia1934686bytes exceden614400; thumbnail transparente176864 excede102400. Dimensiones/orientación/metadatos/WebP decodificable PASS. Mantener calidad80; falta piso visual/corpus representativo para optimizar sin degradación arbitraria.
- UI serializa una foto; no se acredita cola2 completa del plan. Pending/retry cubierto por regresiones ejecutables simuladas, no202 provocado en API real.
- Fingerprint ACL OID-dependent tras última restauración continúa sin aprobación. Equivalencia estructural exacta y recorrido runtime son evidencias distintas; no hubo grants ni nueva baseline.
- Republicación tras desocupación, lectura binaria por revisores y contratación/cobros siguen en cortes separados. Depósito deshabilitado.
- No main, PR, F7/cloud ni despliegue. La aceptación visual/de producto corresponde al usuario.
