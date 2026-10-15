# Guía práctica de la demo GADM-04

La demo es local, sintética y sin pagos ni efectos reales. Para prepararla ejecuta desde la raíz del repositorio:

```bash
bash scripts/prepare-gadm-04-demo.sh
```

Al terminar, el script muestra la URL Angular (`http://127.0.0.1:<puerto>/`), URL de health, carpeta del estado privado y el RUN_ID. Las cinco cuentas y contraseñas están en `.local/testing/gadm-04-demo/<RUN_ID>/accounts.json` (0600); no las copies a capturas, tickets ni archivos versionados. Usa `globalAdmin` para administración global, `admin` para el administrador heredado, `host` para dueño de publicación, `renter` para la cuenta objetivo y `thirdParty` como tercero sin permisos administrativos.

## Recorrido de cuenta y publicaciones

1. Abre la URL Angular e inicia sesión como `globalAdmin`. En **Administración global → Directorio de cuentas**, busca por correo o prefijo; abre la ficha minimizada. Edita solo los campos permitidos y guarda. Los cambios de perfil no agregan roles.
2. En la misma ficha usa **Bloquear cuenta y revocar sesiones** con un motivo habilitado. Confirma que la sesión de `renter` queda revocada; luego desbloquea con motivo y autentica una sesión nueva. Tanto la acción como su autor quedan auditados.
3. Ve a **Publicaciones**. Abre una publicación sintética y revisa su ficha, galería y horario. Cambia visibilidad de una foto sintética y un bloque de horario; la interfaz muestra que las fotos son fixtures locales y no almacenamiento real.
4. Prueba **Retirar** primero sobre “Retiro con obligación FE-14”: debe rechazarlo mientras existe una reserva/obligación abierta. Retira “Retiro permitido FE-14” y comprueba su estado/historial. No se borran físicamente reservas, contratos, snapshots ni evidencia.
5. Cambia a `admin` y comprueba que las rutas globales son 403; las rutas heredadas siguen disponibles. Cambia a `thirdParty` y verifica que las superficies globales y heredadas deniegan acceso.

## KPI, auditoría y recuperación

En **Indicadores y auditoría**, define un rango temporal y zona horaria. `generated_at`, cantidades, dinero y ventanas provienen de Postgres; CLP se conserva con exactitud. El demo no instrumenta visitas ni DAU, y los importes no representan liquidaciones/pagos reales.

Filtra auditoría por actor, cuenta objetivo, recurso, motivo, resultado o correlación. Actor y objetivo pueden ser cuentas distintas. Prueba paginación con cursor y exporta el mismo filtro como CSV y JSON. La exportación es una descarga privada del navegador: verifica el archivo local descargado antes de compartirlo. Un error de rango/motivo/versión se muestra como rechazo; corrige el dato, vuelve a aplicar y confirma que el evento de rechazo/éxito queda separado. Al recibir 401/403, inicia sesión con el actor apropiado; no reutilices tokens revocados.

## Retención y retiro

Para detener sin eliminar datos ni cuentas:

```bash
bash scripts/stop-gadm-04-demo.sh <RUN_ID>
```

Para reanudar y esperar readiness de API y frontend, ejecuta: `bash scripts/start-gadm-04-demo.sh <RUN_ID>`. Para retirar de forma definitiva el demo y eliminar sus credenciales/volumen aislado (después de la revisión):

```bash
bash scripts/remove-gadm-04-demo.sh <RUN_ID>
```

El wrapper valida el RUN_ID y su estado antes de operar; solo apunta al project GADM-04 correspondiente. Se retiene bajo responsabilidad de HernanMEC para la revisión local solicitada. La retención puede continuar hasta que la revisión concluya; después debe retirarse y eliminarse el estado privado.
