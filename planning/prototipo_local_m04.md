# Prototipo local M04: borradores privados de espacios

Este primer recorrido M04 permite crear, listar, consultar y editar borradores completos propios mediante Base de Datos, Backend y API reales y el mock sencillo. Trabaja en la rama/PR del corte #128–#133. No finaliza M04 ni cierra las Issues amplias #52–#61.

## Levantar y probar

1. Desde la raíz, inicia el entorno con `scripts/dev-env.sh up -d`. El script conserva `espacigo_pgdata`, credenciales locales y datos sintéticos, y aplica solo migraciones pendientes (incluida V5); no uses `clean`, `down --volumes` ni borres secretos.
2. Abre el mock en `http://127.0.0.1:8081`. Registra una cuenta sintética si no tienes una activa. Abre Mailpit en `http://127.0.0.1:8025`, verifica el correo y luego inicia sesión.
3. En **Espacios propios · borradores M04**, elige una categoría, completa título, descripción de al menos 100 caracteres, superficie positiva, capacidad, reglas, unidad (hora/día/mes), precio base superior a $5.000 CLP y dirección privada. Pulsa **Crear borrador**.
4. Pulsa **Cargar mis borradores**; el elemento creado aparece con estado `borrador`. Ábrelo para cargar los datos, edita un campo y pulsa **Guardar cambios**. Recarga la lista para comprobar persistencia.
5. Para comprobar ownership, crea una segunda cuenta y confirma que su lista no incluye el borrador anterior. Las pruebas automatizadas verifican además que leer/editar por ID ajeno produce el mismo 404 que un ID inexistente.

## Contrato y límites

Todas las llamadas `/api/v1/spaces*` requieren bearer y derivan el propietario desde la cuenta validada por M01. `GET /api/v1/spaces/categories`, `POST/GET /api/v1/spaces`, `GET/PUT /api/v1/spaces/{id}` están documentadas en `planning/openapi.yaml`. El JSON no admite propietario, estado ni atributos desconocidos. No hay endpoint de borrar, activar o publicar.

La dirección queda almacenada como texto privado; geocodificación, coordenadas, búsqueda pública, galería, subida de archivos, storage privado y calendario quedan para cortes posteriores. Aprobación KYC sintética no habilita permiso comercial. No se implementan reservas ni cobros. Categorías: Oficina, Sala/multipropósito, Bodega, Estacionamiento, Local flexible, Stand, Quincho y Parcela/eventos.

Las pruebas PostgreSQL se ejecutan solo con `TEST_DATABASE_URL` de un contenedor descartable y sin `DATABASE_URL`; no usan el volumen persistente. Los pendientes M02/M03, DB02-09 y #123 permanecen separados.
