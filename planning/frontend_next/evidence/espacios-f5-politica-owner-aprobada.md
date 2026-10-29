# F5 — Política documental y verificaciones aprobada por el owner

**Fecha de aprobación:** 2026-10-28  
**Evidencia de aprobación:** registrada el 2026-10-28 sobre `main` en `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`. **Implementación local revisada en esta continuación:** `/home/nandev/Documentos/projects/espaciGo`, rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`, HEAD `c50a2ef22dd2e3fb3134d0cb8bfe248fcedb7f6d`; los cambios F5 de esta continuación permanecen sin commit.
**Estado:** requisitos funcionales aprobados; implementación e integración F5 todavía parciales.  
**Fuente:** política y autorización expresa aportadas por el owner en esta conversación. Los plazos indicados son decisiones operativas, no conclusiones legales.

## Decisiones aprobadas

### Cuenta, actividad y verificaciones

- Una cuenta de usuario única puede publicar espacios y arrendar. No se selecciona un rol exclusivo durante el registro.
- Las etiquetas «Arrendador» y «Arrendatario» se derivan de actividad y pueden coexistir. No conceden acceso documental ni datos privados.
- Hay dos verificaciones independientes: identidad de la cuenta y cada espacio individual. Una no concede automáticamente la otra.
- La verificación de espacio es voluntaria y sirve solo para el distintivo de ese espacio. La falta de antecedentes o distintivo de espacio no bloquea publicar si se satisfacen las demás condiciones.
- La identidad documental puede dejar explorar, preparar publicaciones y publicar. Exigir identidad verificada a ambas partes antes de confirmar una contratación; mantener la autorización al servidor Go en cada operación.
- No prometer seguridad absoluta, ausencia de fraude, habitabilidad, seguridad estructural ni cumplimiento normativo no revisado.

### Documentos y revisión

- Identidad: cédula vigente o pasaporte vigente, solo caras/páginas necesarias. No solicitar datos bancarios, domicilio, ingresos ni archivos adicionales por defecto. Revisión manual por personal autorizado; no reconocimiento facial automatizado ni almacenamiento biométrico en esta versión. El distintivo comunica «identidad contrastada mediante documentación» y no prueba por sí solo que quien subió el archivo sea su titular.
- Espacio, asociado a un espacio y cuenta administradora: antecedente de propiedad, autorización del propietario, otro antecedente habilitante o permiso/certificado pertinente. Se presenta la alternativa aplicable; no se exigen todos. El reglamento del recinto por sí solo no prueba facultad para ofrecerlo. Se requiere identidad de cuenta verificada antes de conceder el distintivo del espacio.
- Separar capacidades de revisión de identidad y de espacios; impedir autoaprobación. Go autoriza cada operación por cuenta/recurso; los prefijos de storage no son ACL. Otros usuarios solo ven distintivos y su alcance, nunca archivos.

### Archivos, inspección y estados

| Uso | Formatos | Tamaño y cuota | Páginas PDF |
|---|---|---|---|
| Identidad | JPEG, PNG, PDF | 10 MiB por archivo; hasta 3 por solicitud | hasta 5 |
| Espacio | JPEG, PNG, PDF | 10 MiB por archivo; hasta 10 por solicitud | hasta 20 |

Permitir una solicitud abierta de identidad por cuenta y una de espacio por espacio; reemplazos y pendientes consumen cuota. Validar contenido real, rechazar PDF cifrado, contenido activo, adjuntos incrustados y archivos corruptos; retirar metadatos innecesarios de imágenes, incluido GPS. Mantener documentos separados del pipeline/galería WebP.

Todo archivo inicia en cuarentena. Estados técnicos separados: `pendiente`, `limpio`, `bloqueado`, `error`; una falla del scanner mantiene el archivo bloqueado. Estados de solicitud: `borrador`, `enviada`, `en_revision`, `requiere_antecedentes`, `aprobada`, `rechazada`, `vencida`, `revocada`. Limpio no equivale a aprobación y una decisión no habilita descargar un archivo bloqueado. Scanner privado/controlado por el proyecto, firmas actualizadas, sin enviar documentos a servicios públicos de análisis.

### Vigencia, auditoría y retención

- Identidad: nueva revisión máximo a 24 meses, antes si vence el documento o cambia un dato esencial. Espacio: revisión máximo a 12 meses o antes si vence el antecedente. Cambio de titular/administrador exige nueva revisión; cambios relevantes de ubicación/identidad del espacio suspenden distintivo; cambios ordinarios de precio, disponibilidad o descripción no lo quitan.
- Registrar alcance, fecha, responsable y vencimiento. Una verificación de espacio vencida/revocada retira el distintivo, pero no la publicación salvo causa independiente.
- Cifrado en tránsito y reposo; descargas privadas autorizadas por Go con `private`, `no-store`, `nosniff` y `Content-Disposition: attachment`. Sin enlaces permanentes ni PII/nombre original en claves. Auditar actor, fecha, objeto, resultado y motivo; nunca contenido, tokens ni enlaces privados.
- Aplicar los plazos aprobados: borrador abandonado 24 h (respetando cargas activas); malicioso bloqueado y eliminado hasta 24 h; error de inspección con reintentos limitados y eliminación a 7 días; subsanación sin respuesta cierra tras 30 días y elimina archivos durante los siguientes 30; documento de identidad tras decisión final se elimina a 30 días y solo queda constancia mínima; documento de espacio aprobado se conserva durante vigencia y se elimina a 30 días del vencimiento/revocación/sustitución; rechazado/sustituido se elimina a 30 días; auditoría mínima 12 meses desde cierre/fin de vigencia sin contenido documental. Excepción anticipada exige motivo, responsable y fecha de revisión.
- La eliminación debe contemplar versiones, soft delete y respaldos. No prometer desaparición instantánea de todas las copias; cualquier garantía exacta debe corresponder al almacenamiento efectivamente aprobado y probado.

### Aclaraciones del owner — 2026-10-29 (prevalecen sobre notas anteriores)

- Las capacidades `review_identity` y `review_space` son distintas y requieren asignación explícita por persona. La implementación local autoriza por actor autenticado + grant con `ReviewerID` igual al actor + capacidad + scope exacto; mapea identidad al `SubjectID` del caso y espacio al `SpaceID`. No crea roster ni persistencia de grants.
- El administrador vigente del espacio no puede revisar ni revocar un caso de ese espacio, aunque `SubjectID` histórico sea otra persona. Un cambio de titular/administrador exige un caso nuevo; el caso previo no se transfiere.
- El titular vigente solo puede descargar archivos `limpio` mientras el caso siga abierto. Al cerrar el caso, la descarga termina. Soporte no recibe acceso a binarios por defecto.
- No está aprobado que una persona tenga ambas capacidades ni existe todavía roster/scope administrativo persistido. El servicio local rechaza grants duales y solo acepta scope por recurso explícito; no inferir roles más amplios.
- El límite de firmas de 48 horas se usa únicamente en la prueba local configurable. La política productiva de antigüedad de firmas sigue pendiente.

## Cambio de contrato Go requerido

El flujo de publicación no debe exigir la elegibilidad KYC sintética vigente que hoy verifica `internal/adapters/postgres/spaces.Repository.SetPublicationState`; se conservan zona IANA, estado válido, ownership, suppression/administrative blocks y conflicto de fixture. El flujo de contratación conserva la comprobación de identidad para ambas partes y debe alinearse a esta verificación de identidad aprobada, no a documentos de espacio. La prueba actual que exige KYC para publicar se debe sustituir por prueba de publicación sin identidad/documentos y reserva bloqueada hasta verificar ambas partes.

No confundir «identidad verificada» de esta política con documentación de espacio. La clasificación/decisión de si la verificación KYC sintética existente es una fuente válida para el nuevo distintivo de identidad debe quedar explícita en el contrato; no autoaprobar ni trasladar estados mediante migración sin una regla de reconciliación aprobada.

## Autorización y límites operativos

El owner autoriza diseño e implementación local de contratos API, permisos, UI, storage privado, scanner, migraciones incrementales necesarias y pruebas con archivos sintéticos. Las pruebas funcionales deben usar un entorno aislado identificado.

Esta autorización **no** permite modificar PostgreSQL persistente, crear DB/roles/containers/volúmenes por inferencia, crear/modificar buckets o IAM/Terraform, desplegar, ni usar cuentas o documentos reales. No ejecutar migraciones hasta identificar y aprobar expresamente un target aislado **de F5** y verificarlo; la autorización/DB de F6 no cubre F5. No iniciar `cmd/api` sin verificar un perfil sin workers mutadores y storage filesystem privado.

El preflight inicial del 2026-10-28 no halló `clamscan` ni `clamav-daemon` en `PATH`; esa limitación de herramienta quedó superada solo para pruebas sintéticas locales por el alcance ClamAV aprobado y la evidencia actualizada en `espacios-f5-clamav-prueba-local-20261029.md`. Esto no habilita documentos reales, producción, bases persistentes ni servicios cloud.

La auditoría previa `espacios-f5-documentos-contrato-no-operativo.md` refleja el estado anterior a esta aprobación y queda como histórico; no contradice la política recién aprobada.

## Próximas rebanadas

1. Congelar un ADR/contrato que mapee verificación de identidad y verificación por espacio; revisar las rutas/roles actuales contra el contrato y no reutilizar almacenamiento de medios.
2. Implementar RED→GREEN del gate Go: publicación permitida sin KYC de espacio/identidad documental; contratación rechazada hasta que ambas cuentas tengan identidad vigente. Mantener aislado el test DB; no aplicar DDL al PostgreSQL persistente.
3. Diseñar tipos/repositorio de solicitudes y archivos, autorización de revisor no propio, scanner fail-closed y retención, con pruebas unitarias sintéticas; añadir la migración incremental como artefacto revisable, sin aplicarla todavía.
4. Preparar UI de solicitudes/estados y distintivos sin mostrar archivos a terceros, más contratos OpenAPI y BFF.
5. Identificar/autorización explícita del entorno aislado existente y motor de inspección privado; entonces ejecutar el flujo real completo y verificar retención/eliminación también en copias/versiones.

## Criterio de cierre

F5 cierra solo al demostrar el recorrido real de envío, cuarentena/inspección, revisión manual, distintivo independiente, revocación/vencimiento y eliminación, con aislamiento entre usuarios/espacios y auditoría mínima. Mocks y pruebas unitarias documentan cortes, pero no sustituyen esa integración.
