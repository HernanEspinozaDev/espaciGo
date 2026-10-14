# LOCAL-CLOSE-00 — base, preservación y conciliación

**Fecha:** 2026-10-14 (America/Santiago). **Baseline de release:** `28f63f3dcba5d1b6a57b47b0c229e0caf2bba7eb`, merge de PR #307. **Worktree de cierre:** `/home/nandev/.codex/worktrees/local-close-00-baseline-plan/espaciGo`, rama `codex/docs/chore/local-close-00-baseline-plan`, creado desde `origin/main` en el mismo SHA. El worktree estaba limpio; no se ejecutaron pruebas ni comandos Compose.

## Checkout habitual preservado

El checkout habitual continúa en `main` con HEAD `c797f0eb734e2c7eda37eb70a50d4d509e82677d`, cinco commits detrás del remoto. Tras `git fetch origin --prune`, `origin/main` estaba en `28f63f3`. No hice fast-forward, reset, stash, checkout de archivos ni copié sus modificaciones al árbol de cierre.

Los cinco archivos trackeados modificados eran:

- `frontend/e2e/fe14.spec.cjs`
- `frontend/src/app/features/explore/explore-page.html`
- `frontend/src/app/features/explore/explore-page.scss`
- `frontend/src/app/features/explore/explore-page.spec.ts`
- `frontend/src/app/features/explore/explore-page.ts`

Además estaba sin tracking `planning/auditorias/`, con el MD integral, un JSON de resultados y logs/JSON de comprobaciones. Se conservaron todos en una copia privada local excluida de Git junto con patch binario, hashes de originales/copias y manifest:

- Directorio: `.local/preservation/local-close-00-20261014/`
- SHA-256 de `manifest.txt`: `0f135269794e46815ee4016c3ea4f593da853ad61833591aec7686fcd671822f`
- SHA-256 de `checkout.patch`: `60150ff51ebda9f8ecff92c0b30e3dec0803f4c01cc0e4096900226eb49cd164`
- Confirmación de exclusión: `.gitignore:2:.local/`

La copia incluye los archivos untracked para preservarlos, pero no se añadieron ni publicaron logs/JSON sin revisión. El texto de la auditoría original se leyó como diagnóstico; el plan y la matriz versionados contienen únicamente findings y estados resumidos que se contrastaron.

## Comparación contra #307

Comparé los cinco archivos locales con `origin/main` (`28f63f3`, merge #307); no realicé cherry-pick ni propagué diffs. La clasificación por contenido (sin atribuir autoría ni intención) es:

| Archivos | Clase | Comparación con #307 y tratamiento |
| --- | --- | --- |
| `explore-page.ts`, `.html`, `.scss` | Trabajo de producto local no integrado | Reintroduce estado/controles de vista y columnas, medición de ancho con `ResizeObserver` y layout por viewport; contradice el layout CSS fijo aceptado. Preservado, sin copiar al PR. |
| `fe14.spec.cjs` | Mixto: escenarios redundantes y expectativas distintas | Reescribe cobertura de layout/page size con controles anteriores; cambia robots a `index, follow`; elimina la prueba conectada de páginas keyset 48 y algunos asserts de booleanos tipados presentes en #307. También conserva cobertura móvil/orden que puede ser útil. No es duplicado puro ni evidencia de comportamiento aceptado; revisar cada caso antes de reutilizar. |
| `explore-page.spec.ts` | Cambios de prueba no integrados | Incluye cobertura adicional de cómo completar requisitos de orden desde el diálogo móvil. No implica que el código actual cambie ni que esa UI haya sido aceptada; queda como candidato de prueba aparte. |
| `planning/auditorias/` | Diagnóstico/evidencia local sin tracking | Se copió completa a la preservación excluida para mantener originales. Solo hallazgos revisados se resumieron en plan/matriz; ningún log o JSON bruto se añadió al PR. |

Hay comprobaciones de page size/orden que solapan funcionalidad ya cubierta, pero dependen de controles de vista retirados; las marqué como solapamiento dependiente del diseño anterior, no como duplicado directamente reutilizable. Por estar mezclados en los mismos archivos, no promoví ningún hunk ni decidí qué cambios corresponden a otro propietario. El diff íntegro y el snapshot están fuera de Git, en la ruta/manifiesto anteriores.

## Estado conciliado desde fuentes actuales

- **FE-15:** PR #292 fusionado en `5405a82e90bc429aa89666ead430fe2cb39f102e`; Angular reemplaza el mock. Registros FE14 que dicen “FE-15 no se inicia” son históricos/pre-fusión.
- **MPV2-13:** PR #307 fusionado por `HernanEspinozaDev` en `28f63f3`; aceptación del usuario acotada a UX/demo/código `736f81a`. No acepta DOCKER-02 ni cierre local integral.
- **DOCKER-01:** #268 cerrada/aceptada en su alcance tras PR #271, merge `e074d78f126cd3a5bbe9c108da26178bdb726ad4`.
- **DOCKER-02:** #269 cerrada administrativamente el 2026-10-12, con un comentario que declara criterios funcionales no comprobados; #267 sigue OPEN. No se cambia el estado de Issues en LOCAL-CLOSE-00.
- **DOCKER-03:** #270 cerrada/aceptada dentro de su alcance tras PR #273, merge `b0f62b976c9586d260cea15ed9acc29ab12257a7`; su evidencia expresa que no sustituye DOCKER-02.
- La evidencia del recorrido Docker de 2026-10-11 registra el HEAD exacto del ensayo como no consignado. La suite FE-14/Angular/Go #307 es evidencia de otro corte/SHA, no una nueva corrida DOCKER-02. Ver detalle y límites en `../../matriz_cierre_local.md`.

## Resultado de LOCAL-CLOSE-00

El árbol habitual no se sincronizó porque tiene trabajo ajeno sin integrar. El árbol limpio de release desde `origin/main` es la base canónica documentada para este PR y para decidir cortes posteriores. Se preservaron los cambios locales y la auditoría original; no se inició LOCAL-CLOSE-01, LOCAL-CLOSE-02, GCP-PREP ni se tocaron las demos retenidas o `espacigo_pgdata`.
