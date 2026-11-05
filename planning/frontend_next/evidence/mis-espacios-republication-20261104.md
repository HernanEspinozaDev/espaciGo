# Mis espacios — confirmación de desocupación y republicación local

Código probado frontend `5dc7095d44770fc0e59b76b8e3dd1f1b515de3e5`, backend `b2d7c117b06b806f5c7a33725f85d0f7b1d47be8`. Ramas de tarea vigentes, sin main, deploy o F7.

- Se reutiliza POST reception y checkout existentes; no se monta un flujo paralelo de firma/cobros/garantía.
- Panel explícito en editor de Mis espacios: casilla, confirmación, readback y botón Republicar para oculta. Solo servidor decide titularidad y liberación; no hay éxito optimista ni republicación automática.
- Fachada conserva incertidumbre y la misma intención, bloquea mutaciones competidoras, permite GET manual y no duplica POST; solo vacancy_confirmed verdadero despeja el bloqueo. clear y key sesión/permisos/espacio aíslan respuesta y casilla.
- Adaptador envía cuerpo `{}` conforme a Go estricto; el result lo determina servidor a partir de observations. El mock de espacio se corrigió a DTO directo e IDs UUID válidos: el wrapper data anterior impedía mostrar el panel.
- Editor de tarifas conserva CLP y hora/dia/mes; min=5001, step=1 y validación >5000 sustituyen min/step100 incompatibles con 5001 válido.

## Evidencia ejecutada por el padre

| Gate | Resultado |
|---|---|
| Tipos, lint, build, API check | PASS |
| Unitarias tooling | 118 PASS, 0 FAIL, 0 SKIP |
| Playwright global final | 72 PASS, 50 SKIP, 0 FAIL |
| Nativo `playwright.republication.config.ts` | 1 PASS, 0 FAIL, 0 SKIP |
| Revisión independiente frontend | Hallazgo de checkbox entre sesiones corregido; re-revisión PASS de fuente |

Nativo con fixture fresco final 20261104-return3: nueve categorías/perfiles reales y opciones residenciales, cuenta ajena denegada, recepción antes de checkout denegada, checkout existente, publicación antes de confirmar denegada, recepción desde casilla/botón por HTTPS BFF→Go→PostgreSQL, readback confirmado, Republicar 200 y reload activa. Axe main desktop/móvil y overflow pasan. No se usaron mocks ni se creó otra cuenta.

Readback independiente y SQL posterior: activa, 30 fotos/una portada conservadas, intervalo exacto terminado inactivo, otros bloqueos activos, contrato histórico sin cambios y titular sin aprobación identity vigente. La preparación de arriendos/contrato histórico fue un fixture sintético explícito; no acredita reserva, emisión, firma o pagos.

Crear/guardar/reload/galería, cupo30/31 y transparencia conservan la evidencia válida anterior; este test no los presenta como otra creación completa. Detalle de matriz, referencia ACL independiente, texturas, backups y límites: evidencia backend `planning/frontend_next/evidence/mis-espacios-republicacion-acl-20261104.md`.

Demo retenida https://127.0.0.1:4203/anfitrion/espacios, acceso mediante las dos cuentas sintéticas ya autorizadas. Credenciales/selectores solo en la raíz privada ignorada. Para reproducir, preparar un selector nuevo con el script backend y usar PHASE_REPUBLICATION_NATIVE_E2E, PHASE6_PRIVATE_TESTING_DIR, PHASE6_ACCOUNT_INVENTORY_FILE y PHASE_REPUBLICATION_FIXTURE_FILE; trace/video/screenshots de la suite nativa apagados.

Pendientes separados: corpus/piso visual de textura; gate operacional ACL anterior no modificado; revisión documental y reserva/contrato/cobros/garantía según ADR; autorización de main/cloud y aceptación humana. Los commits documentales posteriores no cambian el código probado.
