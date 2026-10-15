# Cargador del kit `espacigo-1000`

## Alcance del primer corte

`cmd/dataset-loader` solo ejecuta una prevalidación de lectura. Comprueba el manifiesto, las 1.000 asignaciones de cuenta, las 1.200 asignaciones de publicación y escenario, los JSONL de contenido, las relaciones entre anfitrión/publicación/arrendatario y los totales de categoría, visibilidad y tipo de escenario. Su salida contiene conteos, no nombres ni contenido. No abre conexión a PostgreSQL y no crea cuentas ni transacciones.

Ejemplo, con el kit mantenido fuera de Git:

```sh
go run ./cmd/dataset-loader --dataset-dir /ruta/privada/.local/datasets/espacigo-1000
```

La salida `preflight passed (no database writes)` es evidencia de integridad estructural del kit solamente. No acredita una carga ni el cumplimiento de las reglas de dominio.

## Cortes siguientes

1. **Importador reanudable de identidad.** Registrar las cuentas usando el servicio de identidad, correo local de verificación, contraseña leída solo desde la política privada 0600 y correo sintético bajo `.invalid`. Persistir en `.local/testing/` un mapa privado de referencia externa a UUID devuelto por el servicio y checkpoints por acción. Resolver roles con el mecanismo administrativo vigente; las diez cuentas planificadas como administradoras reciben solo el rol legado, nunca el rol global. Una repetición debe reconocer acciones ya completadas y comprobar el resultado antes de continuar.
2. **Publicaciones y calendario.** Crear borradores, tarifas CLP `int64`, perfil de atributos, zona IANA y horarios semanales mediante servicios Go. Ejecutar transiciones de publicación/ocultación y asociar fotos exclusivamente a fixtures sintéticos explícitos. Revalidar visibilidad efectiva; la intención del JSONL no es prueba de publicación.
3. **Escenarios de trato.** Ejecutar cada escenario por servicios/API del dominio, incluyendo eventos fake actuales. Un reloj controlado debe pertenecer a un proceso y entorno de ensayo dedicados; no avanzar el reloj compartido del API persistente. Guardar y verificar checkpoint después de cada transición.
4. **Ensayo aislado.** Restaurar el dump previo en un PostgreSQL efímero separado, completar un piloto de 20 cuentas con publicaciones y escenarios representativos, validar integridad y resumir resultados sin identificadores personales. Retirar solo los recursos nombrados del ensayo y conservar el backup origen.
5. **Carga persistente.** Solo después de fusionar este trabajo y verificar preflight, restauración y piloto: tomar/validar backup reciente, comprobar inventario y espacio, fijar HEAD, ejecutar el cargador con destino explícito `espacigo_local`, confirmar conteos y transiciones, y escribir el informe privado. Una interrupción debe permitir reanudación por checkpoints sin borrar filas ni reiniciar volúmenes.

## Límites

La implementación actual cubre únicamente la prevalidación estructural. Los cortes siguientes son trabajo pendiente, no capacidades ya disponibles. Esta carga no crea infraestructura GCP, pagos reales, verificación de identidad real, firmas reales ni fotos reales. Ningún historial, libro financiero, reserva, contrato o estado de cuenta se debe construir mediante SQL de importación.
