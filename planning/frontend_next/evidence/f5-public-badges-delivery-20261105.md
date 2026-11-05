# Entrega local — distintivos públicos documentales, 2026-11-05

## Alcance

Continuación autorizada por el usuario del plan de espacios: distintivos independientes de publicación, scanner vigente y validación local sintética. No integra a main ni despliega; la aceptación humana es la siguiente puerta.

Código frontend probado `62310202e9b5cfbc5f80db906966cee9368d541f`; backend coordinado `ce7aa563d40316448cc116e4d52e90c3e487a317`. El commit documental posterior no cambia fuentes. HEAD/ref final se comprueban después del push a la rama de tarea, sin PR ni force-push.

## Comportamiento

- Lista y detalle normalizan el campo opcional OpenAPI `verification` a dominio. Ausencia o valores no booleanos no generan un distintivo; espacio depende de identidad vigente. Relectura `no-store`, detalle dinámico; no se conservan flags de una respuesta previa.
- Tarjeta y ficha muestran «Identidad verificada» y «Antecedentes del espacio revisados». Avisos accesibles no acreditan dominio, legalidad, garantía ni firma. Solo se exponen dos booleanos, no IDs o documentos privados.
- Backend deriva desde la última decisión del ámbito/titular actual antes de comprobar aprobación/vigencia, sin recuperar decisiones históricas tras revocación o vencimiento. Catálogo no exige KYC contractual para publicar. Catálogo local se monta por flag explícito worker-free, sin activar reservas/pagos.

## Evidencia ejecutada por el padre

- Frontend144 unitarias PASS; tipos/lint/API/formato/build PASS. Un warning preexistente sobre `img`; cero errores lint.
- E2E global aislado4210:76PASS/53SKIP. Los omitidos no se cuentan como integración. El ensayo nativo nuevo está opt-in y se ejecutó aparte.
- Seis invocaciones nativas en HTTPS4203, sin mocks: revocación previa para baseline; identidad aprobada; espacio aprobado/revocado; vencimiento real sin worker/reloj artificial; identidad revocada con anuncio aún visible; demo pendiente/PDF y tarjeta pública sin badges. AXE desktop/mobile y sin overflow PASS.
- Backend: Go completo/vet PASS;10escenarios PostgreSQL de catálogo y4del helper identidad PASS, fixtures de metadatos con rollback. Constraint V80 rechaza identidad sustituida (SQLSTATE23514 exacto); la objeción de revisión fue retirada tras verificarla. Revisiones independientes frontend y backend sin bloqueos vigentes.
- Scanner oficial actualizado daily28174 del2026-11-05T06:26Z, control limpio/EICAR reales PASS; se reanudó el caso antes bloqueado y se prepararon los nuevos documentos mediante inspección/escaneo reales. Retirada de egress comprobada. Política48h intacta.

## Demo y reproducción

Demo retenida `https://127.0.0.1:4203/mi-cuenta/revision-documental`; público `/explorar`. Mantiene un nuevo expediente de identidad enviado/limpio, dos cuentas existentes y asignación exacta vigente. Los anuncios siguen visibles sin badges al final porque las decisiones del ciclo fueron revocadas o vencieron; no se resetearon para una captura.

Harness: `pnpm exec playwright test --config playwright.public-verification-native.config.ts`, con `PHASE_PUBLIC_VERIFICATION_NATIVE_E2E=1`, `PHASE_PUBLIC_VERIFICATION_SCENARIO` y raíz/inventario/fixture privados explícitos mediante `PHASE6_PRIVATE_TESTING_DIR`, `PHASE6_ACCOUNT_INVENTORY_FILE`, `PHASE_DOCUMENT_REVIEW_FIXTURE_FILE`. Escenarios `identity`, `space-revoke`, `space-expire` e `identity-revoke` consumen una decisión preparada; `demo` es lectura y descarga, sin decidir. No copiar credenciales ni inventario a este worktree.

API18082 con perfil `local-http-worker-free` y flags locales explícitos de reviewer/catalog; Next standalone4204 detrás de TLS4203. Next no recibe variables DB, passfiles ni claves documentales. Recursos retenidos y recibos privados pertenecen al ensayo; no detener tmpfs ni limpiar volúmenes. Backup final DB/storage y firmas oficiales fuera de tmpfs, hashes validados. No cuentas nuevas ni nuevas migraciones/ACL.

La matriz única y evidencia principal se mantienen en la rama backend: `planning/frontend_next/plan_creacion_espacios_galeria_documentos.md` y `evidence/f5-distintivos-scanner-local-20261105.md`. Allí se conservan capturas públicas nuevas antes/después de la expiración y el registro de recuperación autorizada.

## Pendiente humano

Revisión/aceptación local por el usuario. Integración a main, F7/cloud, reserva, firma, cobros y garantía no se ejecutaron ni quedan aceptados por estos resultados.
