# Publicación local normal y lectura pública real

## Autorización y destino

El usuario autorizó expresamente el ensayo público local del borrador horario existente mediante el flujo normal Go/UI, con CAS, auditoría y readback. Sólo `espacigo_wizard_20261106`, API worker-free `127.0.0.1:18086` y Next HTTPS `127.0.0.1:4206`; no cuentas nuevas, SQL de escritura, IAM, migraciones, GCP ni navegación online.

Se extendió el opt-in del catálogo al perfil wizard con guard exacto de destino, manteniendo el destino F5 legado y rechazando perfiles/destinos cruzados. El launcher requiere `--public-catalog` y clave privada del propio ensayo; sin opt-in no habilita ni hereda esas variables. El padre repitió las seis pruebas aisladas del launcher y pruebas/vet de Go. Binario público actual: SHA-256 `3d95e1cf497ed2facd41d26148ddf91eecb27d19162eabc6f2012bca0ee81fc5`. Identidad, digest, perfil y destino se comprobaron leyendo el proceso. Readiness y categorías públicas: 200; el borrador antes de publicar: 404.

## Ejecución real del padre

Desde `frontend/`:

```sh
ESPACIGO_WIZARD_PUBLIC_NATIVE=1 node_modules/.bin/playwright test \
  --config=playwright.wizard-public-native.config.ts \
  --grep 'publishes the verified'
```

Resultado: **1 passed (4.7s)**. Hubo un único clic normal en Publicar y un único PUT de publicación con `If-Match` de la lectura fresca. El recibo privado exclusivo se sincronizó antes de la mutación. El propietario releyó el estado `activa`; una lectura independiente posterior confirmó el recibo `verified`. **No volver a ejecutar el caso fresco**: el recurso ya está publicado en el ensayo. Cualquier repetición debe usar exclusivamente el caso de reconciliación sin PUT.

El contexto anónimo nuevo, sin cookies del propietario, obtuvo 200 desde el BFF público real. Validó el DTO minimizado y su horario completo: siete días ISO, lunes 09:00–17:30, martes a viernes 09:00–18:00, fin de semana vacío y America/Santiago. La ficha SSR real mostró el calendario de solo lectura y la franja parcial; no llamó al endpoint semanal del propietario. Axe completo y contención pasaron en escritorio y móvil.

Desde la raíz, el padre ejecutó los dos casos PostgreSQL con los flags explícitos `ESPACIGO_WIZARD_PUBLIC_SQL_READ=1` y `ESPACIGO_WIZARD_PUBLIC_PUBLISHED_READ=1`, usando únicamente el passfile runtime aprobado:

- `TestWizardNativeDetailReadOnlyDraftExcludedFromPublicCatalog`: **PASS**, sobre el borrador día que sigue sin publicarse.
- `TestWizardNativeDetailReadOnlyPublishedHourVisibleWithAudit`: **PASS**, hora activa, exactamente una fila de auditoría de borrador a activa atribuida al propietario con correlación no vacía, y proyección/Detail coincidentes con los tramos almacenados.

Ambos verifican identidad y modo read-only; no fabrican fixtures ni cambian estado por SQL. La identidad contractual no se falsificó: la publicación usa sus condiciones normales existentes, separadas de reservas y contratación.

## Evidencia visual y límites

Capturas regionales bajo `/home/nandev/.hermes/cache/scratch/espacigo-wizard-evidence/native-public-final/`. El padre inspeccionó escritorio: horario exacto 17:30, etiqueta Parcial y leyenda orientativa legibles. La captura móvil inicial tenía el encabezado tapado por el header sticky; se preservó como `mobile-initial-overlay.png`. Se corrigió sólo la posición mediante scroll real, calculando el borde del header y un margen de 24 px, sin modificar estilos, ocultar elementos ni ampliar el viewport.

Después, el padre ejecutó exclusivamente `--grep 'reconciles an existing'`: **1 passed (4.3s)**, sin repetir el PUT. La prueba exige que la región quepa completa en el viewport y esté debajo del header; repite lectura anónima, Axe y contención. El padre abrió el `mobile.png` reproducido y confirmó encabezado completo, leyenda, controles y cierre 17:30 legibles, sin recortes. Esta revisión técnica no es aceptación visual humana.

El primer intento de este test se detuvo en navegación antes de crear intención o publicar. Se corrigieron esperas de campos desaparecidos y `domcontentloaded`, sin aumentar el timeout. El intento exitoso posterior es el único que envió el PUT de publicación.

La hora se retiene activa únicamente para evidencia local; día y mes conservan sus estados de ensayo. No se ha publicado este código, integrado a main ni desplegado. La revisión humana/online y la cadena de release permanecen gates separados.
