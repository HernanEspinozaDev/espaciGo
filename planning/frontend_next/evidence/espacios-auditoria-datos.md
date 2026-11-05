# Auditoría de Modelo Real de Datos, Taxonomía, Media y Validaciones

**Fecha de Auditoría:** 2026-10-27  
**Proyecto GCP:** `espacigo-testprod` | **Región:** `southamerica-west1`  
**Base de Datos Cloud SQL:** `espacigo-testprod-db` / `espacigo_dms_stage_20261020` (IP Privada confirmada gcloud: `10.189.0.15`)  
**Bastión:** `espacigo-dms-bastion-v67` (`southamerica-west1-a`, Compute ID verificado: `5282317084229207114`)  
**SDK Account:** `pasteleriahijitos@gmail.com`  
**Catálogo Live API:** Verificado HTTP 200 en `/api/v1/public/space-categories` (9 categorías canónicas reales). Los perfiles detallados de esta matriz proceden de las migraciones; no se presentan como una nueva lectura SQL de sus filas. La comprobación SQL específica se documenta por separado.  
**Ledger de Migraciones Cloud:** 75 migraciones incrementales completas (V000001 a V000075) verificadas en `.local/testing/recuperacion-schema-release/img01-schema-evidence-49931127a2d5ab6a8cb08876244457778fafd048b34028cb3ddec30ccde3a77c.json`.

> Evidencia histórica de los cortes 2026-10-27/28; ni el catálogo, ledger ni despliegue se presentan como estado remoto actual. Para la reconciliación vigente de fases y límites, consultar `plan_creacion_espacios_galeria_documentos.md` §8 (2026-10-29).

---

## 1. Cadena de Identidad, Cuentas, Anfitrión y Propiedad (Ownership)

### 1.1 Modelo Relacional de Identidad
* **Tabla `public.usuario` (`V000001__m01_identity.sql:5-22`):**
  - `id uuid PRIMARY KEY`
  - `correo_original text NOT NULL`, `correo_normalizado text NOT NULL UNIQUE`
  - `hash_clave text NOT NULL`
  - `estado text NOT NULL CHECK (estado IN ('correo_pendiente', 'activo', 'bloqueado', 'baja_solicitada', 'desidentificado'))`
* **Tabla `public.rol_usuario` (`V000001__m01_identity.sql:24-35`):**
  - `usuario_id uuid NOT NULL REFERENCES public.usuario (id) ON DELETE RESTRICT`
  - `rol text NOT NULL CHECK (rol IN ('arrendatario', 'arrendador', 'administrador'))`
* **Tabla `public.perfil_usuario` (`V000002__m02_profile_rights.sql:2-17`):**
  - `usuario_id uuid PRIMARY KEY REFERENCES public.usuario (id) ON DELETE RESTRICT`
  - `nombre_visible text NOT NULL`, `telefono_normalizado text`
* **Tabla `public.espacio` (`V000005__m04_space_drafts.sql:20-40`):**
  - `id uuid PRIMARY KEY`
  - `propietario_id uuid NOT NULL REFERENCES public.usuario (id) ON DELETE RESTRICT`
  - `categoria_codigo text NOT NULL REFERENCES public.categoria_espacio (codigo) ON DELETE RESTRICT`
  - `titulo text NOT NULL`, `descripcion text NOT NULL`
  - `area_m2 numeric(10, 2) NOT NULL CHECK (area_m2 >= 0.01 AND area_m2 <= 99999999.99)`
  - `capacidad integer NOT NULL CHECK (capacidad > 0)`
  - `normas_uso text NOT NULL`
  - `unidad_tarifa text NOT NULL CHECK (unidad_tarifa IN ('hora', 'dia', 'mes'))`
  - `precio_base_clp bigint NOT NULL CHECK (precio_base_clp > 0)`
  - `direccion text NOT NULL`
  - `estado text NOT NULL DEFAULT 'borrador' CHECK (estado IN ('borrador', 'activa', 'oculta', 'archivada'))`

### 1.2 Regla de Resolución de Identificadores (No Adivinar)
* En Go backend (`internal/identity`), la sesión autenticada inyecta en el contexto `Principal{AccountID: string, SessionID: string, Role: string}`.
* En los handlers HTTP (`internal/spaces/transport/http/handler.go`), `principal.AccountID` se pasa como parámetro `owner` al repositorio y servicios de espacios.
* En la base de datos:
  $$\text{usuario.id} \equiv \text{perfil_usuario.usuario_id} \equiv \text{espacio.propietario_id}$$
  **Conclusión:** Comparten exactamente el mismo UUID. No existe una tabla o entidad separada `anfitrion_id`; `accountId` ES el `usuario.id` y ES el `propietario_id`.

### 1.3 Asignación Automática de Rol Anfitrión
* Migración `V000074__first_business_action_roles.sql:1-18` instala la tabla y regla `rol_usuario_evento`.
* La membresía de arrendador/arrendatario se gana por la primera acción comercial completada: el primer borrador de espacio (`accion = 'primer_borrador_espacio'`) asigna el rol `arrendador`.

### 1.4 Múltiples Espacios por Usuario
* En la base de datos no existe restricción de unicidad para `propietario_id` en `public.espacio`.
* La API expone `GET /api/v1/spaces` (`cmd/api/main.go:317`, `internal/spaces/service.go:50`), la cual invoca `repo.ListOwn(ctx, owner)` y retorna el listado completo de espacios/borradores de la cuenta.

---

## 2. Taxonomía Real: Las 9 Categorías Canónicas en Base de Datos y Live API

Fuente canónica: `public.categoria_espacio` ([V000005](file:///home/nandev/Documentos/projects/espaciGo/db/migrations/V000005__m04_space_drafts.sql#L9-L18), [V000067](file:///home/nandev/Documentos/projects/espaciGo/db/migrations/V000067__inmuebles_residential_rentals.sql#L3-L4)) y `public.categoria_perfil_atributos` ([V000006](file:///home/nandev/Documentos/projects/espaciGo/db/migrations/V000006__m04_extensible_category_attributes.sql#L19-L28), [V000067](file:///home/nandev/Documentos/projects/espaciGo/db/migrations/V000067__inmuebles_residential_rentals.sql#L6-L50)), comprobadas en runtime `GET /api/v1/public/space-categories` (HTTP 200). En total son exactamente **9 categorías** (no confundir con fixtures sintéticos parciales de 4):

| Orden | Código (`codigo`) | Nombre Canónico | Atributos Específicos del Perfil (Código, Tipo, Requerido, Opciones/Límites) | Particularidades / Restricciones |
|---|---|---|---|---|
| 1 | `oficina` | Oficina | 1. `puestos_trabajo` (int, min: 1)<br>2. `mobiliario_incluido` (bool)<br>3. `tipo_uso_oficina` (enum: `privada`, `compartida`)<br>4. `escritorios` (int, min: 0)<br>5. `sillas_trabajo` (int, min: 0)<br>6. `salas_reunion_disponibles` (int, min: 0)<br>7. `impresora_disponible` (bool)<br>8. `wifi` (bool)<br>9. `energia_electrica` (bool)<br>10. `banos_disponibles` (int, min: 0)<br>11. `climatizacion` (enum_list: `aire_acondicionado`, `calefaccion`, `ventilacion_mecanica`) | Espacio de trabajo corporativo. |
| 2 | `sala_multiproposito` | Sala o espacio multipropósito | 1. `disposiciones_admitidas` (enum_list: `auditorio`, `mesa_reunion`, `aula`, `espacio_libre`)<br>2. `sillas_disponibles` (int, min: 0)<br>3. `proyector` (bool)<br>4. `mesas_disponibles` (int, min: 0)<br>5. `pantalla` (bool)<br>6. `pizarra` (bool)<br>7. `videoconferencia` (bool)<br>8. `equipo_audio` (bool)<br>9. `wifi` (bool)<br>10. `energia_electrica` (bool)<br>11. `banos_disponibles` (int, min: 0)<br>12. `climatizacion` (enum_list) | Flexibilidad de distribución. |
| 3 | `bodega` | Bodega | 1. `altura_util_m` (num, m)<br>2. `volumen_util_m3` (num, m³)<br>3. `acceso_carga` (enum: `peatonal`, `vehiculo_liviano`, `furgon`, `camion`)<br>4. `ancho_acceso_m` (num, m)<br>5. `altura_acceso_m` (num, m)<br>6. `carro_carga_disponible` (bool)<br>7. `muelle_carga` (bool)<br>8. `tipo_almacenamiento` (enum: `seco`, `temperatura_controlada`, `refrigerado`) | Control logístico y dimensiones útiles. |
| 4 | `estacionamiento` | Estacionamiento | 1. `largo_plaza_m` (num, m)<br>2. `ancho_plaza_m` (num, m)<br>3. `vehiculos_admitidos` (enum_list: `motocicleta`, `automovil`, `suv`, `furgon`)<br>4. `altura_maxima_m` (num, m)<br>5. `techado` (bool)<br>6. `tipo_acceso` (enum: `abierto`, `porton_manual`, `porton_automatico`)<br>7. `carga_electrica` (bool) | Vehicular y estacionamiento. |
| 5 | `local_flexible` | Local flexible o pop-up | 1. `acceso_nivel_calle` (bool)<br>2. `vitrina` (bool)<br>3. `frente_m` (num, m)<br>4. `mostradores` (int)<br>5. `probadores` (int)<br>6. `bodega_apoyo_m2` (num, m²)<br>7. `iluminacion_exhibicion` (bool)<br>8. `wifi` (bool)<br>9. `energia_electrica` (bool)<br>10. `banos_disponibles` (int)<br>11. `climatizacion` (enum_list) | Comercial minorista temporal. |
| 6 | `stand` | Stand o módulo comercial | 1. `ancho_stand_m` (num, m)<br>2. `fondo_stand_m` (num, m)<br>3. `montaje_incluido` (bool)<br>4. `lados_abiertos` (int, 1 a 4)<br>5. `altura_maxima_m` (num, m)<br>6. `potencia_disponible_w` (int)<br>7. `paneleria` (bool)<br>8. `mesas_incluidas` (int)<br>9. `sillas_incluidas` (int)<br>10. `energia_electrica` (bool)<br>11. `wifi` (bool) | Feria o galería comercial. |
| 7 | `quincho` | Quincho o espacio gastronómico | 1. `tipo_parrilla` (enum: `carbon`, `gas`, `electrica`, `lena`, `sin_parrilla`)<br>2. `techado` (bool)<br>3. `asientos_disponibles` (int)<br>4. `parrillas_disponibles` (int)<br>5. `mesas_disponibles` (int)<br>6. `lavaplatos` (bool)<br>7. `refrigerador` (bool)<br>8. `utensilios_incluidos` (bool)<br>9. `agua_potable` (bool)<br>10. `energia_electrica` (bool)<br>11. `banos_disponibles` (int) | **Regla cruzada Go:** Si `tipo_parrilla = 'sin_parrilla'`, `parrillas_disponibles` debe ser 0 ([domain.go:309-318](file:///home/nandev/Documentos/projects/espaciGo/internal/spaces/domain.go#L309-L318)). |
| 8 | `parcela_eventos` | Parcela para eventos | 1. `superficie_exterior_util_m2` (num)<br>2. `superficie_cubierta_util_m2` (num)<br>3. `estacionamientos_disponibles` (int)<br>4. `salon_cubierto` (bool)<br>5. `cocina_disponible` (bool)<br>6. `quincho_disponible` (bool)<br>7. `piscina_disponible` (bool)<br>8. `acceso_bus` (bool)<br>9. `agua_potable` (bool)<br>10. `energia_electrica` (bool)<br>11. `banos_disponibles` (int) | **Regla Go:** Las superficies exterior y cubierta no pueden superar el `area_m2` total del espacio ([domain.go:358-360](file:///home/nandev/Documentos/projects/espaciGo/internal/spaces/domain.go#L358-L360)). |
| 9 | `inmuebles` | Inmuebles | 34 atributos estructurados ([V000067:13-48](file:///home/nandev/Documentos/projects/espaciGo/db/migrations/V000067__inmuebles_residential_rentals.sql#L13-L48)):<br>- **Subtipos/modalidad:** `tipo_inmueble` (req: true, enum: `Casa`, `Departamento`, `Pieza o habitación`, `Estudio o loft`, `Cabaña`, `Casa anexa`, `Dúplex`, `Otro`), `tipo_de_espacio` (req: true, enum: `Inmueble completo`, `Habitación privada`, `Habitación compartida`, `Cama en habitación compartida`).<br>- **Dimensiones:** `superficie_construida_m2`, `superficie_terreno_m2`.<br>- **Distribución:** `dormitorios`, `camas`, `banos_completos`, `medios_banos`, `privacidad_bano` (`Privado en suite`, `Privado de uso exclusivo`, `Compartido`, `No disponible`), `cocina` (`Completa`, `Kitchenette`, `Compartida`, `No disponible`), `piso`, `pisos_edificio`.<br>- **Comodidades:** `amoblado`, `estacionamientos`, `bodegas`, `ascensor`, `balcon`, `terraza`, `patio_o_jardin`, `piscina`, `wifi`, `calefaccion`, `aire_acondicionado`, `agua_caliente`, `lavadora`, `secadora`, `refrigerador`.<br>- **Servicios incluidos:** `servicios_incluidos` (enum_list: `Agua`, `Electricidad`, `Gas`, `Internet`, `Gastos comunes`, `Limpieza`, `Ropa de cama`).<br>- **Reglas de convivencia:** `mascotas` (`Permitidas`, `No permitidas`, `Consultar`), `visitas` (`Permitidas`, `Con restricciones`, `No permitidas`), `fumadores` (bool), `fiestas_o_reuniones` (bool).<br>- **Estadía:** `duracion_minima_unidades`, `duracion_maxima_unidades`. | **Inmuebles no es una categoría hardcoded separada del backend:** Extiende `categoria_perfil_atributos`. Los subtipos habitacionales (`tipo_inmueble`, `tipo_de_espacio`) se modelan mediante sus atributos versionados en el JSON Schema. |

---

## 3. Modelo de Imágenes, Galería y Subida de Archivos Reales

### 3.1 Modelo Relacional de Media Real (`V000068__img_01b_media_schema.sql`)
* **Tabla `public.media_asset`:**
  - Almacena activos binarios reales en Google Cloud Storage.
  - `idempotency_key text NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 8 AND 128)`.
  - `proposito text NOT NULL CHECK (proposito IN ('espacio', 'perfil'))`.
  - `estado text NOT NULL CHECK (estado IN ('pending', 'ready', 'deleting', 'deleted'))`.
  - Unicidad de idempotencia: `UNIQUE INDEX media_asset_space_idempotency_uq ON public.media_asset(propietario_id, espacio_id, idempotency_key) WHERE proposito='espacio'`.
* **Límites de Galería:**
  - En `V000068:223-234` el trigger `img01b_enforce_gallery_slot()` controla la ocupación concurrente sumando activos sintéticos y reales en estado `pending`/`ready`.
  - **Corrección del padre contra fuente:** el límite actual es **10**, no 30: `internal/media/domain.go:12-15` declara `MaxInputBytes = 10 << 20` y `MaxSpaceMedia = 10`; `internal/gallery/domain.go:13` declara 10. V68 contiene `position BETWEEN 1 AND 10` (línea 120) y `occupied >= 10` (línea 232). Ampliar a 30 requiere delta SQL y Go, además de UI/OpenAPI. El handler admite 1 MiB extra de envoltura multipart; no convierte el límite por archivo en 15 MiB.
* **Ordenación y Portada:**
  - `public.gallery_item_order`: Mapea `espacio_id`, `position` (smallint) y el activo sintético o real.
  - `public.espacio.portada_foto_id`: Almacena la portada seleccionada.
  - Endpoints montados: `POST /api/v1/spaces/{spaceID}/gallery` con multipart `image`; `PUT /api/v1/spaces/{spaceID}/cover`; rutas de actualización, borrado y variantes bajo `/gallery/{photoID}` (`cmd/api/main.go:335-340`). No existe un montaje `/media/upload` en este corte. El mensaje de error de portada aún menciona «sintética» aunque el handler también admite media real; corregir ese copy durante la integración.

---

## 4. Requisitos y Validaciones Estrictas para Publicar (`activa`)

En `internal/adapters/postgres/spaces/repository.go:369-448` (`SetPublicationState`), la transición a estado `activa` ejecuta una serie de compuertas estrictas:
1. **Verificación de Zona Horaria (IANA):** `validIANAZone(draft.TimeZone)` obligatorio; si falta o es inválida $\rightarrow$ error `spaces.ErrTimeZoneRequired`.
2. **Fixture Sintético:** Si el espacio tiene un fixture local de ensayo habilitado (`reserva_ensayo_local_fixture`), no puede publicarse $\rightarrow$ error `spaces.ErrEnabledFixture`.
3. **Máquina de Estados:**
   - Solo se puede activar desde `borrador` u `oculta` $\rightarrow$ de lo contrario error `spaces.ErrPublicationConflict`.
4. **Verificación de Identidad (KYC) Obligatoria:**
   - Consulta `verificationdb.ListSyntheticEligibility(ctx, owner)`. Debe existir elegibilidad de tipo `kyc` con valor `true` $\rightarrow$ si no, falla con `spaces.ErrEligibilityRequired` (HTTP 403).
5. **Historial de Publicación:** Inserta registro de auditoría append-only en `public.espacio_publicacion_historial_local`.

---

## 5. Metadatos de Infraestructura Local y Gestión de Migraciones

### 5.1 Distinción de Instancias Locales (2 Targets)
Se identifican dos componentes locales que no deben confundirse ni equipararse:
1. **PostgreSQL Nativo Local:** Listener en `127.0.0.1:5432`.
2. **PostgreSQL Interno Docker:** Contenedor `espacigo_localbackend_20261024-database-1` (puerto 5432 interno dentro de la red docker, sin puerto host mapeado por defecto a 127.0.0.1). Su API asociada corre en `espacigo_localbackend_20261024-backend-1` (puerto `18081`).

**Regla Operativa:** No elegir Docker simplemente por conveniencia ni crear réplicas. El entorno debe registrar explícitamente ambos targets y ejecutar un preflight vinculante antes de correr cualquier migración.

### 5.2 Capacidad de Planificación en CLI (`cmd/dbmigrate`)
* En `cmd/dbmigrate/main.go:15,25-45`, el comando **sí implementa** la bandera `-plan-only`:
  ```bash
  go run ./cmd/dbmigrate -dir db/migrations -plan-only
  ```
* Calcula el conjunto de migraciones aplicadas (`applied`) vs pendientes (`pending`) sin ejecutar transacciones de cambio (`planOnly == true` imprime el plan y retorna exit 0). La conexión se proporciona mediante las entradas privadas soportadas por `loadConnectionSettings`; **no existe una bandera `-dsn`** y no deben ponerse contraseñas en argumentos ni en el ejemplo.

### 5.3 Túnel SSH Cloud SQL Read-Only Seguro
Para lecturas de auditoría directa sobre GCP Cloud SQL:
* Host bastión: `espacigo-dms-bastion-v67` (`southamerica-west1-a`, Compute ID: `5282317084229207114`).
* IP Privada Cloud SQL confirmada: `10.189.0.15`.
* Puerto local: `127.0.0.1:15432` (verificar previamente que se encuentre libre de escuchas).
* Seguridad de credenciales: No imprimir secretos en variables de entorno o argumentos CLI; archivo `.pgpass` con permisos `0600` en directorio efímero `0700`; sesión estricta en solo lectura con `PGOPTIONS='-c default_transaction_read_only=on -c statement_timeout=30s'`. Cerrar inmediatamente el reenvío de puertos tras la consulta.

---

## 6. Matriz de Brechas: OpenAPI vs Go Backend vs DB

| Aspecto | OpenAPI Documentado (`openapi.yaml`) | Go Backend Montado (`cmd/api/main.go`) | PostgreSQL Real | Brecha Detectada |
|---|---|---|---|---|
| **Categorías** | 9 categorías declaradas | 9 categorías servidas en `/api/v1/public/space-categories` | 9 categorías en `categoria_espacio` y perfiles | **Alineado.** Inmuebles contiene 34 atributos versionados para soportar subtipos (casa, depto, pieza). |
| **Galería / Media** | Ya declara multipart y variantes en `/gallery` | Handler real y sintético en `/gallery`, portada en `/cover` | `media_asset`, `gallery_item_order`, `space_media`; máximo 10 actual | **Brecha de integración y ampliación:** la UI usa el botón sintético; el adaptador ya admite FormData pero descarta variantes en su mapper. No inventar rutas `/media/upload`. |
| **ETag / If-Match** | Hay cabeceras enviadas opcionalmente por el adaptador | El handler de espacios no lee If-Match | No se acreditó CAS/versionado de contenido por esa cabecera | **Brecha funcional:** implementar y probar concurrencia extremo a extremo; enviar una cabecera ignorada no evita sobrescrituras. No dar por garantizado un 409 antes de cerrar el contrato. |
| **Validación Publicación** | Requiere datos completos | Requiere validación IANA timezone y KYC anfitrión aprobado | Comprobación del repositorio contra elegibilidad actual | **Compuerta existente:** Go consulta un motor sintético. No convertirlo en KYC real ni saltarlo desde UI; publicación comercial requiere un corte explícito. |

## 7. Contraste SQL específico posterior

Sesión delegada registrada `2026-10-27T03:59:08-03:00`, destino `espacigo_dms_stage_20261020`, usuario `espacigo_migrator_login`, `transaction_read_only=on`, TLS activo. Snapshot sanitizado contrastado por el padre: `.local/testing/espacios-plan/db-category-snapshot.json` (0600, SHA256 `6f59753b2d82b6e54a0903fd48166abd6d5b410e468af054c33d8f031f79e1d9`).

- Función real `public.img01b_enforce_gallery_slot`: `occupied >= 10`; no 30.
- `gallery_item_order_position_check`: posiciones 1 a 10; unicidad por espacio/posición diferible y exactamente un origen de foto por entrada.
- Lectura de filas de `categoria_perfil_atributos`: **permission denied** para el lector aprobado. No se elevaron privilegios. La matriz de atributos anterior es evidencia del código/seed, no una confirmación de todos sus valores/versiones actuales en Cloud SQL.
- El padre verificó que el puerto 15432 quedó libre. No se aplicaron SQL ni grants.
