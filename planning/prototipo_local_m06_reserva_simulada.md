# Probar M06-LOCAL-01 — reserva con pago simulado

La entrega usa la DB local persistente sin reiniciarla. `scripts/dev-env.sh up -d` conserva `espacigo_pgdata` y secretos y aplica migraciones pendientes incrementalmente (V15 incluye allowlist multi-fixture). No usar `clean`, `down --volumes` ni pruebas destructivas contra esa base.

## Preparación

1. `bash scripts/dev-env.sh up -d`
2. Registra dos cuentas sintéticas distintas en <http://127.0.0.1:8081>, verifica ambos correos en Mailpit (<http://127.0.0.1:8025>) e inicia sesión una vez en cada cuenta para comprobarlas.
3. Habilita una vez el fixture privado (solo cuentas activas y con correo verificado):

   ```sh
   bash scripts/enable-local-booking-fixture.sh anfitrion@example.test arrendatario@example.test sala_multiproposito
   ```

   El comando administrativo crea o reutiliza un borrador sintético para la categoría indicada (por defecto `sala_multiproposito`), tarifa inicial CLP $8.000/h y zona `America/Santiago`, y asigna explícitamente las dos cuentas. Tras V15, cada espacio es una fila allowlisted independiente; el mismo par/categoría es idempotente. No consulta ni modifica KYC; no concede roles comerciales ni expone borradores no allowlisted. Para habilitar otro ejemplo se invoca con otra categoría.

## Recorrido

Abre la aplicación en dos perfiles/ventanas independientes del navegador, uno por cuenta; las sesiones del mock viven solo en memoria.

1. En ambas, autentica la cuenta y abre **Reserva sintética · M06 local**. El aviso **ENSAYO LOCAL — SIN COBRO REAL** permanece visible.
2. Como arrendatario, consulta el fixture autorizado, elige fechas futuras y crea una cotización. Confirma snapshot de tarifa, subtotal, reglas de uso, zona y vencimiento; aún no hay ocupación.
3. Solicita la reserva. Queda `pendiente_de_pago` y se crea retención `[inicio, fin)` en `ocupacion` atómicamente; copia el ID.
4. Como arrendatario, elige éxito, rechazo o sin respuesta. Éxito cambia a `pagada`, extiende la retención hasta el límite de respuesta de 24 h; rechazo la libera; sin respuesta conserva `pendiente_de_pago` hasta el timeout de 15 min.
5. Como anfitrión, consulta historial y aprueba o rechaza. La aprobación queda `aprobada_host`; rechazo termina en `rechazada_arrendador`, registra devolución simulada y libera ocupación.
6. Vuelve a consultar el historial con ambos participantes. `GET` procesa vencimientos pendientes: pago a 15 min y respuesta del anfitrión a 24 h desde el pago. La transición queda en historial y libera ocupación.

Para repetir, usa otro intervalo sin solape y una clave de solicitud nueva. El mismo `Idempotency-Key` con el mismo quote recupera el mismo caso; un cuerpo distinto responde 409. El test concurrente verifica que dos claves no retengan un mismo intervalo.

## Comprobaciones

`bash scripts/test-m06-local-booking-postgres.sh` inicia PostgreSQL desechable aislado y ejecuta la integración con rol `espacigo_runtime`. No usa la DB persistente. Sin Docker/TEST_DATABASE_URL, la prueba se omite; no equivale a aceptación.

El fake no invoca proveedor, no crea movimientos reales, no promete devolución real y no confirma fondos. Los deadlines persisten en DB; la expiración se materializa al consultar o al intentar otra operación. Un worker durable y conciliación son criterios generales M06 aún pendientes. El perfil local y el puerto de API quedan vinculados a loopback; no exponer este ensayo en red.
