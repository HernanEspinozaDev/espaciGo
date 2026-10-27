export type $Read<T> = {
  readonly $read: T;
};
export type $Write<T> = {
  readonly $write: T;
};
export type Readable<T> =
  T extends $Write<any>
    ? never
    : T extends $Read<infer U>
      ? Readable<U>
      : T extends (infer E)[]
        ? Readable<E>[]
        : T extends object
          ? {
              [
                K in keyof T as NonNullable<T[K]> extends $Write<any>
                  ? never
                  : K
              ]: Readable<T[K]>;
            }
          : T;
export type Writable<T> =
  T extends $Read<any>
    ? never
    : T extends $Write<infer U>
      ? Writable<U>
      : T extends (infer E)[]
        ? Writable<E>[]
        : T extends object
          ? {
              [
                K in keyof T as NonNullable<T[K]> extends $Read<any> ? never : K
              ]: Writable<T[K]>;
            } & {
              [
                K in keyof T as NonNullable<T[K]> extends $Read<any> ? K : never
              ]?: never;
            }
          : T;
export interface paths {
  "/admin/global/me": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Confirmar capacidades vigentes del administrador global */
    get: operations["getGlobalAdminMe"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/overview": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Obtener conteos reales de cuentas para administración global */
    get: operations["getGlobalAdminOverview"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/kpis": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Obtener indicadores operativos persistidos por período
     * @description Requiere kpi.read. Zona IANA requerida; intervalo de máximo 31 días. Incluye fuente/definición, instantánea generada y montos CLP como texto exacto. Los estados vigentes no reconstruyen historia ni son DAU; finanzas sintéticas excluyen garantías.
     */
    get: operations["getGlobalAdminKPIs"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/audit-events": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Consultar auditoría minimizada con filtros actor/objetivo separados
     * @description Requiere audit.read. Ventana fija de máximo 31 días; cursor keyset fijado al corte temporal y a filtros/tamaño normalizados. Eventos históricos no se reidentifican; los objetivos sin campo explícito permanecen nulos.
     */
    get: operations["listGlobalAdminAuditEvents"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/audit-events/export": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Exportar auditoría global minimizada en JSON o CSV
     * @description Requiere audit.export. Filtros y ventana son idénticos a la consulta; máximo 10.000 filas y 31 días; falla sin archivo parcial. El instante de corte precede la auditoría de exportación. CSV protege fórmulas y respeta quoting estándar.
     */
    get: operations["exportGlobalAdminAuditEvents"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/accounts": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Listar cuentas con filtros allowlisted y cursor keyset */
    get: operations["listGlobalAdminAccounts"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/accounts/{accountID}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Consultar proyección mínima de una cuenta */
    get: operations["getGlobalAdminAccount"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/accounts/{accountID}/profile": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    /**
     * Actualizar campos allowlisted del perfil con versión esperada
     * @description El correo verificado no se cambia aquí. Requiere sesión creada en los últimos 10 minutos y motivo permitido.
     */
    patch: operations["updateGlobalAdminAccountProfile"];
    trace?: never;
  };
  "/admin/global/accounts/{accountID}/roles/{role}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    /**
     * Establecer de forma idempotente un rol ordinario de marketplace
     * @description Solo admite arrendatario o arrendador; nunca concede administrador ni administrador global.
     */
    put: operations["setGlobalAdminMarketplaceRole"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/accounts/{accountID}/legacy-administrator": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    /**
     * Establecer de forma idempotente el rol legado de administrador
     * @description Requiere capacidad admin_access.manage y sesión reciente. Concede solo el rol administrador legado; nunca altera ni concede la membresía administrador_global. Cada transición, incluida una repetición idempotente, queda auditada.
     */
    put: operations["setGlobalAdminLegacyAdministratorRole"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/accounts/{accountID}/sessions/revoke": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** Revocar todas las sesiones activas de otra cuenta */
    post: operations["revokeGlobalAdminAccountSessions"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/accounts/{accountID}/block": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** Bloquear o desbloquear una cuenta con revocación de sesiones */
    post: operations["setGlobalAdminAccountBlocked"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/accounts/{accountID}/resources/{resourceKind}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get: operations["listGlobalAdminAccountResources"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/accounts/{accountID}/resources/{resourceKind}/{resourceID}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get: operations["getGlobalAdminAccountResource"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/publications": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Consultar todas las publicaciones con propietario y visibilidad pública efectiva */
    get: operations["listGlobalAdminPublications"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/publications/{publicationID}/state": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /**
     * Publicar u ocultar mediante las reglas vigentes de catálogo y KYC
     * @description No cambia propietario ni ocupaciones. No retira físicamente publicaciones y conserva el historial append-only.
     */
    post: operations["changeGlobalAdminPublicationState"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/publications/{publicationID}/content": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    /**
     * Corregir campos allowlisted, tarifa versionada o portada publicada
     * @description Solo borradores, publicaciones activas u ocultas. Valida contenido del dominio y versión esperada; mantiene propietario y snapshots de reservas.
     */
    patch: operations["updateGlobalAdminPublicationContent"];
    trace?: never;
  };
  "/admin/global/publications/{publicationID}/retire": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** Retirar publicación con retención y obligación controladas */
    post: operations["retireGlobalAdminPublication"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/publications/{publicationID}/schedule": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Consultar el horario semanal de una publicación por hora
     * @description Requiere permiso administrativo de lectura y owner_id. La lectura genera auditoría y no revela horarios de otros propietarios a usuarios normales.
     */
    get: operations["getGlobalAdminPublicationSchedule"];
    /**
     * Guardar el horario semanal mediante revisión administrativa
     * @description Solo publicaciones no eliminadas con tarifa por hora. Exige sesión reciente, owner_id, versión actual y motivo permitido. No crea cotizaciones, reservas ni fixture de participantes.
     */
    put: operations["updateGlobalAdminPublicationSchedule"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/publications/{publicationID}/gallery": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get: operations["listGlobalAdminPublicationGallery"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/publications/{publicationID}/gallery/{photoID}/visibility": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch: operations["setGlobalAdminPublicationPhotoVisibility"];
    trace?: never;
  };
  "/admin/global/publications/{publicationID}/gallery/{photoID}/cover": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    post: operations["selectGlobalAdminPublicationCover"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/global/publications/{publicationID}/gallery/{photoID}/remove": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    post: operations["removeGlobalAdminPublicationPhoto"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/public/spaces": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Explorar publicaciones públicas elegibles */
    get: operations["listPublicSpaces"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/public/space-categories/{code}/filters": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Listar únicamente los atributos públicos allowlisted para una versión de perfil */
    get: operations["getPublicSpaceCategoryFilters"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/public/locations": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Buscar nombres y códigos públicos DPA región/comuna */
    get: operations["listPublicLocations"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/public/spaces/{spaceID}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Consultar publicación pública elegible */
    get: operations["getPublicSpace"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/public/spaces/{spaceID}/images/{imageID}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Leer una imagen declarada pública en una publicación vigente
     * @description Resuelve la visibilidad y asociación en cada solicitud; las fotos existentes siguen privadas hasta su declaración explícita.
     */
    get: operations["getPublicSpaceImage"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/public/spaces/{spaceID}/images/{imageID}/variants/{variant}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Revalida elegibilidad pública del espacio, visibilidad explícita de la foto y estado ready. Solo display/thumbnail WebP; responde con caché de 60 segundos para limitar la ventana de revocación y no revela claves internas. */
    get: operations["getPublishedSpaceMediaVariant"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/public/space-categories": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Listar categorías activas */
    get: operations["listPublicSpaceCategories"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceID}/reviews": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Reseñas sintéticas de una publicación activa */
    get: operations["listActiveSpaceReviews"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/reviews": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Consultar reseñas de una reserva propia */
    get: operations["listReservationReviews"];
    put?: never;
    /** Crear una reseña sintética inmutable tras finalizar o cerrar por liquidación confirmada */
    post: operations["createReservationReview"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/reviews/{reviewID}/report": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** Reportar una reseña del espacio propio */
    post: operations["reportSpaceReview"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/reputation/me": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Consultar reputación propia en una vista autenticada */
    get: operations["getOwnRenterReputation"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/review-reports": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Listar reportes pendientes (administrador) */
    get: operations["listPendingReviewReports"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/review-reports/{reportID}/{action}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** Desestimar u ocultar con motivo (administrador) */
    post: operations["moderateReviewReport"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/notices": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** Listar avisos con fallo terminal (administrador) */
    get: operations["listTerminalLocalNotices"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/notices/{noticeID}/reopen": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** Reabrir un ciclo terminal con motivo estructurado */
    post: operations["reopenLocalNotice"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/terms": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Versiones sintéticas para CU-01; no textos legales productivos. */
    get: operations["listLocalTerms"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/register": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description CU-01/RQF-001–007/186–187/213: crea una cuenta sin asignar roles comerciales ni una preferencia de actividad. Registra de forma atómica las versiones actuales de Términos y condiciones y Política de privacidad. Si falla la entrega del correo tras guardar la cuenta, devuelve 202 y permite reemitir la verificación sin repetir el registro. */
    post: operations["registerAccount"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/verification/reissue": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description CU-02/RQF-188; TTL 24 h, invalida token anterior; 3 emisiones/cuenta/h más cuota IP local. */
    post: operations["reissueVerification"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/verification": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description CU-02/RQF-008–010; ID y secreto en cuerpo; 5 fallos/token, cuota IP. Consumo/activación atómicos, no acorta bloqueo de login. */
    post: operations["verifyEmail"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/verification/status": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Inspecciona el estado de un enlace de verificación de correo mediante prueba de posesión sin consumirlo. */
    post: operations["getEmailVerificationLinkStatus"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/password/recovery": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description CU-05/RQF-019–020. Siempre responde con el mismo 202 para cuenta elegible/desconocida; cuota por IP puede responder 429. Envío local capturado por Mailpit; token 15 min, reemplazo invalida anterior, máximo 3 emisiones/cuenta/h. */
    post: operations["requestPasswordRecovery"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/password/recovery/consume": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description CU-05/RQF-021–022. Token ID/secreto solo en cuerpo, hash en base de datos, 5 intentos. Cambia clave y revoca sesiones atómicamente. No implementa historial RQF-217 mientras DB02-09 no defina retención. */
    post: operations["consumePasswordRecovery"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/password/recovery/status": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Inspecciona el estado de un enlace de recuperación de contraseña mediante prueba de posesión sin consumirlo. */
    post: operations["getPasswordRecoveryLinkStatus"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/password/change": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description CU-50/RQF-214–218. Requiere Bearer y clave actual, rechaza clave idéntica, invalida todas las sesiones y envía correo local tras commit. Historial RQF-217 y entrega durable/outbox RQF-218 dependen de DB02-09. */
    post: operations["changePassword"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/profile": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Perfil solo del titular autenticado. La foto sintética se administra mediante /profile/photo. */
    get: operations["getOwnProfile"];
    /** @description Nombre visible y teléfono opcional de nueve dígitos según CU-07. */
    put: operations["updateOwnProfile"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/profile/photo": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Metadata de la foto PNG fija generada por Backend; nunca acepta bytes del cliente. */
    get: operations["getOwnSyntheticProfilePhoto"];
    /** @description Sin cuerpo conserva el flujo sintético histórico. Con multipart/form-data y campo image admite JPEG, PNG y WebP estático, aplica límites, orientación y normaliza derivados WebP en almacenamiento privado. Requiere Idempotency-Key y Bearer del titular. */
    put: operations["generateOwnSyntheticProfilePhoto"];
    post?: never;
    delete: operations["removeOwnSyntheticProfilePhoto"];
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/profile/photo/content": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description PNG privado del titular autenticado. */
    get: operations["getOwnSyntheticProfilePhotoContent"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/payout-account": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Referencia ficticia del adaptador local; no contiene datos bancarios. */
    get: operations["getOwnSyntheticPayoutAccount"];
    /** @description Requiere KYC sintético efectivo. No recibe RUT, datos bancarios ni credenciales externas. */
    put: operations["createOrReplaceOwnSyntheticPayoutAccount"];
    post?: never;
    delete: operations["revokeOwnSyntheticPayoutAccount"];
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/rights-requests": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Lista solo solicitudes del titular autenticado. */
    get: operations["listOwnRightsRequests"];
    put?: never;
    /** @description Registra acceso o supresión en_revision sin borrar/desidentificar datos. */
    post: operations["requestDataRight"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/privacy/export": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Devuelve una instantánea consistente y limitada a la cuenta autenticada: correo proporcionado, estado, preferencia inicial, perfil, roles, aceptaciones de términos y solicitudes propias. Excluye hashes, sesiones, tokens, secretos y datos de terceros. No equivale a exportación de todas las áreas del producto. */
    get: operations["exportOwnIdentityData"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/privacy/export/archive": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Devuelve un ZIP descargable versionado con los datos propios actualmente implementados, un manifiesto de secciones/archivos/exclusiones y archivos sintéticos propios disponibles. Requiere sesión vigente y sigue disponible aunque una solicitud de supresión esté bloqueada. No incluye credenciales, tokens, datos personales de otros participantes, mensajes ajenos, payloads de adaptadores ni información administrativa restringida. Cada módulo produce una proyección de lectura acotada; la exportación no modifica datos. */
    get: operations["downloadCompleteOwnDataArchive"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/privacy/suppression-requests/{request_id}/review": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Operación exclusiva del rol administrador. Revisa y persiste obligaciones locales detectables bajo bloqueo de la cuenta. No ejecuta la baja. Las disputas sintéticas abiertas se consultan como obligación. Las retenciones sintéticas locales ratificadas permiten determinar elegibilidad, pero no ejecutan por sí solas la baja. Repetir Idempotency-Key devuelve el mismo resultado. */
    post: operations["reviewLocalSuppressionBlockers"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/privacy/suppression-requests/{request_id}/execute": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Exclusivo de administrador sobre una solicitud del titular. Revalida bajo bloqueo reservas, pagos/devoluciones y disputas abiertas. En la baja elegible aplica privacidad_local_v1 solo a datos sintéticos; preserva hechos requeridos, revoca credenciales y minimiza la cuenta. La respuesta distingue bloqueo, limpieza de archivos pendiente y baja con minimización y retención residual; nunca afirma anonimización o supresión integral. Idempotente por solicitud y clave. El borrado de archivos se recupera desde la cola persistente. */
    post: operations["executeLocalSyntheticSuppression"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/privacy/retention/purge": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Operación administrativa local por lotes. Procesa clases sintéticas con plazos ratificados e independientes: vínculos de reservas terminales, metadata/evidencia de verificación, cotizaciones no convertidas, aceptaciones de términos y solicitudes de derechos. El retiro de archivos privados usa leases recuperables y no elimina metadata antes de confirmar el borrado. No declara supresión integral ni anonimización. */
    post: operations["purgeExpiredLocalPrivacyData"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/credential-notices": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Listar avisos locales con fallo terminal
     * @description Lista avisos de cambio de credenciales en fallo terminal. Solo administradores. No incluye destinatario, contenido SMTP, contraseñas, tokens ni datos personales de la cuenta.
     */
    get: operations["listTerminalLocalCredentialNotices"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/credential-notices/{event_id}/reopen": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /**
     * Reabrir idempotentemente un ciclo de aviso terminal
     * @description Reabre de forma idempotente un evento terminal elegible, iniciando un ciclo de hasta ocho intentos sin reiniciar el total acumulado. Auditoría estructurada y destinatario activo obligatorio. SMTP puede duplicar un correo tras una caída entre aceptación SMTP y confirmación en base de datos; no se garantiza entrega exactamente una vez.
     */
    post: operations["reopenTerminalLocalCredentialNotice"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/privacy/suppression-requests": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Cola mínima, disponible únicamente para administradores autorizados. No incluye datos de cuenta ni terceros; identifica cada solicitud solo por ID y fecha para permitir su evaluación explícita. */
    get: operations["listPendingLocalSuppressionRequests"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/verifications": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Devuelve los casos propios y referencias sintéticas; nunca documentos ni RUT. */
    get: operations["listOwnVerifications"];
    put?: never;
    /** @description Crea un caso kyc/kyb sintético en revisión manual. No valida identidad ni consulta proveedor externo. Idempotencia por usuario/clave. */
    post: operations["startSyntheticVerification"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/verifications/eligibility": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Elegibilidad efectiva derivada de aprobación sintética vigente. Devuelve falso para cuenta inactiva o caso retirado; KYC y KYB no se sustituyen ni modifican roles. El ZIP propio incluye elegibilidad e historial. */
    get: operations["listOwnSyntheticEligibility"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/verifications/{id}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get: operations["getOwnVerification"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/verifications/{id}/history": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get: operations["listOwnVerificationHistory"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/verifications/{id}/retry": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Reintenta solo casos rechazados del mismo titular; indica la corrección fixture compatible con el motivo y exige clave idempotente. */
    post: operations["retryRejectedVerification"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/verifications/{id}/evidence": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
      };
      cookie?: never;
    };
    /** @description Lista metadatos de fixtures sintéticos de un caso propio; no expone rutas ni URLs públicas. */
    get: operations["listOwnSyntheticEvidence"];
    put?: never;
    /** @description Crea un PNG de fixture fijo en almacenamiento local privado. No acepta archivos arbitrarios ni documentos reales. */
    post: operations["uploadSyntheticEvidence"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/verifications/{id}/evidence/{evidenceId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
        evidenceId: string;
      };
      cookie?: never;
    };
    /** @description Descarga inline un fixture propio con autenticación y Cache-Control no-store. */
    get: operations["getOwnSyntheticEvidenceContent"];
    put?: never;
    post?: never;
    /** @description Elimina explícitamente solo el archivo sintético y sus metadatos; no elimina el volumen local. */
    delete: operations["deleteOwnSyntheticEvidence"];
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/verifications/{id}/evidence/{evidenceId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
        evidenceId: string;
      };
      cookie?: never;
    };
    /** @description Descarga el fixture únicamente con el rol administrador de revisión. */
    get: operations["reviewSyntheticEvidenceContent"];
    put?: never;
    post?: never;
    /** @description Limpieza explícita del fixture con permiso de revisión. */
    delete: operations["deleteReviewedSyntheticEvidence"];
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/verifications/{id}/evidence": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Lista metadatos de evidencias fixture de un caso para el rol administrador de revisión. */
    get: operations["listEvidenceForReview"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/verifications": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Solo rol administrador; cola de referencias fixture, sin PII. */
    get: operations["listPendingVerifications"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/verifications/failed": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Solo rol administrador; casos rechazados de fixture para revisar motivo y subsanación, sin PII. */
    get: operations["listRejectedVerifications"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/verifications/{id}/review": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo rol administrador. Rechazo requiere motivo codificado. La aprobación concede solo elegibilidad sintética local del tipo registrado; no acredita identidad real ni modifica roles. El gate de cada operación depende del mapeo de producto vigente. */
    post: operations["reviewSyntheticVerification"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/verifications/{id}/revoke": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo administrador; revoca explícitamente una elegibilidad sintética vigente y registra auditoría. No cancela reservas ni altera historiales. */
    post: operations["revokeSyntheticEligibility"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/login": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description CU-03/04: credencial incorrecta genérica 401, no verificada 403, bloqueo 429. Sesión 8 h absoluta, idle 30 min. Entrada inválida 422 con error.details field/code (email:invalid_email, password:required), sin valores enviados. */
    post: operations["login"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/session": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Consulta solo la sesión del Bearer actual y sus roles/estado; no renueva inactividad. No acepta IDs de terceros ni parámetros de URL. */
    get: operations["currentSession"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/categories": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Lista el catálogo versionado de categorías para una sesión válida; no concede permisos comerciales. */
    get: operations["listSpaceCategories"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/categories/{categoryCode}/attributes": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        categoryCode: string;
      };
      cookie?: never;
    };
    /** @description Entrega el perfil inmutable vigente que describe los campos y validaciones de la categoría. */
    get: operations["getSpaceAttributeProfile"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/categories/{categoryCode}/attributes/{version}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        categoryCode: string;
        version: number;
      };
      cookie?: never;
    };
    /** @description Devuelve una versión específica e inmutable del perfil, para conservar el contrato de borradores existentes. */
    get: operations["getVersionedSpaceAttributeProfile"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Lista exclusivamente espacios propios; borradores, publicaciones activas y espacios ocultos permanecen privados fuera de los recorridos autorizados. */
    get: operations["listOwnSpaces"];
    put?: never;
    /** @description Crea un borrador privado. El propietario se obtiene de la sesión; no hay publicación, reserva ni cobro. */
    post: operations["createOwnSpaceDraft"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Consulta un espacio propio; un recurso ajeno o inexistente responde 404. */
    get: operations["getOwnSpaceDraft"];
    /** @description Reemplaza los campos de un borrador propio; no permite cambios de estado comercial. */
    put: operations["updateOwnSpaceDraft"];
    post?: never;
    /** @description Retira una publicación propia elegible. Revalida reservas no terminales y obligaciones financieras o de disputa bajo los bloqueos de propietario/espacio. Si procede, retira contenido del catálogo, minimiza el registro técnico y encola los archivos privados para limpieza durable. Conserva reservas, ocupaciones, snapshots e historiales; no cancela ni elimina en cascada. Repetir la solicitud devuelve el recibo persistido. No implica anonimización. */
    delete: operations["deleteOwnPublication"];
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/publication": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    get?: never;
    /** @description Publica u oculta un espacio propio. Requiere rol arrendador; activar exige KYC sintético efectivo de la cuenta personal y no concede ni infiere roles. Solo cambia el estado local del espacio, sin abrirlo al catálogo general ni alterar reservas existentes. Publicar exige una zona IANA válida; si falta, responde `time_zone_required` y conserva el borrador. Un fixture de ensayo habilitado impide transiciones hasta que se deshabilite; se responde 409 con código fixture_enabled. */
    put: operations["setOwnSpacePublicationState"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/publication-content": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    get?: never;
    /** @description Modifica título, descripción, capacidad, reglas de uso, ubicación pública y/o precio base de una publicación propia activa u oculta. Requiere titularidad y rol arrendador. Un cambio de precio agrega una nueva versión de tarifa y conserva cotizaciones/reservas anteriores. La ubicación usa el catálogo SUBDERE_DPA_2018 y no se deriva de dirección o coordenadas. No cambia el estado ni la visibilidad del espacio. */
    put: operations["updateOwnPublishedSpaceContent"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/gallery": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Lista la galería propia compuesta en un orden común; fotos reales permanecen privadas hasta la declaración explícita. */
    get: operations["listOwnSyntheticSpaceGallery"];
    put?: never;
    /** @description Sin cuerpo conserva la generación del PNG sintético. Con multipart/form-data y campo image admite JPEG, PNG y WebP estático, normaliza derivados WebP y los guarda en el namespace privado media/v1. Requiere rol arrendador, propiedad del espacio, bearer y clave de idempotencia; máximo común de 10 fotos. */
    post: operations["createSyntheticOwnSpaceGalleryPhoto"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/cover": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    get?: never;
    /** @description Selecciona como portada una imagen PNG sintética activa de la galería propia. La selección queda ligada al mismo espacio y titular; retirar la imagen elimina la selección en la misma transacción. El archivo continúa privado y no se expone automáticamente al catálogo. */
    put: operations["selectOwnSyntheticSpaceCover"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/gallery/{photoId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
      };
      cookie?: never;
    };
    get: operations["getOwnSyntheticSpaceGalleryPhoto"];
    put: operations["updateOwnSpaceMediaPhoto"];
    post?: never;
    /** @description Retira la imagen propia y encola limpieza recuperable del archivo privado. */
    delete: operations["removeOwnSyntheticSpaceGalleryPhoto"];
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/gallery/{photoId}/publication": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
      };
      cookie?: never;
    };
    get?: never;
    /** @description Declara o retira explícitamente una foto sintética de la ficha pública. La portada solo se muestra mientras también sea pública. */
    put: operations["setOwnSyntheticSpaceGalleryPhotoPublication"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/gallery/{photoId}/content": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
      };
      cookie?: never;
    };
    /** @description Autoriza de nuevo al titular en cada consulta; responde PNG privado sin URL persistente. */
    get: operations["getOwnSyntheticSpaceGalleryPhotoContent"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/gallery/{photoId}/variants/{variant}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
        variant: "display" | "thumbnail";
      };
      cookie?: never;
    };
    /** @description Sirve WebP privado mediante autorización del titular en cada solicitud. Nunca expone bucket, object key ni generación. */
    get: operations["getOwnSpaceMediaVariant"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/availability": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Comprueba intervalos finitos contra ocupaciones activas. El calendario `ocupacion` sigue siendo propiedad de M06. */
    get: operations["queryOwnDraftAvailability"];
    /** @description Configura una zona IANA explícita para el borrador propio. Para un espacio activo u oculto heredado, solo repara una zona ausente y solo si no tiene reservas; nunca reemplaza una zona existente ni asigna un valor automáticamente. La autorización se limita al titular autenticado. */
    put: operations["configureOwnDraftAvailability"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/tariff": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Consulta la versión vigente de la tarifa base CLP del espacio propio en estado borrador, activo u oculto. */
    get: operations["getOwnSpaceTariff"];
    /** @description Actualiza la tarifa del espacio propio en estado borrador, activo u oculto creando una versión inmutable. Una modalidad incompatible con un horario semanal activo responde conflicto y no cambia la tarifa. Cotizaciones, reservas y simulaciones existentes conservan sus snapshots. No cambia el estado de publicación. */
    put: operations["updateOwnSpaceTariff"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/tariffs": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Lista versiones inmutables de tarifa del espacio propio en estado borrador, activo u oculto. */
    get: operations["listOwnSpaceTariffVersions"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/price-simulations": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Persiste una simulación privada del precio base si M06 confirma disponibilidad. Conserva snapshot de tarifa/versión/unidades/CLP/zona; no crea ocupación, retención, reserva, confirmación ni publicación. Horas transcurridas y fechas/meses calendario locales iniciados según aniversario local; ver decisión M05. */
    post: operations["createPrivateOwnDraftPriceSimulation"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/price-simulations/{simulationId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        simulationId: string;
      };
      cookie?: never;
    };
    /** @description Consulta el snapshot privado del titular sin recalcularlo con la tarifa actual. */
    get: operations["getOwnPrivatePriceSimulation"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/availability/blocks": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Lista los bloqueos manuales activos propios que intersectan el rango consultado. */
    get: operations["listOwnDraftManualBlocks"];
    put?: never;
    /** @description Crea un intervalo manual finito semiabierto `[inicio, término)` con motivo obligatorio. Un solape activo devuelve 409. */
    post: operations["createOwnDraftManualBlock"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/availability/timezone": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Lee la zona horaria IANA configurada para un espacio propio en estado borrador, activo u oculto. */
    get: operations["getOwnDraftTimeZone"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/spaces/{spaceId}/availability/blocks/{blockId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        blockId: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    post?: never;
    /** @description Desactiva el bloqueo manual del borrador propio; conserva el registro de ocupación. */
    delete: operations["deleteOwnDraftManualBlock"];
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/fixture": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Compatibilidad con M06-LOCAL-01: retorna el primer fixture autorizado para esa cuenta. Para seleccionar entre varios fixtures usa el catálogo y detalle; cotizar siempre requiere el space_id explícito. No es búsqueda pública ni permiso comercial. */
    get: operations["getAuthorizedLocalBookingFixture"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/catalog": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Lista fixtures de ensayo explícitamente habilitados para uno de sus dos participantes y publicaciones locales activas cuyo titular conserva elegibilidad KYC sintética efectiva. Los borradores no habilitados, espacios ocultos, ofertas sin KYC efectivo y espacios propios del actor no aparecen. No expone dirección, coordenadas geográficas ni propietario. La búsqueda textual literal e insensible a mayúsculas consulta únicamente título, descripción y categoría pública; superficie mínima, categoría, disponibilidad, atributos, precio y proximidad se combinan con AND. Los elementos con punto sintético incluyen solo una posición x/y normalizada para el mapa local; no son latitud/longitud. Cuando un intervalo filtra un espacio por hora con horario semanal activo, el intervalo debe caber por completo en un tramo y quedar libre de ocupaciones. Los filtros de atributos usan los tipos/unidades del perfil versionado indicado y solo aplican a espacios guardados con esa versión. Si se filtra por mínimo/máximo CLP inclusivo, se exige intervalo y se devuelve el total estimado de ese intervalo; la búsqueda no crea cotización ni ocupación. La proximidad requiere latitude, longitude y radius_km juntos; PostGIS aplica radio inclusivo y reporta distancia directa aproximada, no ruta. Con proximidad el orden es distancia sin redondear, total estimado cuando existe, luego UUID; con intervalo sin proximidad ordena total estimado y UUID; sin intervalo ni proximidad ordena categoría, título y UUID. Se filtran y ordenan todos los resultados antes de seleccionar la página. page_size predeterminado 5, rango 1–25. next_cursor es opaco, versionado y vinculado a cuenta, filtros y tamaño; enviarlo solo continúa la misma búsqueda. Sin next_cursor termina. Un cursor malformado, desconocido o reutilizado con otra cuenta/filtros/tamaño responde 422. La elegibilidad del titular y la autorización de fixtures se comprueban en cada solicitud. Los cursores del prototipo dependen de la instancia del Backend y pueden invalidarse al reiniciarla. Cada página consulta estado vigente; cambios de tarifa, disponibilidad o catálogo pueden cambiar la continuación. Reinicia la búsqueda para comenzar de nuevo. El paginado limita la respuesta, pero hoy se calcula y ordena el catálogo filtrado completo en el servicio, por lo que no optimiza toda la consulta. */
    get: operations["searchAuthorizedLocalSyntheticCatalog"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/catalog/{spaceId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Devuelve el detalle seguro de un fixture habilitado para uno de sus participantes o de una publicación activa con KYC sintético efectivo del titular, incluyendo categoría, perfil/atributos, tarifa y zona del mismo espacio. Borradores no habilitados, ocultos, espacios propios y ofertas sin elegibilidad responden 404. Nunca incluye dirección, coordenadas, propietario ni galería privada. */
    get: operations["getAuthorizedLocalSyntheticCatalogDetail"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/catalog/{spaceId}/availability-options": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Consulta candidatos para una fecha local de un fixture autorizado o publicación activa visible; requiere una sesión válida y la autorización correspondiente al espacio. La modalidad siempre procede de su tarifa vigente. El horizonte de inicio es hoy–hoy+90 días en la zona del espacio. Hora: duración transcurrida 1/2/4 h, inicios cada 30 min; día: 1/2/3 fechas calendario completas; mes: aniversario de un mes calendario con ajuste al último día. Excluye inicios pasados/iguales al reloj Backend, horas locales DST inexistentes, candidatos que no quepan por completo en un tramo semanal activo cuando aplica, y candidatos que solapen una ocupación activa; ambas instancias de una hora repetida se distinguen por sus intervalos UTC. Procesa vencimientos usando el mecanismo de M06. No persiste cotizaciones ni ocupaciones y no revela datos de otras reservas. La disponibilidad se revalida al cotizar y reservar. */
    get: operations["getAuthorizedLocalFixtureAvailabilityOptions"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/business-clock": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Devuelve el instante de negocio actual de Go para vistas autenticadas que calculan fechas locales. Usa el reloj real normalmente y el reloj manual solo en el stack de demo FE-14 aislado. */
    get: operations["getLocalBusinessClock"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/catalog/{spaceId}/weekly-hours": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    /** @description Solo el anfitrión del fixture sintético por hora puede consultar la regla. Sin fila, activo es false y se conserva la disponibilidad previa. Con activo true, días sin tramos están cerrados. La respuesta incluye la zona IANA vigente. */
    get: operations["getOwnSyntheticHourlyWeeklyHours"];
    /** @description Solo el anfitrión. Hasta dos tramos por día, apertura < cierre, sin solapes ni cruce de medianoche. Cierre 24:00 es límite exclusivo final del día; apertura 00:00 es válida. Horas inexistentes no producen intervalos; durante horas repetidas solo se ofrece un intervalo si todas sus porciones reales permanecen dentro de un tramo. La escritura se serializa con cotizaciones y solicitudes. No altera ocupaciones, reservas ni historial. Día/mes no son elegibles. */
    put: operations["replaceOwnSyntheticHourlyWeeklyHours"];
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/quotes": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Cotiza sin ocupar un fixture habilitado o una publicación activa visible; conserva snapshot de tarifa, zona, condiciones, categoría y perfil. Al solicitar una reserva, el Backend exige elegibilidad KYC local vigente para anfitrión y arrendatario. Para espacios por hora con horario semanal activo, el intervalo completo debe caber en un tramo y estar libre de ocupaciones. La validación se serializa con cambios de horario. Caducidad provisional configurable de 15 minutos; reservar vuelve a validar publicación, tarifa, elegibilidad, horario y disponibilidad dentro de la transacción. */
    post: operations["quoteAuthorizedLocalBooking"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Historial de reservas del participante autenticado en el fixture privado. */
    get: operations["listOwnLocalTrialReservations"];
    put?: never;
    /** @description Revalida horario, ocupación y elegibilidad KYC sintética de anfitrión y arrendatario dentro de la transacción. La falta de KYC, una cotización que ya no cabe u ocupación incompatible producen conflicto sin crear reserva ni retención. El cambio de elegibilidad no modifica reservas ni historiales previos. */
    post: operations["requestLocalTrialReservation"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    get: operations["getOwnLocalTrialReservation"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/payment": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Adaptador fake solo con LOCAL_BOOKING_TRIAL=1; admite exito, rechazo o sin_respuesta. Nunca se conecta a una pasarela. */
    post: operations["simulateLocalTrialPayment"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/settlement": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    /** @description Consulta de solo lectura para participantes; un administrador activo también puede consultar desde su bandeja administrativa. No inicia la transferencia. */
    get: operations["getOwnLocalSettlement"];
    put?: never;
    /** @description Solo el anfitrión. Persiste una única intención después del check-out + 24 horas y de verificar obligaciones; reintentos reutilizan la misma clave. Timeout queda pendiente y no se presenta como pagado. No incluye garantía ni captura por daños. */
    post: operations["requestLocalFakeSettlement"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/settlement-reconcile": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo administrador; resuelve una operación incierta persistida sin iniciar una segunda transferencia. */
    post: operations["reconcileLocalFakeSettlement"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/receipt": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    /** @description Solo participantes; comprobante JSON de ensayo generado únicamente para una liquidación confirmada. Sin validez fiscal ni movimientos reales. */
    get: operations["getOwnLocalSettlementReceipt"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/guarantee": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    /** @description Devuelve snapshot fijo de CLP 50.000, autorización/captura/liberación separadas y operaciones fake. Solo participantes de la reserva o administrador autorizado; la respuesta nunca incluye datos privados de las partes. */
    get: operations["getOwnSyntheticReservationGuarantee"];
    put?: never;
    /** @description Arrendatario ejecuta autorización fake luego del pago confirmado. Administrador registra captura o liberación. La misma operación/tipo se reutiliza idempotentemente; timeout queda pendiente para conciliación. Rechazo confirmado solo aplica a autorización; captura/liberación admiten éxito o timeout. La liberación automática de una reserva sin reclamo usa el check-out persistido + 24 horas. Nunca representa movimiento de dinero real. */
    post: operations["executeLocalSyntheticGuaranteeOperation"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/guarantee-reconcile": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo administrador; persiste/deduplica un resultado fake tardío de autorización, captura o liberación. Rechazo solo es válido para autorización. Una autorización tardía no reactiva la reserva y crea su liberación compensatoria pendiente. */
    post: operations["reconcileLocalGuaranteeFakeResult"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/financial-decision": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo administrador; decisión inmutable separada del resultado histórico M10. Deducción 0..importe autorizado; reclamo rechazado exige cero. Una deducción positiva exige motivo estructurado y evidencia sintética de esta reserva. La captura debe confirmarse antes de marcar aplicada. */
    post: operations["recordAdminLocalFinancialDecision"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/payment-events": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Callback exclusivo del fake local. La firma HMAC en X-Local-Payment-Signature se valida antes de persistir. El inbox inmutable deduplica provider_event_id; la reconciliación aplica el evento después y puede recuperarlo tras reiniciar Backend. No corresponde a una firma ni contrato de proveedor real. */
    post: operations["receiveAuthenticatedLocalPaymentEvent"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/decision": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    post: operations["decideLocalTrialReservation"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/cancellation-preview": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Solo el arrendatario titular. Muestra snapshot de política, plazo, subtotal reembolsable y etiqueta antes de confirmar. La elegibilidad es informativa; POST vuelve a validar bajo bloqueo. */
    get: operations["previewLocalTrialReservationCancellation"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/cancel": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Conserva cancelación pre-pago sin devolución. Para pagada, aprobada_host, firma_parcial o lista_para_checkin, solo el arrendatario titular puede cancelar estrictamente antes del inicio mediante local_flexible_v1. La cancelación y las firmas comparten el bloqueo de reserva: una versión parcialmente firmada se anula; una versión ya firmada queda como hecho histórico y la reserva puede cancelarse con devolución fake. La transacción cancela, libera ocupacion, agrega historial y una obligación de devolución fake por el subtotal pagado. La respuesta no confirma que la devolución haya sido procesada. Política local_flexible_v1 solo para fixtures. */
    post: operations["cancelLocalTrialReservation"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/refund": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Fake exclusivo local. Usa el operation_id estable devuelto por detalle/cancelación como Idempotency-Key en todos los intentos. Fallo y timeout conservan la obligación pendiente; éxito marca una única devolución completada. Sin movimiento de dinero. */
    post: operations["simulateLocalTrialRefund"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/messages": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    /** @description Solo participantes leen. Devuelve los mensajes más recientes ordenados por secuencia ascendente; before es la secuencia exclusiva del cursor y older_cursor permite cargar la página previa. Conserva lectura en estados terminales. */
    get: operations["listOwnLocalReservationMessages"];
    put?: never;
    /** @description Envío idempotente solo para los anfitriones y arrendatarios de la reserva en pendiente_de_pago, pagada, aprobada_host, firma_parcial y lista_para_checkin. Los estados cancelados, rechazados o vencidos conservan solo lectura. Para firma_parcial, un envío nuevo consulta el reloj tras adquirir los bloqueos comunes de participantes/reserva; si start_at ya llegó, materializa en la misma transacción el vencimiento existente de contrato, libera la ocupación y registra la devolución fake, luego responde conflicto sin insertar el mensaje. Los reintentos idempotentes de mensajes ya persistidos devuelven su resultado original. No agrega notificaciones ni tiempo real. */
    post: operations["sendOwnLocalReservationMessage"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/messages/read": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo el anfitrión o arrendatario de la reserva. Avanza de forma monotónica el cursor propio hasta una secuencia de mensaje del mismo hilo. El mock lo solicita únicamente después de mostrar correctamente la página reciente; no confirma lectura a la contraparte. */
    post: operations["advanceOwnLocalReservationMessageReadCursor"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservation_id}/disputes": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservation_id: string;
      };
      cookie?: never;
    };
    /** @description Solo el anfitrión y arrendatario de la reserva; terceros reciben 404. */
    get: operations["listOwnReservationTrialDisputes"];
    put?: never;
    /** @description Excepción sintética de privacidad: solo su anfitrión abre, incluso en una reserva terminal. No implementa CU-39, ventana de 24 horas, evidencia, operación, cambios de reserva ni fondos. */
    post: operations["openOwnHostedTrialDispute"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/disputes/{dispute_id}/history": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get: operations["getOwnTrialDisputeHistory"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/disputes": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Solo administrador; consulta local sin resolución económica. */
    get: operations["listOpenLocalTrialDisputes"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/disputes/{dispute_id}/close": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Cierre administrativo del ensayo; no es resolución, garantía ni liquidación. */
    post: operations["closeLocalTrialDispute"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/auth/logout": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description CU-06/RQF-023: requiere sesión válida; revoca credencial. Replay/expirada devuelve 401. */
    post: operations["logout"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationId}/contract": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Crea idempotentemente una versión snapshot privada tras aprobación del anfitrión. Documento de ensayo sintético sin validez jurídica. */
    post: operations["createOrGetSyntheticReservationContract"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/contracts/{contractId}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        contractId: string;
      };
      cookie?: never;
    };
    get: operations["getOwnSyntheticContract"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/contracts/{contractId}/sign": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        contractId: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Revalida titularidad y start_at bajo bloqueo de reserva; es una firma simulada, no tiene validez jurídica. */
    post: operations["signSyntheticContractAsParticipant"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/contracts/{contractId}/reject": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        contractId: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description El rechazo es definitivo para el firmante y versión; no cancela la reserva inmediatamente. */
    post: operations["rejectSyntheticContractVersion"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/contracts/{contractId}/document": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        contractId: string;
      };
      cookie?: never;
    };
    /** @description Devuelve PDF privado solo a anfitrión/arrendatario cuando ambas firmas están completas. */
    get: operations["downloadSignedSyntheticContract"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/check-in": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo el arrendatario, con ambas firmas completas y en la fecha local de inicio del espacio. El servidor crea una evidencia PNG sintética privada y registra hora/ubicación de ensayo. */
    post: operations["recordSyntheticCheckIn"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/check-out": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo el arrendatario sobre una reserva en curso. El servidor genera evidencia privada y persiste el instante que inicia las 24 horas locales para reclamos. */
    post: operations["recordSyntheticCheckOut"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/reception": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description El anfitrión registra recepción y, opcionalmente, observaciones sobre una reserva finalizada. No crea por sí misma un reclamo formal ni liquida fondos ni modifica el plazo que parte del check-out. */
    post: operations["recordSyntheticHostReception"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/operations": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    /** @description Historial secuencial de check-in, check-out y recepción, solo para participantes. */
    get: operations["listSyntheticRentalOperations"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/evidence/{evidenceID}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
        evidenceID: string;
      };
      cookie?: never;
    };
    /** @description Descarga de imagen PNG sintética privada; solo anfitrión o arrendatario de la reserva. */
    get: operations["downloadSyntheticRentalOperationEvidence"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/damage-claim/defense/evidence": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo el arrendatario titular de un descargo existente en reclamo abierto. Genera en Backend una imagen PNG sintética privada; no acepta archivos del cliente. Idempotente por reserva y clave. Su archivo sigue la retención de vínculos de reserva ratificada de 24 meses y la limpieza por baja. */
    post: operations["createSyntheticDamageClaimDefenseEvidence"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/damage-claim": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    /** @description Solo los participantes; devuelve el reclamo formal local separado de la observación de recepción y de la incidencia de privacidad. */
    get: operations["getOwnSyntheticDamageClaim"];
    put?: never;
    /** @description Solo el anfitrión, antes de 24 horas desde el check-out persistido. Pasa la reserva a en_disputa; no adjudica responsabilidad ni mueve fondos. Reutiliza la evidencia sintética privada del check-out. */
    post: operations["openSyntheticHostDamageClaim"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/local/booking-trial/reservations/{reservationID}/damage-claim/defense": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo el arrendatario, mientras exista reclamo abierto. Registra texto sintético; la evidencia privada se genera mediante createSyntheticDamageClaimDefenseEvidence y no acepta archivos del cliente. */
    post: operations["submitSyntheticDamageClaimDefense"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/audit-events": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Consultar eventos minimizados de auditoría
     * @description Solo administrador activo. Periodo requerido UTC desde inclusivo/hasta exclusivo, máximo 31 días. Filtros exactos sin texto libre. Orden occurred_at DESC e ID DESC. Cursor URL-safe ligado a cuenta, filtros normalizados, tamaño y corte estable; las peticiones administrativas autenticadas de lectura no aparecen en su propio recorrido. Registra un evento mínimo por petición válida o rechazada; no persiste respuestas.
     */
    get: operations["listAdminLocalAuditEvents"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/audit-events/export": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Descargar exportación versionada de auditoría local
     * @description Mismos filtros obligatorios que el listado, máximo 31 días y 10.000 eventos. Si se supera el máximo devuelve 422 y no entrega un archivo truncado. El corte se fija antes de registrar la petición de exportación; el propio acceso queda fuera del archivo.
     */
    get: operations["exportAdminLocalAuditEvents"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/accounts/{accountID}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Buscar una cuenta sintética por UUID técnico exacto
     * @description Solo administrador activo. La respuesta excluye correo, contacto, credenciales y texto libre. Cada petición queda auditada, incluida la búsqueda sin resultado.
     */
    get: operations["getAdminLocalAccountByID"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/published-spaces": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Consultar publicaciones locales activas
     * @description Solo administrador activo. Devuelve una proyección mínima sin dirección, descripción, atributos privados ni contactos. Solo lectura y una auditoría por petición.
     */
    get: operations["listAdminLocalPublishedSpaces"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/published-spaces/{spaceID}": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Consultar una publicación activa local
     * @description Solo administrador activo. Los borradores y publicaciones ocultas no son visibles en este recurso. No incluye dirección ni contenido privado.
     */
    get: operations["getAdminLocalPublishedSpace"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/accounts/{accountID}/block": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /**
     * Bloquear cuenta para el prototipo local
     * @description Solo administrador activo. Registra motivo y auditoría, invalida sesiones/tokens normales y conserva reservas/obligaciones. El usuario podrá autenticarse luego en modo restringido.
     */
    post: operations["blockLocalAccount"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/accounts/{accountID}/unblock": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    get?: never;
    put?: never;
    /**
     * Desbloquear cuenta local
     * @description Elimina solo el estado actual de bloqueo y conserva historial/auditoría. Revoca también la sesión restringida para exigir un nuevo inicio de sesión.
     */
    post: operations["unblockLocalAccount"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/reports/reservations": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Contar reservas creadas en periodo por su estado vigente
     * @description Administrador activo; lectura y auditoría por petición. No representa estados históricos ni muta reservas.
     */
    get: operations["getAdminLocalReservationsReport"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/reports/finance": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Consultar hechos financieros fake y obligaciones pendientes
     * @description Categorías separadas, sin garantía como ingreso, sin duplicar reintentos y sin conciliar; periodo obligatorio de hasta 31 días calendario.
     */
    get: operations["getAdminLocalFinanceReport"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/reports/disputes": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Contar reclamos sintéticos abiertos en el período por su estado vigente
     * @description Administrador activo; lectura y una auditoría por petición. El período se aplica a la apertura y los grupos reflejan el estado vigente al generar el reporte; no reconstruye estados históricos ni incluye texto, evidencias o datos personales.
     */
    get: operations["getAdminLocalDisputesReport"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/reservations": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /**
     * Consultar reservas sintéticas para administración
     * @description Solo administrador. Filtros por ID, estado y fecha de creación (RFC3339; desde inclusivo/hasta exclusivo). Orden creada_en DESC e ID DESC. Cursor opaco vinculado a cuenta/filtros. No concilia ni modifica datos de negocio; audita una consulta por petición.
     */
    get: operations["listAdminLocalReservations"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/reservations/{reservationID}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    /**
     * Consultar historial y finanzas de una reserva sintética
     * @description Solo administrador. Respuesta mínima con identificadores de participantes solo mientras sigan vinculados; no expone contacto, credenciales, documentos, mensajes ni payloads privados. Una lectura no inicia operaciones ni procesa vencimientos.
     */
    get: operations["getAdminLocalReservation"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/damage-claims": {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    /** @description Solo administrador. Lista reclamos sintéticos abiertos y su evidencia privada para revisión local; nunca devuelve montos ni datos de pagos. */
    get: operations["listOpenLocalSyntheticDamageClaims"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/damage-claims/{claimID}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        claimID: string;
      };
      cookie?: never;
    };
    /** @description Solo administrador; incluye estado, historial y evidencia privada ligada al reclamo. */
    get: operations["getAdminLocalSyntheticDamageClaim"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/damage-claims/{claimID}/evidence/{evidenceID}": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        claimID: string;
        evidenceID: string;
      };
      cookie?: never;
    };
    /** @description Descarga inline solo de PNG sintéticos asociados al reclamo, con rol administrador. */
    get: operations["getAdminLocalDamageClaimEvidence"];
    put?: never;
    post?: never;
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
  "/admin/local/damage-claims/{claimID}/resolution": {
    parameters: {
      query?: never;
      header?: never;
      path: {
        claimID: string;
      };
      cookie?: never;
    };
    get?: never;
    put?: never;
    /** @description Solo administrador. Cierra el reclamo de forma transaccional e idempotente con resultado y motivo estructurado. No asigna montos ni mueve fondos; garantía, deducciones, captura, liberación y devoluciones quedan para LOCAL-FIN-01. */
    post: operations["resolveLocalSyntheticDamageClaim"];
    delete?: never;
    options?: never;
    head?: never;
    patch?: never;
    trace?: never;
  };
}
export type webhooks = Record<string, never>;
export interface components {
  schemas: {
    /** @enum {string} */
    AdminCapability:
      | "legacy.administration"
      | "accounts.list"
      | "accounts.read"
      | "accounts.update"
      | "accounts.block"
      | "accounts.roles"
      | "publications.list"
      | "publications.read"
      | "publications.update"
      | "publications.moderate"
      | "publications.retire"
      | "admin_access.manage"
      | "kpi.read"
      | "audit.read"
      | "audit.export";
    GlobalAdminMeEnvelope: {
      data: {
        /** Format: uuid */
        account_id: string;
        roles: string[];
        capabilities: components["schemas"]["AdminCapability"][];
        /** Format: date-time */
        generated_at: string;
      };
    };
    GlobalAdminOverviewEnvelope: {
      data: {
        /** Format: int64 */
        total_accounts: number | string;
        /** Format: int64 */
        administrative_blocked: number | string;
        by_state: {
          [key: string]: number | string;
        };
        by_role: {
          [key: string]: number | string;
        };
        /** Format: date-time */
        generated_at: string;
        definition: string;
      };
    };
    GlobalAdminAccount: {
      /** Format: uuid */
      account_id: string;
      email: string;
      display_name?: string | null;
      phone?: string | null;
      /** Format: date-time */
      profile_updated_at?: string | null;
      /** @enum {string} */
      state:
        | "correo_pendiente"
        | "activo"
        | "bloqueado"
        | "baja_solicitada"
        | "desidentificado";
      /** Format: date-time */
      created_at: string;
      administrative_blocked: boolean;
      roles: (
        "arrendatario" | "arrendador" | "administrador" | "administrador_global"
      )[];
      /** Format: int64 */
      publication_count: number | string;
      /** Format: int64 */
      reservation_count: number | string;
      /** Format: int64 */
      incident_count: number | string;
    };
    GlobalAdminAccountEnvelope: {
      data: components["schemas"]["GlobalAdminAccount"];
    };
    GlobalAdminAccountPageEnvelope: {
      data: {
        items: components["schemas"]["GlobalAdminAccount"][];
        next_cursor: string;
        /** Format: int64 */
        matching_total: number | string;
        /** Format: date-time */
        generated_at: string;
      };
    };
    GlobalAdminPublication: {
      /** Format: uuid */
      publication_id: string;
      /** Format: uuid */
      owner_id: string;
      owner_name?: string | null;
      owner_email: string;
      category_code: string;
      category_name: string;
      title: string;
      description: string;
      area_m2: string;
      capacity: number;
      usage_rules: string;
      address: string;
      /** @enum {string} */
      state: "borrador" | "activa" | "oculta" | "eliminada";
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      /** @description CLP decimal string; preserves all PostgreSQL bigint digits */
      base_price_clp: string;
      /** Format: date-time */
      updated_at: string;
      publicly_visible: boolean;
      public_cover_available: boolean;
      attribute_schema_version: number;
      attributes: {
        [key: string]: unknown;
      };
      attribute_schema: {
        [key: string]: unknown;
      };
    };
    GlobalAdminPublicationPageEnvelope: {
      data: {
        items: components["schemas"]["GlobalAdminPublication"][];
        next_cursor: string;
        /** Format: int64 */
        matching_total: number | string;
        /** Format: date-time */
        generated_at: string;
      };
    };
    PublicSpace: {
      /** Format: uuid */
      space_id: string;
      category_code: string;
      category_name: string;
      title: string;
      description: string;
      area_m2: number;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      /** Format: int64 */
      base_price_clp: number | string;
      /** @constant */
      currency: "CLP";
      region_code?: string | null;
      region_name?: string | null;
      comuna_code?: string | null;
      comuna_name?: string | null;
      /**
       * Format: int64
       * @description Estimación calculada con PriceUnits
       */
      estimated_total_clp: number | string | null;
      /**
       * Format: uuid
       * @description Solo se incluye cuando la publicación tiene una portada PNG activa y declarada pública. El endpoint público vuelve a validar la visibilidad al servirla.
       */
      cover_image_id?: string;
      features?: components["schemas"]["PublicSpaceFeature"][];
      images?: components["schemas"]["PublicSpaceImageRef"][];
    };
    PublicSpaceEnvelope: {
      data: components["schemas"]["PublicSpace"];
    };
    PublicSpaceDetail: {
      /** Format: uuid */
      space_id: string;
      category_code: string;
      category_name: string;
      title: string;
      description: string;
      area_m2: number;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      /** Format: int64 */
      base_price_clp: number | string;
      /** @constant */
      currency: "CLP";
      region_code?: string | null;
      region_name?: string | null;
      comuna_code?: string | null;
      comuna_name?: string | null;
      /** Format: int64 */
      estimated_total_clp: number | string | null;
      usage_rules: string;
      features: components["schemas"]["PublicSpaceFeature"][];
      images: components["schemas"]["PublicSpaceImageRef"][];
    };
    PublicSpaceDetailEnvelope: {
      data: components["schemas"]["PublicSpaceDetail"];
    };
    PublicSpaceFeature: {
      code: string;
      label: string;
      value: boolean;
    };
    PublicSpaceImageRef: {
      /** Format: uuid */
      image_id: string;
      is_cover: boolean;
      /** @enum {string} */
      source: "synthetic" | "media";
    };
    PublicSpacePage: {
      items: components["schemas"]["PublicSpace"][];
      next_cursor?: string;
    };
    PublicSpaceCategory: {
      code: string;
      name: string;
      order: number;
    };
    PublicSpaceCategories: {
      items: components["schemas"]["PublicSpaceCategory"][];
    };
    PublicCategoryFilter: {
      code: string;
      label: string;
      /** @enum {string} */
      type: "boolean" | "integer" | "number" | "enum" | "enum_list";
      order: number;
      unit?: string;
      options?: string[];
      minimum?: number;
      maximum?: number;
      step?: number;
      profile_version: number;
    };
    PublicCategoryFilters: {
      category_code: string;
      items: components["schemas"]["PublicCategoryFilter"][];
    };
    PublicLocation: {
      region_code: string;
      region_name: string;
      comuna_code: string;
      comuna_name: string;
    };
    PublicLocations: {
      items: components["schemas"]["PublicLocation"][];
    };
    AdminLocalBlockInput: {
      /** @enum {string} */
      reason_code:
        "riesgo_seguridad" | "uso_indebido" | "revision_administrativa";
    };
    AdminLocalUnblockInput: {
      /** @enum {string} */
      reason_code: "revision_concluida" | "error_bloqueo" | "riesgo_resuelto";
    };
    AdminLocalBlock: {
      /** Format: uuid */
      account_id: string;
      blocked: boolean;
      /** Format: date-time */
      changed_at: string;
    };
    AdminLocalBlockEnvelope: {
      data: components["schemas"]["AdminLocalBlock"];
      /** @constant */
      notice: "ENSAYO LOCAL — SIN MOVIMIENTOS REALES";
    };
    AdminLocalAccount: {
      /** Format: uuid */
      account_id: string;
      /** @enum {string} */
      state:
        | "correo_pendiente"
        | "activo"
        | "bloqueado"
        | "baja_solicitada"
        | "desidentificado";
      /** Format: date-time */
      created_at: string;
      blocked: boolean;
      roles: (
        "arrendatario" | "arrendador" | "administrador" | "administrador_global"
      )[];
    };
    AdminLocalAccountEnvelope: {
      data: components["schemas"]["AdminLocalAccount"];
    };
    AdminLocalPublishedSpace: {
      /** Format: uuid */
      space_id: string;
      /** Format: uuid */
      owner_id: string | null;
      title: string;
      category_code: string;
      category_name: string;
      /** @constant */
      state: "activa";
      /** Format: date-time */
      updated_at: string;
    };
    AdminLocalPublishedSpaceEnvelope: {
      data: components["schemas"]["AdminLocalPublishedSpace"];
    };
    AdminLocalPublishedSpacesEnvelope: {
      data: {
        items: components["schemas"]["AdminLocalPublishedSpace"][];
      };
    };
    AdminLocalReportPeriod: {
      /**
       * Format: date-time
       * @description Inclusivo y normalizado UTC.
       */
      from: string;
      /**
       * Format: date-time
       * @description Exclusivo y normalizado UTC.
       */
      until: string;
      /** @description Zona IANA usada para validar el límite de 31 días calendario. */
      timezone: string;
    };
    AdminLocalReservationsReport: {
      /** Format: date-time */
      generated_at: string;
      period: components["schemas"]["AdminLocalReportPeriod"];
      /** @constant */
      currency: "CLP";
      /** @constant */
      notice: "ENSAYO LOCAL — SIN MOVIMIENTOS REALES";
      items: {
        state: string;
        count: number;
      }[];
    };
    AdminLocalFinanceLine: {
      /** @enum {string} */
      kind:
        | "cobro_arriendo"
        | "devolucion"
        | "garantia_autorizada"
        | "garantia_capturada"
        | "garantia_liberada";
      state: string;
      count: number;
      amount_clp: number;
    };
    AdminLocalFinanceReport: {
      /** Format: date-time */
      generated_at: string;
      period: components["schemas"]["AdminLocalReportPeriod"];
      /** @constant */
      currency: "CLP";
      /** @constant */
      notice: "ENSAYO LOCAL — SIN MOVIMIENTOS REALES";
      confirmed_movements: components["schemas"]["AdminLocalFinanceLine"][];
      pending_obligations: components["schemas"]["AdminLocalFinanceLine"][];
    };
    AdminLocalReservationsReportEnvelope: {
      data: components["schemas"]["AdminLocalReservationsReport"];
      /** @constant */
      notice: "ENSAYO LOCAL — SIN MOVIMIENTOS REALES";
    };
    AdminLocalFinanceReportEnvelope: {
      data: components["schemas"]["AdminLocalFinanceReport"];
      /** @constant */
      notice: "ENSAYO LOCAL — SIN MOVIMIENTOS REALES";
    };
    AdminLocalDisputesReport: {
      /** Format: date-time */
      generated_at: string;
      period: components["schemas"]["AdminLocalReportPeriod"];
      /** @constant */
      notice: "ENSAYO LOCAL — SIN MOVIMIENTOS REALES";
      items: {
        /** @enum {string} */
        state: "abierto" | "resuelta";
        count: number;
      }[];
    };
    AdminLocalDisputesReportEnvelope: {
      data: components["schemas"]["AdminLocalDisputesReport"];
      /** @constant */
      notice: "ENSAYO LOCAL — SIN MOVIMIENTOS REALES";
    };
    AdminLocalAuditFilter: {
      /**
       * Format: date-time
       * @description Inclusivo y normalizado a UTC.
       */
      from: string;
      /**
       * Format: date-time
       * @description Exclusivo y normalizado a UTC.
       */
      until: string;
      /** Format: uuid */
      actor_id?: string;
      resource_type?: string;
      /** Format: uuid */
      resource_id?: string;
      action?: string;
      /** @enum {string} */
      result?: "exito" | "rechazo";
    };
    AdminLocalAuditEvent: {
      /** Format: uuid */
      id: string;
      /** Format: date-time */
      occurred_at: string;
      /** Format: uuid */
      actor_id: string | null;
      resource_type: string;
      /** Format: uuid */
      resource_id: string;
      action: string;
      /** @enum {string} */
      result: "exito" | "rechazo";
      reason_code: string;
      correlation_id: string;
    };
    AdminLocalAuditPageEnvelope: {
      data: {
        items: components["schemas"]["AdminLocalAuditEvent"][];
        next_cursor?: string;
      };
      safety_notice: string;
    };
    AdminLocalAuditExport: {
      /** @enum {integer} */
      version: 1;
      /** Format: date-time */
      generated_at: string;
      filters: components["schemas"]["AdminLocalAuditFilter"];
      events: components["schemas"]["AdminLocalAuditEvent"][];
    };
    AdminLocalReservationSummary: {
      /** Format: uuid */
      id: string;
      /**
       * Format: uuid
       * @description null tras retirar vínculo por retención
       */
      host_id: string | null;
      /**
       * Format: uuid
       * @description null tras retirar vínculo por retención
       */
      renter_id: string | null;
      state: string;
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
      /** Format: int64 */
      subtotal_clp: number | string;
      /** @enum {string} */
      currency: "CLP";
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      updated_at: string;
    };
    AdminLocalReservationPageEnvelope: {
      data: {
        items: components["schemas"]["AdminLocalReservationSummary"][];
        next_cursor?: string;
      };
      safety_notice: string;
    };
    AdminLocalReservationDetailEnvelope: {
      data: components["schemas"]["AdminLocalReservationSummary"] & {
        /** Format: uuid */
        space_id: string;
        history: {
          /** Format: int64 */
          sequence: number | string;
          from?: string | null;
          to: string;
          /** Format: uuid */
          actor_id?: string | null;
          /** Format: date-time */
          at: string;
        }[];
        rental_payments: {
          result: string;
          amount_clp: number;
          /** Format: date-time */
          at: string;
        }[];
        rental_payment_operations: {
          /** Format: uuid */
          id: string;
          state: string;
          requested_result: string;
          /** Format: date-time */
          created_at: string;
          /** Format: date-time */
          updated_at: string;
        }[];
        refund: {
          [key: string]: unknown;
        } | null;
        guarantee: {
          [key: string]: unknown;
        } | null;
        claim: {
          [key: string]: unknown;
        } | null;
      };
      safety_notice: string;
    };
    LocalReview: {
      /** Format: uuid */
      id: string;
      /** @enum {string} */
      target_type?: "espacio" | "arrendatario";
      rating: number;
      /** @enum {string} */
      state?: "publicada" | "reportada";
      comment?: string;
      /** Format: date-time */
      created_at: string;
      reused?: boolean;
    };
    LocalReviewEnvelope: {
      data: components["schemas"]["LocalReview"];
      notice: string;
    };
    LocalReviewListEnvelope: {
      data: {
        items: components["schemas"]["LocalReview"][];
      };
      notice: string;
    };
    LocalSpaceReviewsEnvelope: {
      data: {
        items: components["schemas"]["LocalReview"][];
        average: number;
        count: number;
      };
      notice: string;
    };
    LocalReviewReport: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      review_id: string;
      /** Format: uuid */
      space_id: string;
      review_comment?: string;
      /** @enum {string} */
      reason_code:
        "insultos_acoso" | "datos_personales" | "spam" | "ajeno_experiencia";
      /** @enum {string} */
      state: "pendiente" | "desestimada" | "oculta";
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      resolved_at?: string | null;
      resolution_reason_code?: string;
    };
    LocalReviewReportEnvelope: {
      data: components["schemas"]["LocalReviewReport"];
      notice: string;
    };
    LocalReviewReportListEnvelope: {
      data: {
        items: components["schemas"]["LocalReviewReport"][];
      };
      notice: string;
    };
    LocalNoticeTerminal: {
      /** Format: uuid */
      event_id: string;
      /** @enum {string} */
      event_type:
        | "checkin_registrado"
        | "resena_reportada"
        | "reclamo_abierto"
        | "reclamo_resuelto"
        | "reserva_cancelada"
        | "cuenta_bloqueada_login"
        | "verificacion_aprobada"
        | "verificacion_rechazada"
        | "pago_arriendo_confirmado"
        | "pago_arriendo_rechazado"
        | "nueva_reserva_pagada"
        | "invitacion_firma"
        | "contrato_firmado";
      /** @enum {string} */
      state: "fallo_terminal";
      error_code: string;
      cycle_number: number;
      total_attempts: number;
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      terminal_at: string;
    };
    LocalNoticeListEnvelope: {
      data: {
        items: components["schemas"]["LocalNoticeTerminal"][];
      };
      notice: string;
    };
    LocalNoticeRecoveryEnvelope: {
      data: {
        /** Format: uuid */
        notice_id: string;
        cycle: number;
        reused?: boolean;
      };
      notice: string;
    };
    LocalDamageClaimInput: {
      /** @description Texto sintético del reclamo o descargo. */
      description: string;
    };
    LocalDamageClaimEnvelope: {
      data: components["schemas"]["LocalDamageClaim"];
      /** @constant */
      notice: "ENSAYO LOCAL — RECLAMO SINTÉTICO; SIN ADJUDICACIÓN NI MOVIMIENTO DE FONDOS";
    };
    LocalDamageClaimDefenseEnvelope: {
      data: components["schemas"]["LocalDamageClaimDefense"];
      /** @constant */
      notice: "ENSAYO LOCAL — RECLAMO SINTÉTICO; SIN ADJUDICACIÓN NI MOVIMIENTO DE FONDOS";
    };
    LocalDamageClaimListEnvelope: {
      data: {
        items: components["schemas"]["LocalDamageClaim"][];
      };
      /** @constant */
      notice: "ENSAYO LOCAL — RECLAMO SINTÉTICO; SIN ADJUDICACIÓN NI MOVIMIENTO DE FONDOS";
    };
    LocalDamageClaimResolutionInput: {
      /** @enum {string} */
      outcome: "acogido" | "rechazado";
      /** @enum {string} */
      reason_code:
        | "evidencia_suficiente"
        | "evidencia_insuficiente"
        | "hecho_no_acreditado"
        | "informacion_insuficiente";
    };
    /** @description Los identificadores de actores se devuelven como null cuando la política local de privacidad ya retiró el vínculo. El hecho de la resolución permanece conservado. */
    LocalDamageClaimResolution: {
      /** @enum {string} */
      outcome: "acogido" | "rechazado";
      reason_code: string;
      /** Format: uuid */
      actor_id: string | null;
      /** Format: date-time */
      resolved_at: string;
      reused?: boolean;
    };
    LocalDamageClaimEvidence: {
      /** Format: uuid */
      id: string;
      /** @enum {string} */
      operation: "checkin" | "checkout" | "recepcion" | "descargo";
      /** @constant */
      fixture_code: "synthetic-png-v1";
      /** @constant */
      mime_type: "image/png";
      sha256: string;
      size_bytes: number;
      /** Format: date-time */
      created_at: string;
      content_url: string;
    };
    /** @description Los actores desvinculados por privacidad se representan como null; la transición histórica se conserva. */
    LocalDamageClaimTransition: {
      /** Format: int64 */
      sequence: number | string;
      /** @enum {string} */
      action: "reclamo_abierto" | "descargo_registrado" | "reclamo_resuelto";
      /** Format: uuid */
      actor_id: string | null;
      /** Format: date-time */
      occurred_at: string;
    };
    /** @description Tras vencer la retención local pueden retirarse los vínculos de reserva, participantes, operación, evidencia y actores. Los identificadores requeridos se representan como null, nunca como UUID vacío. evidence y history se devuelven siempre como arreglos JSON; evidence puede ser [] tras purgar vínculos, mientras history conserva los hechos históricos disponibles. */
    LocalDamageClaim: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      reservation_id: string | null;
      /** Format: uuid */
      host_id: string | null;
      /** Format: uuid */
      renter_id: string | null;
      /** Format: uuid */
      checkout_operation_id: string | null;
      /** Format: uuid */
      checkout_evidence_id: string | null;
      description: string;
      /** @enum {string} */
      state: "abierto" | "resuelta";
      /** Format: date-time */
      opened_at: string;
      /** Format: date-time */
      claim_deadline_at: string;
      reused?: boolean;
      defense?: components["schemas"]["LocalDamageClaimDefense"];
      resolution?: components["schemas"]["LocalDamageClaimResolution"];
      evidence: components["schemas"]["LocalDamageClaimEvidence"][];
      history: components["schemas"]["LocalDamageClaimTransition"][];
    };
    /** @description actor_id es null si el vínculo personal se retiró al vencer la retención local. */
    LocalDamageClaimDefense: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      actor_id: string | null;
      description: string;
      /** Format: date-time */
      created_at: string;
      reused?: boolean;
      evidence: components["schemas"]["LocalDamageClaimEvidence"][];
    };
    LocalRentalOperationInput: {
      /** @description Comentario sintético opcional de check-in o check-out. */
      comments?: string;
      /** @description Observación opcional de recepción; no equivale a reclamo formal. */
      observations?: string;
    };
    LocalRentalOperationEnvelope: {
      data: components["schemas"]["LocalRentalOperation"];
      /** @constant */
      notice: "ENSAYO LOCAL — EVIDENCIA SINTÉTICA; SIN VALIDEZ PROBATORIA REAL";
    };
    LocalRentalOperationListEnvelope: {
      data: {
        items: components["schemas"]["LocalRentalOperation"][];
      };
      /** @constant */
      notice: "ENSAYO LOCAL — EVIDENCIA SINTÉTICA; SIN VALIDEZ PROBATORIA REAL";
    };
    LocalRentalOperation: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      reservation_id: string;
      /** @enum {string} */
      kind: "checkin" | "checkout" | "recepcion" | "descargo";
      /** Format: uuid */
      actor_id: string;
      /** Format: date-time */
      occurred_at: string;
      time_zone: string;
      synthetic_location: {
        /** @constant */
        source: "synthetic-fixture-v1";
        /** @constant */
        location_code: "santiago-demo-center-v1";
        /** @constant */
        latitude: -33.456;
        /** @constant */
        longitude: -70.6693;
      };
      comments?: string;
      observations?: string;
      /** @enum {string} */
      result:
        "registrada" | "recepcion_conforme" | "recepcion_con_observaciones";
      /**
       * Format: date-time
       * @description Check-out registrado más 24 horas; basado en instante persistido.
       */
      claim_deadline_at?: string;
      sequence: number;
      reused?: boolean;
      evidence: {
        /** Format: uuid */
        id: string;
        content_url?: string;
        /** @constant */
        fixture_code: "synthetic-png-v1";
        /** @constant */
        mime_type: "image/png";
        sha256: string;
        size_bytes: number;
        /** Format: date-time */
        created_at: string;
      }[];
    };
    LocalSyntheticContract: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      reservation_id: string;
      version: number;
      /** @enum {string} */
      state: "generado" | "firma_parcial" | "firmado" | "anulado";
      /**
       * Format: byte
       * @description Snapshot JSON serializado; solo hechos de la reserva y aviso sintético.
       */
      snapshot: string;
      sha256: string;
      signatures: {
        /** Format: uuid */
        signer_id: string;
        /** @enum {string} */
        role: "anfitrion" | "arrendatario";
        /** @enum {string} */
        state: "pendiente" | "firmada" | "rechazada";
        reason?: string;
        /** Format: date-time */
        updated_at: string;
      }[];
      history: {
        sequence: number;
        /** Format: uuid */
        actor_id?: string;
        action: string;
        reason?: string;
        /** Format: date-time */
        at: string;
      }[];
    };
    LocalBookingFixtureEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: {
        /** Format: uuid */
        space_id: string;
        title: string;
        category: string;
        /** Format: uuid */
        host_id: string;
        /** Format: uuid */
        renter_id: string;
        /** @enum {string} */
        rate_unit: "hora" | "dia" | "mes";
        base_price_clp: number;
        /** @constant */
        currency: "CLP";
        time_zone: string;
      };
    };
    LocalCatalogEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: {
        items: components["schemas"]["LocalCatalogItem"][];
        /** @description Cursor opaco presente únicamente cuando existe otra página. */
        next_cursor?: string;
      };
    };
    LocalCatalogDetailEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalCatalogItem"];
    };
    LocalAvailabilityOptionsEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalAvailabilityOptions"];
    };
    LocalBusinessClockEnvelope: {
      data: {
        /** Format: date-time */
        now: string;
      };
    };
    LocalAvailabilityOptions: {
      /** Format: uuid */
      space_id: string;
      time_zone: string;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      items: components["schemas"]["LocalAvailableInterval"][];
    };
    LocalWeeklyHoursEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalWeeklyHours"];
    };
    LocalWeeklyHours: {
      /** Format: uuid */
      space_id: string;
      enabled: boolean;
      time_zone: string;
      days: components["schemas"]["LocalWeeklyDay"][];
    };
    LocalWeeklyDay: {
      /** @description Día ISO de la semana; lunes=1, domingo=7. */
      weekday: number;
      periods: components["schemas"]["LocalWeeklyPeriod"][];
    };
    LocalWeeklyPeriod: {
      open: string;
      /** @description 24:00 representa el límite final exclusivo del día y solo puede usarse al cerrar un tramo. */
      close: string;
    };
    LocalAvailableInterval: {
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
    };
    LocalCatalogItem: {
      /** Format: uuid */
      space_id: string;
      /** @enum {string} */
      category_code:
        | "oficina"
        | "sala_multiproposito"
        | "bodega"
        | "estacionamiento"
        | "local_flexible"
        | "stand"
        | "quincho"
        | "parcela_eventos";
      category_name: string;
      title: string;
      description: string;
      area_m2: number;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      base_price_clp: number;
      /** @constant */
      currency: "CLP";
      time_zone: string;
      profile_version: number;
      profile: Record<string, never>;
      attributes: Record<string, never>;
      available?: boolean;
      /** @description Estimación inclusiva para el intervalo de búsqueda; null si no se indicó intervalo o si no hay zona válida. No es cotización ni disponibilidad. */
      estimated_total_clp?: number;
      /**
       * Format: float
       * @description Distancia geográfica directa aproximada a una décima de kilómetro; solo se incluye con filtro geográfico. No es distancia por carretera.
       */
      distance_km?: number;
      /**
       * @description Identifica distancia directa, no ruta vial.
       * @enum {string}
       */
      distance_kind?: "direct";
      map_position?: components["schemas"]["SyntheticMapPosition"];
    };
    /** @description Posición normalizada y relativa del plano sintético de esta página; no representa coordenadas geográficas, domicilio ni calles. */
    SyntheticMapPosition: {
      x: number;
      y: number;
    };
    LocalBookingQuoteEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalBookingQuote"];
    };
    LocalBookingQuote: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      space_id: string;
      rate_version: number;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      unit_price_clp: number;
      /** @constant */
      currency: "CLP";
      units: number;
      subtotal_clp: number;
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
      time_zone: string;
      conditions: string;
      /** @enum {string} */
      category_code:
        | "oficina"
        | "sala_multiproposito"
        | "bodega"
        | "estacionamiento"
        | "local_flexible"
        | "stand"
        | "quincho"
        | "parcela_eventos";
      profile_version: number;
      profile_values: Record<string, never>;
      /**
       * @description Política local snapshot del fixture sintético; no es política comercial.
       * @constant
       */
      cancellation_policy_version: "local_flexible_v1";
      /** @constant */
      guarantee_policy_version: "garantia_local_fija_v1";
      /** @constant */
      guarantee_currency: "CLP";
      /**
       * @description Snapshot exclusivo para nuevas cotizaciones del prototipo. No es un pago ni modifica reservas históricas.
       * @constant
       */
      guarantee_expected_clp: 50000;
      /** @constant */
      settlement_policy_version: "liquidacion_local_0pct_v1";
      /**
       * @description Comisión parametrizada del ensayo local; no decisión comercial productiva.
       * @constant
       */
      settlement_commission_bps: 0;
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      expires_at: string;
    };
    LocalBookingReservationEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalBookingReservation"];
    };
    LocalBookingTimeoutEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalBookingReservation"];
      error: {
        /** @constant */
        code: "simulated_payment_timeout";
        message: string;
      };
    };
    LocalBookingListEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: {
        items: components["schemas"]["LocalBookingReservation"][];
      };
    };
    LocalBookingDetailEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalBookingDetail"];
    };
    LocalBookingDetail: components["schemas"]["LocalBookingReservation"] & {
      history: components["schemas"]["LocalBookingTransition"][];
    };
    LocalBookingCancellationPreviewEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalBookingCancellationPreview"];
    };
    LocalBookingCancellationPreview: {
      /** Format: uuid */
      reservation_id: string;
      /** @constant */
      policy_version: "local_flexible_v1";
      eligible: boolean;
      /**
       * Format: date-time
       * @description Pago vence para pendiente_de_pago; inicio reservado para pagada/aprobada_host.
       */
      deadline: string;
      /** @description Subtotal efectivamente confirmado por el fake; sin comisiones ni garantías. */
      amount_clp: number;
      /** @constant */
      currency: "CLP";
      /** @constant */
      refund_label: "Devolución simulada — sin movimiento de dinero";
      /** @enum {string} */
      reason_code?:
        | "sin_devolucion"
        | "devolucion_simulada_completa"
        | "inicio_alcanzado"
        | "pago_fake_no_confirmado"
        | "estado_no_cancelable";
    };
    LocalBookingCancellationResultEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: {
        reservation: components["schemas"]["LocalBookingReservation"];
        refund_amount_clp?: number;
        /** @enum {string} */
        refund_state: "no_aplica" | "pendiente" | "completada";
        /** @enum {string} */
        notice_status:
          | "mailpit_local_no_durable"
          | "no_disponible_no_durable"
          | "no_enviado_no_durable"
          | "no_reintentado_por_idempotencia";
      };
    };
    LocalBookingRefundResultEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: {
        /** Format: uuid */
        reservation_id: string;
        /**
         * Format: uuid
         * @description Identidad estable que se reutiliza en todos los reintentos fake.
         */
        operation_id: string;
        amount_clp: number;
        /** @constant */
        currency: "CLP";
        /** @enum {string} */
        state: "pendiente" | "completada";
        /** @enum {string} */
        last_result:
          "exito_simulado" | "fallo_simulado" | "sin_respuesta_simulada";
        /** Format: date-time */
        updated_at: string;
        /** @enum {string} */
        notice_status:
          | "mailpit_local_no_durable"
          | "no_disponible_no_durable"
          | "no_enviado_no_durable"
          | "no_reintentado_por_idempotencia";
        /** @description La operación ya se había completado concurrentemente; se devuelve su resultado persistido y no se repite el aviso. */
        reused?: boolean;
      };
    };
    LocalBookingRefundTimeoutEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: {
        /** Format: uuid */
        reservation_id: string;
        /** Format: uuid */
        operation_id: string;
        amount_clp: number;
        /** @constant */
        currency: "CLP";
        /** @constant */
        state: "pendiente";
        /** @constant */
        last_result: "sin_respuesta_simulada";
        /** Format: date-time */
        updated_at: string;
        /** @enum {string} */
        notice_status:
          | "mailpit_local_no_durable"
          | "no_disponible_no_durable"
          | "no_enviado_no_durable";
      };
      error: {
        /** @constant */
        code: "simulated_refund_timeout";
        message: string;
      };
    };
    LocalBookingTransition: {
      /** Format: int64 */
      sequence: number | string;
      from?: string;
      to: string;
      /** Format: uuid */
      actor_id?: string;
      reason: string;
      /** Format: date-time */
      at: string;
    };
    LocalReservationMessageEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalReservationMessage"];
    };
    LocalReservationMessagesPageEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: {
        items: components["schemas"]["LocalReservationMessage"][];
        /** Format: int64 */
        older_cursor: number | string | null;
      };
    };
    LocalReservationReadCursorEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: {
        /** Format: int64 */
        read_through_sequence: number | string;
      };
    };
    LocalReservationMessage: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      reservation_id: string;
      /** Format: uuid */
      author_id: string;
      /** Format: int64 */
      sequence: number | string;
      body: string;
      /** Format: date-time */
      created_at: string;
    };
    LocalBookingReservation: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      quote_id: string;
      /** Format: uuid */
      space_id: string;
      /** Format: uuid */
      host_id: string;
      /** Format: uuid */
      renter_id: string;
      /** @enum {string} */
      state:
        | "pendiente_de_pago"
        | "pagada"
        | "aprobada_host"
        | "firma_parcial"
        | "lista_para_checkin"
        | "en_curso"
        | "finalizada"
        | "cerrada"
        | "en_disputa"
        | "cancelada_por_firma"
        | "cancelada_por_pago"
        | "rechazada_arrendador"
        | "vencida_pago"
        | "vencida_host"
        | "cancelada_arrendatario";
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      unit_price_clp: number;
      /** @constant */
      currency: "CLP";
      units: number;
      subtotal_clp: number;
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
      time_zone: string;
      conditions: string;
      /**
       * @description Snapshot local tomado desde la cotización.
       * @constant
       */
      cancellation_policy_version: "local_flexible_v1";
      /** @enum {string|null} */
      guarantee_policy_version?: "garantia_local_fija_v1" | null;
      /** @enum {string|null} */
      guarantee_currency?: "CLP" | null;
      /**
       * @description Ausente/null únicamente para reservas históricas previas a V40.
       * @enum {integer|null}
       */
      guarantee_expected_clp?: 50000 | null;
      /**
       * @description Ausente/null únicamente para reservas históricas previas a V44.
       * @enum {string|null}
       */
      settlement_policy_version?: "liquidacion_local_0pct_v1" | null;
      /**
       * @description Snapshot local; no asignado retroactivamente a reservas históricas.
       * @enum {integer|null}
       */
      settlement_commission_bps?: 0 | null;
      /** Format: uuid */
      refund_id?: string;
      /** Format: uuid */
      refund_operation_id?: string;
      refund_amount_clp?: number;
      /** @enum {string} */
      refund_state?: "pendiente" | "completada";
      /** @enum {string} */
      refund_last_result?:
        "exito_simulado" | "fallo_simulado" | "sin_respuesta_simulada";
      /** Format: date-time */
      refund_updated_at?: string;
      /** Format: date-time */
      pay_expires_at: string;
      /** Format: date-time */
      host_expires_at?: string | null;
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      updated_at: string;
      unread_count: number;
    };
    LocalGuaranteeEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalGuarantee"];
    };
    LocalSettlementEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN VALIDEZ FISCAL — SIN MOVIMIENTOS REALES";
      data: components["schemas"]["LocalSettlement"];
    };
    LocalSettlementReceiptEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN VALIDEZ FISCAL — SIN MOVIMIENTOS REALES";
      data: components["schemas"]["LocalSettlementReceipt"];
    };
    LocalSettlement: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      reservation_id: string;
      /** @constant */
      policy_version: "liquidacion_local_0pct_v1";
      /** @constant */
      currency: "CLP";
      gross_rent_clp: number;
      confirmed_refunds_clp: number;
      commission_bps: number;
      commission_clp: number;
      transfer_clp: number;
      /** @enum {string} */
      state: "pendiente" | "por_conciliar" | "liquidada" | "rechazada";
      /** Format: uuid */
      operation_id: string;
      /** @enum {string} */
      last_result?:
        "exito_simulado" | "rechazo_simulado" | "sin_respuesta_simulada";
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      updated_at: string;
      reused?: boolean;
    };
    LocalSettlementReceipt: {
      /** @constant */
      version: 1;
      /** @constant */
      kind: "comprobante_ensayo";
      /** @constant */
      banner: "ENSAYO LOCAL — SIN VALIDEZ FISCAL — SIN MOVIMIENTOS REALES";
      /**
       * Format: uuid
       * @description Identificador técnico interno sin folio fiscal.
       */
      receipt_id: string;
      /** Format: uuid */
      reservation_id: string;
      /** @constant */
      policy_version: "liquidacion_local_0pct_v1";
      /** @constant */
      currency: "CLP";
      gross_rent_clp: number;
      confirmed_refunds_clp: number;
      commission_clp: number;
      transfer_clp: number;
      /** Format: date-time */
      confirmed_at: string;
    };
    LocalGuaranteeOperationEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalGuaranteeOperation"];
    };
    LocalFinancialDecisionEnvelope: {
      /** @constant */
      safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
      data: components["schemas"]["LocalFinancialDecision"];
    };
    LocalGuarantee: {
      /** @constant */
      policy_version: "garantia_local_fija_v1";
      /** @constant */
      currency: "CLP";
      /** @constant */
      expected_clp: 50000;
      authorized_clp: number;
      captured_clp: number;
      released_clp: number;
      /** @enum {string} */
      state:
        | "pendiente_pago"
        | "pendiente_autorizacion"
        | "autorizada"
        | "no_disponible"
        | "por_conciliar"
        | "captura_pendiente"
        | "captura_por_conciliar"
        | "parcialmente_capturada"
        | "liberacion_pendiente"
        | "liberacion_por_conciliar"
        | "liberada"
        | "cancelada";
      /** Format: date-time */
      authorization_deadline?: string;
      operations: components["schemas"]["LocalGuaranteeOperation"][];
      financial_decision?: components["schemas"]["LocalFinancialDecision"];
    };
    LocalGuaranteeOperation: {
      /** Format: uuid */
      id: string;
      /** @enum {string} */
      kind: "autorizacion" | "captura" | "liberacion";
      amount_clp: number;
      /** @enum {string} */
      state:
        "pendiente" | "por_conciliar" | "confirmada" | "rechazada" | "vencida";
      last_result?: string;
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      updated_at: string;
    };
    LocalFinancialDecision: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      claim_id?: string;
      /** @enum {string} */
      outcome: "acogido" | "rechazado";
      deduction_clp: number;
      /** @enum {string} */
      reason_code: "dano_acreditado" | "faltante_acreditado" | "sin_deduccion";
      /** Format: uuid */
      evidence_id?: string;
      /** @enum {string} */
      state: "pendiente" | "por_conciliar" | "aplicada" | "sin_deduccion";
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      updated_at: string;
    };
    SpaceCategory: {
      code: string;
      name: string;
    };
    SpaceCategoryList: {
      items: components["schemas"]["SpaceCategory"][];
    };
    SpaceDraftInput: {
      category_code: string;
      title: string;
      description: string;
      area_m2: number;
      capacity: number;
      usage_rules: string;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      base_price_clp: number;
      /** @description Dirección privada del borrador. */
      address: string;
      /** @description Código de región del catálogo público SUBDERE_DPA_2018; omitir ambos códigos deja la ubicación pública sin declarar. */
      region_code?: string;
      /** @description Código de comuna de la región seleccionada; debe enviarse junto con region_code. */
      comuna_code?: string;
      /** @description Versión del perfil de atributos; se asume la vigente si se omite. */
      attribute_schema_version?: number;
      /** @description Valores opcionales declarados; claves y tipos se validan contra el perfil de categoría. */
      attributes?: Record<string, never>;
    };
    SpaceDraft: {
      /** Format: uuid */
      id: string;
      category_code: string;
      category_name: string;
      title: string;
      description: string;
      area_m2: number;
      capacity: number;
      usage_rules: string;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      base_price_clp: number;
      address: string;
      /** @enum {string} */
      state: "borrador" | "activa" | "oculta";
      /** @description Zona IANA propia; puede ser nula en datos heredados pendientes de reparación. */
      time_zone?: string | null;
      /** @description Código regional público declarado por el anfitrión; ausente para espacios anteriores sin ubicación pública. */
      region_code?: string;
      /** @description Código comunal público declarado por el anfitrión; ausente para espacios anteriores sin ubicación pública. */
      comuna_code?: string;
      attribute_schema_version: number;
      /** @description Características declaradas y validadas por el perfil versionado. */
      attributes: Record<string, never>;
      /**
       * Format: uuid
       * @description Identificador de portada sintética activa propia; no revela contenido ni URL del archivo privado.
       */
      cover_photo_id?: string | null;
    };
    SyntheticSpaceCover: {
      /** Format: uuid */
      space_id: string;
      /** Format: uuid */
      photo_id: string;
      /** @constant */
      selected: true;
    };
    SyntheticSpaceGalleryPhoto: {
      /** Format: uuid */
      id: string;
      /** @constant */
      fixture_code: "synthetic-png-v1";
      /** @constant */
      mime_type: "image/png";
      sha256: string;
      size_bytes: number;
      /** Format: date-time */
      created_at: string;
      public_in_catalog: boolean;
      position?: number;
    };
    SpaceMediaPhoto: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      space_id: string;
      alt_text: string;
      visible: boolean;
      cover: boolean;
      position: number;
      /** @enum {string} */
      state: "ready";
      /** Format: date-time */
      created_at: string;
      variants: components["schemas"]["SpaceMediaVariant"][];
    };
    SpaceMediaVariant: {
      /** @enum {string} */
      name: "display" | "thumbnail" | "avatar";
      /** Format: uri-reference */
      url: string;
      /** @constant */
      mime_type: "image/webp";
      size_bytes: number;
      sha256: string;
      width: number;
      height: number;
    };
    SpaceMediaPhotoEnvelope: {
      photo: components["schemas"]["SpaceMediaPhoto"];
      reused: boolean;
    };
    SyntheticSpaceGalleryPhotoEnvelope: {
      photo: components["schemas"]["SyntheticSpaceGalleryPhoto"];
      reused: boolean;
      /** @constant */
      synthetic: true;
    };
    SyntheticSpaceGalleryRemoval: {
      /** Format: uuid */
      photo_id: string;
      /** @constant */
      removed: true;
      reused: boolean;
    };
    SpacePublicationInput: {
      /** @enum {string} */
      state: "activa" | "oculta";
    };
    PublishedSpaceContentInput: {
      title?: string;
      description?: string;
      /** Format: int32 */
      capacity?: number;
      usage_rules?: string;
      /** Format: int64 */
      base_price_clp?: number | string;
      /** @description Enviar junto a comuna_code para cambiar ubicación; enviar ambos vacíos la retira del catálogo público. */
      region_code?: string;
      /** @description Código de comuna dentro de region_code; enviar ambos vacíos retira la ubicación. */
      comuna_code?: string;
    };
    AvailabilityConfig: {
      /** @description Nombre de zona horaria IANA validado en el servidor. */
      time_zone: string;
    };
    AvailabilityConfigResponse: {
      /** Format: uuid */
      space_id: string;
      time_zone: string;
    };
    AvailabilityResult: {
      /** Format: uuid */
      space_id: string;
      time_zone: string;
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
      available: boolean;
    };
    RateInput: {
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      base_price: number;
    };
    Rate: {
      /** Format: uuid */
      space_id: string;
      version: number;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      base_price: number;
      /** @constant */
      currency: "CLP";
      /** Format: date-time */
      created_at: string;
    };
    RateList: {
      items: components["schemas"]["Rate"][];
    };
    PriceSimulationInput: {
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
    };
    PriceSimulation: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      space_id: string;
      rate_version: number;
      /** @enum {string} */
      rate_unit: "hora" | "dia" | "mes";
      base_price: number;
      billed_units: number;
      /** @constant */
      currency: "CLP";
      base_subtotal: number;
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
      time_zone: string;
      /** @constant */
      private: true;
      /** Format: date-time */
      created_at: string;
    };
    ManualBlockInput: {
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
      reason: string;
    };
    ManualBlock: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      space_id: string;
      /** Format: date-time */
      start_at: string;
      /** Format: date-time */
      end_at: string;
      time_zone: string;
      reason: string;
      /** Format: date-time */
      created_at: string;
    };
    ManualBlockList: {
      /** Format: uuid */
      space_id: string;
      time_zone: string;
      items: {
        /** Format: uuid */
        id: string;
        /** Format: uuid */
        space_id: string;
        /** Format: date-time */
        start_at: string;
        /** Format: date-time */
        end_at: string;
        time_zone: string;
        reason: string;
        /** Format: date-time */
        created_at: string;
      }[];
    };
    SpaceAttributeProfile: {
      category_code: string;
      schema_version: number;
      attributes: {
        code: string;
        label: string;
        description?: string;
        /** @enum {string} */
        type: "boolean" | "integer" | "number" | "enum" | "enum_list";
        required?: boolean;
        order: number;
        unit?: string;
        options?: string[];
        minimum?: number;
        maximum?: number;
        step?: number;
        filter_candidate: boolean;
      }[];
    };
    SpaceDeletionReceipt: {
      /** Format: uuid */
      space_id: string;
      /** @enum {string} */
      state: "eliminada";
      /** Format: date-time */
      occurred_at: string;
      removed: string[];
      preserved: string[];
      files_pending_cleanup: number;
      reused: boolean;
    };
    SpaceDraftList: {
      items: components["schemas"]["SpaceDraft"][];
    };
    Profile: {
      display_name: string;
      phone?: string | null;
      /** Format: date-time */
      updated_at: string;
    };
    ProfileUpdate: {
      display_name: string;
      phone?: string;
    };
    RightsRequestCreate: {
      /** @enum {string} */
      type: "acceso" | "supresion";
    };
    RightsRequest: {
      /** Format: uuid */
      id: string;
      /** @enum {string} */
      kind: "acceso" | "supresion";
      /** @enum {string} */
      state:
        | "recibida"
        | "identidad_pendiente"
        | "en_revision"
        | "resuelta"
        | "denegada_fundada";
      /** Format: date-time */
      created_at: string;
    };
    RightsRequestList: {
      items: components["schemas"]["RightsRequest"][];
    };
    OwnIdentityExport: {
      /** @constant */
      scope: "identidad_local_v1";
      account: {
        /** Format: email */
        email: string;
        state: string;
        /** @enum {string|null} */
        use_preference?: "ofrecer" | "arrendar" | null;
        /** Format: date-time */
        created_at: string;
      };
      profile?: {
        display_name: string;
        phone?: string | null;
        /** Format: date-time */
        updated_at: string;
      };
      roles: (
        "arrendatario" | "arrendador" | "administrador" | "administrador_global"
      )[];
      terms_acceptances: {
        code: string;
        /** @enum {string} */
        type: "terminos" | "privacidad" | "politica_arrendador";
        /** Format: date-time */
        accepted_at: string;
      }[];
      rights_requests: components["schemas"]["RightsRequest"][];
    };
    VerificationCreate: {
      /** @enum {string} */
      type: "kyc" | "kyb";
    };
    VerificationRetry: {
      /** @enum {string} */
      correction_code:
        | "fixture_vigente_actualizado"
        | "antecedentes_fixture_actualizados"
        | "inicio_actividades_fixture_actualizadas";
    };
    SyntheticEvidenceCreate: {
      /** @enum {string} */
      fixture_code: "synthetic-png-v1";
    };
    SyntheticEvidence: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      verification_id: string;
      /** @enum {string} */
      fixture_code: "synthetic-png-v1";
      /** @enum {string} */
      mime_type: "image/png";
      size_bytes: number;
      sha256: string;
      /** Format: date-time */
      created_at: string;
      /** @description Falso cuando el archivo fue retirado por vencimiento y solo se conserva metadata temporal. */
      file_available: boolean;
    };
    SyntheticEvidenceList: {
      items: components["schemas"]["SyntheticEvidence"][];
    };
    VerificationReview: {
      /** @enum {string} */
      decision: "aprobada" | "rechazada";
      /** @enum {string} */
      reason_code?:
        | "documento_vencido"
        | "antecedentes_incompletos"
        | "inicio_actividades_no_confirmado";
    };
    VerificationCase: {
      /** Format: uuid */
      id: string;
      /** @enum {string} */
      type: "kyc" | "kyb";
      /** @enum {string} */
      state: "en_revision" | "aprobada" | "rechazada" | "revocada";
      /** @constant */
      provider: "local-fixture-v1";
      evidence_ref: string;
      /** Format: uuid */
      retry_of?: string;
      reason_code?: string;
      /** @enum {string} */
      correction_code?:
        | "fixture_vigente_actualizado"
        | "antecedentes_fixture_actualizados"
        | "inicio_actividades_fixture_actualizadas"
        | "legado_pre_v28_sin_codigo";
      /** Format: date-time */
      revoked_at?: string | null;
      /** @enum {string} */
      revocation_reason_code?:
        "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada";
      /** Format: date-time */
      created_at: string;
      /** Format: date-time */
      resolved_at?: string | null;
    };
    VerificationEligibilityList: {
      /** @constant */
      scope: "synthetic_local";
      items: {
        /** @enum {string} */
        type: "kyc" | "kyb";
        eligible: boolean;
        verification_id: string;
        /** Format: date-time */
        granted_at: string | null;
        /** Format: date-time */
        revoked_at: string | null;
        revocation_reason_code: string;
      }[];
    };
    VerificationHistoryList: {
      items: {
        /** Format: int64 */
        sequence: number | string;
        /** @enum {string} */
        action:
          | "solicitada"
          | "revision_aprobada"
          | "revision_rechazada"
          | "subsanacion_solicitada"
          | "elegibilidad_revocada"
          | "elegibilidad_retirada_privacidad"
          | "estado_observado_en_migracion";
        from_state?: string;
        /** @enum {string} */
        to_state:
          | "en_revision"
          | "aprobada"
          | "rechazada"
          | "revocada"
          | "retirada_privacidad";
        reason_code?: string;
        /** Format: date-time */
        occurred_at: string;
      }[];
    };
    VerificationList: {
      items: components["schemas"]["VerificationCase"][];
    };
    Error: {
      error: components["schemas"]["ErrorDetail"];
    };
    ErrorDetail: {
      /**
       * @description Código estable, legible por máquina y documentado por operación.
       * @example validation_error
       */
      code: string;
      /**
       * @description Mensaje seguro y no sensible para presentación o diagnóstico.
       * @example La solicitud contiene campos inválidos.
       */
      message: string;
      /** Format: uuid */
      request_id: string;
      /** @description Detalles seguros, opcionales y específicos del error. Registro/acceso usan email:invalid_email, email:already_registered, password:required, password:password_policy y terms_version_ids:consent_invalid. Los borradores y el contenido publicado pueden devolver field/code para category_code, title, description, area_m2, capacity, usage_rules, rate_unit, base_price_clp, address, region_code, comuna_code, attribute_schema_version, attributes o attributes.<código de perfil>. Nunca incluye valores enviados ni errores internos. */
      details?: components["schemas"]["FieldError"][];
    };
    FieldError: {
      field: string;
      code: string;
      message?: string;
    };
    /** @description Convención para colecciones; cada endpoint define su elemento. */
    Page: {
      items: unknown[];
      /** @description Cursor opaco; null cuando no existen más resultados. */
      next_cursor: string | null;
    };
    Money: {
      /**
       * @description Decimal exacto serializado como string, sin notación exponencial; escala y redondeo se definen por operación.
       * @example 123.45
       */
      amount: string;
      /** @description Código de moneda ISO 4217; cada operación fija las monedas admitidas. */
      currency: string;
    };
    /**
     * Format: date-time
     * @description Instante RFC 3339 normalizado a UTC y terminado en Z.
     * @example 2026-10-02T12:30:00Z
     */
    UtcTimestamp: string;
    /**
     * Format: date
     * @description Fecha civil ISO 8601 sin hora ni zona horaria.
     * @example 2026-10-02
     */
    CivilDate: string;
    LocalTrialDispute: {
      /** Format: uuid */
      id: string;
      /** Format: uuid */
      reservation_id: string;
      /** Format: uuid */
      host_id: string;
      /** Format: uuid */
      renter_id: string;
      /** Format: uuid */
      opened_by: string;
      /** @constant */
      opening_reason_code: "ensayo_privacidad";
      /** @enum {string} */
      state: "abierta" | "cerrada";
      /** Format: date-time */
      opened_at: string;
      /** Format: uuid */
      closed_by?: string;
      /** @enum {string} */
      close_reason_code?:
        "ensayo_finalizado" | "registro_erroneo" | "duplicada";
      /** Format: date-time */
      closed_at?: string;
      reused: boolean;
    };
    LocalTrialDisputeTransition: {
      /** Format: int64 */
      sequence: number | string;
      /** @enum {string} */
      previous_state?: "abierta" | "cerrada";
      /** @enum {string} */
      new_state: "abierta" | "cerrada";
      /** Format: uuid */
      actor_id: string;
      /** @enum {string} */
      reason_code:
        | "ensayo_privacidad"
        | "ensayo_finalizado"
        | "registro_erroneo"
        | "duplicada";
      /** Format: date-time */
      occurred_at: string;
    };
    RegisterRequest: {
      /** Format: email */
      email: string;
      /** @description RQF-003–006; mayúscula, número, especial; máximo bcrypt 72 bytes. */
      password: $Write<string>;
      /** @description Debe contener exactamente una versión vigente de Términos y condiciones y una de Política de privacidad. */
      terms_version_ids: string[];
    };
    EmailRequest: {
      /** Format: email */
      email: string;
    };
    VerificationRequest: {
      /** Format: uuid */
      token_id: string;
      token: $Write<string>;
    };
    ActionLinkStatusRequest: {
      /** Format: uuid */
      token_id: string;
      token: $Write<string>;
    };
    ActionLinkStatusResponse: {
      /** @enum {string} */
      status: "ready" | "used" | "expired" | "invalid";
    };
    RecoveryConsumeRequest: {
      /** Format: uuid */
      token_id: string;
      token: $Write<string>;
      /** @description Contraseña conforme a la política de M01. */
      new_password: $Write<string>;
      confirm_password: $Write<string>;
    };
    PasswordChangeRequest: {
      current_password: $Write<string>;
      new_password: $Write<string>;
      confirm_password: $Write<string>;
    };
    LoginRequest: {
      /** Format: email */
      email: string;
      password: $Write<string>;
    };
    AccountCreated: {
      /** Format: uuid */
      account_id: string;
      /** @enum {string} */
      status: "email_pending" | "verification_pending";
      message?: string;
    };
    LoginResponse: {
      /** Format: uuid */
      account_id: string;
      roles: (
        "arrendatario" | "arrendador" | "administrador" | "administrador_global"
      )[];
      capabilities: components["schemas"]["AdminCapability"][];
      /** @description Bearer opaco; solo en esta respuesta, nunca URL/log. Mantener en memoria en mock. */
      access_token: $Read<string>;
      /** @constant */
      token_type: "Bearer";
      expires_at: components["schemas"]["UtcTimestamp"];
      /** @description true si el Backend permite únicamente acceso restringido a reservas existentes. */
      restricted_mode: boolean;
    };
    SessionResponse: {
      /** Format: uuid */
      account_id: string;
      roles: (
        "arrendatario" | "arrendador" | "administrador" | "administrador_global"
      )[];
      capabilities: components["schemas"]["AdminCapability"][];
      /** @description La cuenta sigue bloqueada administrativamente y requiere operaciones de reserva existentes. */
      restricted_mode: boolean;
    };
    TermsResponse: {
      items: {
        /** Format: uuid */
        id: string;
        code: string;
        /** @enum {string} */
        type: "terminos" | "privacidad";
        sha256: string;
        published_at: components["schemas"]["UtcTimestamp"];
        /** @constant */
        synthetic: true;
        document: {
          id: string;
          code: string;
          type: string;
          title: string;
          scope: string;
          status: string;
          sections: {
            heading: string;
            body: string;
          }[];
        };
      }[];
    };
    RecoveryAccepted: {
      /** @constant */
      status: "accepted";
      message: string;
    };
  };
  responses: {
    /** @description Solicitud mal formada o parámetros inválidos. */
    BadRequest: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description Credencial ausente, inválida o expirada; se envía un desafío aplicable. */
    Unauthorized: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        "WWW-Authenticate": components["headers"]["WWW-Authenticate"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description Identidad autenticada sin permiso para la acción o recurso. */
    Forbidden: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description Recurso inexistente o no visible para esta identidad. */
    NotFound: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description Conflicto con el estado actual, versión o clave de idempotencia. */
    Conflict: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description Error inesperado; no se exponen detalles internos ni secretos. */
    InternalServerError: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description validation_error o invalid_token: regla semántica no satisfecha. */
    UnprocessableEntity: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description rate_limited o login_blocked. Retry-After en segundos. */
    TooManyRequests: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        "Retry-After"?: string;
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description mail_unavailable: solicitar reenvío al recuperar el correo local. */
    ServiceUnavailable: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description Usar Content-Type application/json. */
    UnsupportedMediaType: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
    /** @description Método no admitido. */
    MethodNotAllowed: {
      headers: {
        "X-Request-ID": components["headers"]["X-Request-ID"];
        [name: string]: unknown;
      };
      content: {
        "application/json": components["schemas"]["Error"];
      };
    };
  };
  parameters: {
    /** @description Clave opaca que identifica una misma operación para reintentos seguros. */
    IdempotencyKey: string;
  };
  requestBodies: never;
  headers: {
    /** @description Identificador de correlación generado o propagado por el servidor. */
    "X-Request-ID": string;
    /**
     * @description Desafío Bearer que acompaña una respuesta 401.
     * @example Bearer
     */
    "WWW-Authenticate": string;
    /** @description Clave opaca del cliente para repetir de forma segura una operación elegible. */
    "Idempotency-Key": string;
  };
  pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
  getGlobalAdminMe: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Sesión global confirmada */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["GlobalAdminMeEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
    };
  };
  getGlobalAdminOverview: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Agregados actuales; activo no equivale a DAU */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["GlobalAdminOverviewEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
    };
  };
  getGlobalAdminKPIs: {
    parameters: {
      query: {
        from: string;
        /** @description Exclusivo */
        until: string;
        time_zone: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Indicadores persistidos y auditados */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": Record<string, never>;
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listGlobalAdminAuditEvents: {
    parameters: {
      query: {
        from: string;
        until: string;
        actor_id?: string;
        target_account_id?: string;
        resource_type?: string;
        resource_id?: string;
        action?: string;
        result?: "exito" | "rechazo";
        reason_code?: string;
        correlation_id?: string;
        page_size?: number;
        cursor?: string;
        /** @description Corte registrado devuelto por la primera página; debe coincidir con el corte codificado en el cursor. */
        snapshot_cutoff?: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Página auditada */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": Record<string, never>;
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  exportGlobalAdminAuditEvents: {
    parameters: {
      query: {
        from: string;
        until: string;
        actor_id?: string;
        target_account_id?: string;
        resource_type?: string;
        resource_id?: string;
        action?: string;
        result?: "exito" | "rechazo";
        reason_code?: string;
        correlation_id?: string;
        /** @description Corte obtenido con la página consultada; la generación del archivo queda reportada por separado. */
        snapshot_cutoff: string;
        format?: "json" | "csv";
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Archivo exportado sin cache */
      200: {
        headers: {
          "Content-Disposition"?: string;
          [name: string]: unknown;
        };
        content: {
          "application/json": Record<string, never>;
          "text/csv": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      /** @description Filtros inválidos o exceso de filas */
      422: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  listGlobalAdminAccounts: {
    parameters: {
      query?: {
        account_id?: string;
        email_prefix?: string;
        name_prefix?: string;
        role?:
          | "arrendatario"
          | "arrendador"
          | "administrador"
          | "administrador_global";
        state?:
          | "correo_pendiente"
          | "activo"
          | "bloqueado"
          | "baja_solicitada"
          | "desidentificado";
        administrative_blocked?: boolean;
        /** @description RFC3339 inclusivo UTC */
        created_from?: string;
        /** @description RFC3339 exclusivo UTC */
        created_until?: string;
        page_size?: number;
        cursor?: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Página mínima y conteo */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["GlobalAdminAccountPageEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getGlobalAdminAccount: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Cuenta y conteos de recursos */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["GlobalAdminAccountEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  updateGlobalAdminAccountProfile: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          display_name: string;
          phone?: string | null;
          /** Format: date-time */
          expected_updated_at: string;
          /** @enum {string} */
          reason:
            | "correccion_perfil"
            | "revision_administrativa"
            | "gestion_operativa";
        };
      };
    };
    responses: {
      /** @description Perfil actualizado y auditado */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  setGlobalAdminMarketplaceRole: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
        role: "arrendatario" | "arrendador";
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          granted: boolean;
          /** @enum {string} */
          reason:
            | "correccion_perfil"
            | "revision_administrativa"
            | "gestion_operativa";
        };
      };
    };
    responses: {
      /** @description Estado deseado aplicado o ya vigente; evento auditado */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  setGlobalAdminLegacyAdministratorRole: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          granted: boolean;
          /** @enum {string} */
          reason:
            | "aprovisionamiento_sintetico"
            | "revision_administrativa"
            | "retirada_acceso";
        };
      };
    };
    responses: {
      /** @description Estado deseado aplicado o ya vigente; evento auditado */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  revokeGlobalAdminAccountSessions: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          reason:
            | "correccion_perfil"
            | "revision_administrativa"
            | "gestion_operativa";
        };
      };
    };
    responses: {
      /** @description Sesiones revocadas y acción auditada */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  setGlobalAdminAccountBlocked: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          blocked: boolean;
          /** @enum {string} */
          reason:
            | "riesgo_seguridad"
            | "uso_indebido"
            | "revision_administrativa"
            | "revision_concluida"
            | "error_bloqueo"
            | "riesgo_resuelto";
        };
      };
    };
    responses: {
      /** @description Estado cambiado; historial y auditoría atómicos */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  listGlobalAdminAccountResources: {
    parameters: {
      query?: {
        page_size?: number;
        cursor?: string;
      };
      header?: never;
      path: {
        accountID: string;
        resourceKind: "reservations" | "incidents";
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Recursos vinculados a la cuenta */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  getGlobalAdminAccountResource: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
        resourceKind: "reservations" | "incidents";
        resourceID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Recurso autorizado con datos mínimos necesarios */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  listGlobalAdminPublications: {
    parameters: {
      query?: {
        owner_id?: string;
        category?: string;
        state?: "borrador" | "activa" | "oculta" | "eliminada";
        search?: string;
        publicly_visible?: boolean;
        page_size?: number;
        cursor?: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Inventario paginado; visible distingue el estado activa de la política pública vigente */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["GlobalAdminPublicationPageEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  changeGlobalAdminPublicationState: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        publicationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          state: "activa" | "oculta";
          /** Format: date-time */
          expected_updated_at: string;
          /** @enum {string} */
          reason:
            | "correccion_perfil"
            | "revision_administrativa"
            | "gestion_operativa";
        };
      };
    };
    responses: {
      /** @description Estado cambiado con reglas de publicación e historial transaccional */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  updateGlobalAdminPublicationContent: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        publicationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          title?: string;
          description?: string;
          capacity?: number;
          usage_rules?: string;
          base_price_clp?: string;
          region_code?: string;
          comuna_code?: string;
          /** Format: uuid */
          cover_photo_id?: string;
          attribute_schema_version?: number;
          attributes?: {
            [key: string]: unknown;
          };
          /** Format: date-time */
          expected_updated_at: string;
          /** @enum {string} */
          reason:
            | "correccion_perfil"
            | "revision_administrativa"
            | "gestion_operativa";
        };
      };
    };
    responses: {
      /** @description Campos allowlisted actualizados y auditados */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  retireGlobalAdminPublication: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        publicationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          owner_id: string;
          /** Format: date-time */
          expected_updated_at: string;
          /** @enum {string} */
          reason:
            | "moderacion_contenido"
            | "riesgo_seguridad"
            | "solicitud_titular"
            | "revision_administrativa";
        };
      };
    };
    responses: {
      /** @description Retiro append-only con actor administrativo y limpieza durable */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  getGlobalAdminPublicationSchedule: {
    parameters: {
      query: {
        owner_id: string;
      };
      header?: never;
      path: {
        publicationID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Horario semanal vigente */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      403: components["responses"]["Forbidden"];
    };
  };
  updateGlobalAdminPublicationSchedule: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        publicationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          owner_id: string;
          enabled: boolean;
          days: unknown[];
          /** Format: date-time */
          expected_updated_at: string;
          /** @enum {string} */
          reason:
            | "correccion_perfil"
            | "revision_administrativa"
            | "gestion_operativa";
        };
      };
    };
    responses: {
      /** @description Horario semanal y auditoría guardados transaccionalmente */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  listGlobalAdminPublicationGallery: {
    parameters: {
      query: {
        owner_id: string;
      };
      header?: never;
      path: {
        publicationID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Metadatos minimizados de galería con portada y limpieza pendiente */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  setGlobalAdminPublicationPhotoVisibility: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        publicationID: string;
        photoID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          owner_id: string;
          enabled: boolean;
          /** Format: date-time */
          expected_updated_at: string;
          reason: string;
        };
      };
    };
    responses: {
      /** @description Visibilidad de foto actualizada y auditada */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  selectGlobalAdminPublicationCover: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        publicationID: string;
        photoID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          owner_id: string;
          /** Format: date-time */
          expected_updated_at: string;
          reason: string;
        };
      };
    };
    responses: {
      /** @description Portada seleccionada */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  removeGlobalAdminPublicationPhoto: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        publicationID: string;
        photoID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          owner_id: string;
          /** Format: date-time */
          expected_updated_at: string;
          reason: string;
        };
      };
    };
    responses: {
      /** @description Foto retirada con limpieza durable */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      409: components["responses"]["Conflict"];
      /** @description Se requiere sesión reciente */
      428: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  listPublicSpaces: {
    parameters: {
      query?: {
        /** @description Texto NFC; máximo 120 caracteres. Stopwords o puntuación sin lexemas dan una página vacía. */
        q?: string;
        category_code?: string;
        region_code?: string;
        comuna_code?: string;
        /** @description Superficie positiva en m² */
        min_area_m2?: number;
        /** @description Repetible hasta 32 veces y requiere category_code. Booleanos conservan `codigo:true|false`; integer/number usan `codigo:gte:valor` y/o `codigo:lte:valor`; enum usa `codigo:eq:opcion`; enum_list usa `codigo:in:opcion1,opcion2` (OR dentro del atributo). Códigos, operadores, opciones, límites y versión se validan contra la allowlist pública y el perfil; duplicados ambiguos dan 422. */
        attribute?: string[];
        rate_unit?: "hora" | "dia" | "mes";
        min_unit_price_clp?: number | string;
        max_unit_price_clp?: number | string;
        /** @description RFC3339 con offset. Ambos extremos habilitan estimación con la tarifa y zona horaria vigentes. */
        start_at?: string;
        end_at?: string;
        min_total_clp?: number | string;
        max_total_clp?: number | string;
        /** @description default ordena por categoría y título; title_asc/title_desc comparan título normalizado con space_id estable. Las tarifas requieren rate_unit explícito, relevance requiere q y total estimado requiere inicio/fin. */
        sort?:
          | "default"
          | "title_asc"
          | "title_desc"
          | "unit_price_asc"
          | "unit_price_desc"
          | "relevance"
          | "estimated_total_asc"
          | "estimated_total_desc";
        /** @description Tamaño de página limitado por cursor keyset. Conserva valores históricos 1–25 y admite hasta 48. */
        page_size?: number;
        cursor?: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Página pública minimizada */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["PublicSpacePage"];
        };
      };
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getPublicSpaceCategoryFilters: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        code: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Filtros tipados y versionados de la categoría */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["PublicCategoryFilters"];
        };
      };
      404: components["responses"]["NotFound"];
    };
  };
  listPublicLocations: {
    parameters: {
      query?: {
        q?: string;
        region_code?: string;
        limit?: number;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Comunas del snapshot DPA activo */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["PublicLocations"];
        };
      };
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getPublicSpace: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Detalle público minimizado */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["PublicSpaceDetailEnvelope"];
        };
      };
      404: components["responses"]["NotFound"];
    };
  };
  getPublicSpaceImage: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceID: string;
        imageID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description PNG público */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "image/png": string;
        };
      };
      404: components["responses"]["NotFound"];
    };
  };
  getPublishedSpaceMediaVariant: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceID: string;
        imageID: string;
        variant: "display" | "thumbnail";
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Variante WebP publicada. */
      200: {
        headers: {
          ETag?: string;
          "Cache-Control"?: string;
          [name: string]: unknown;
        };
        content: {
          "image/webp": string;
        };
      };
      404: components["responses"]["NotFound"];
    };
  };
  listPublicSpaceCategories: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Categorías activas */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["PublicSpaceCategories"];
        };
      };
    };
  };
  listActiveSpaceReviews: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Reseñas y promedio público minimizado */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalSpaceReviewsEnvelope"];
        };
      };
      404: components["responses"]["NotFound"];
    };
  };
  listReservationReviews: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Reseñas visibles para participantes */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReviewListEnvelope"];
        };
      };
      404: components["responses"]["NotFound"];
    };
  };
  createReservationReview: {
    parameters: {
      query?: never;
      header: {
        /** @description Clave opaca que identifica una misma operación para reintentos seguros. */
        "Idempotency-Key": components["parameters"]["IdempotencyKey"];
      };
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          rating: number;
          comment?: string;
        };
      };
    };
    responses: {
      /** @description Reintento idéntico reutiliza la reseña */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReviewEnvelope"];
        };
      };
      /** @description Reseña creada */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReviewEnvelope"];
        };
      };
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  reportSpaceReview: {
    parameters: {
      query?: never;
      header: {
        /** @description Clave opaca que identifica una misma operación para reintentos seguros. */
        "Idempotency-Key": components["parameters"]["IdempotencyKey"];
      };
      path: {
        reservationID: string;
        reviewID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          reason_code:
            | "insultos_acoso"
            | "datos_personales"
            | "spam"
            | "ajeno_experiencia";
        };
      };
    };
    responses: {
      /** @description Reintento idéntico reutiliza el reporte */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReviewReportEnvelope"];
        };
      };
      /** @description Reporte creado y reseña marcada reportada */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReviewReportEnvelope"];
        };
      };
      409: components["responses"]["Conflict"];
    };
  };
  getOwnRenterReputation: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Calificaciones minimizadas recibidas por la cuenta */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  listPendingReviewReports: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Cola local de moderación */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReviewReportListEnvelope"];
        };
      };
    };
  };
  moderateReviewReport: {
    parameters: {
      query?: never;
      header: {
        /** @description Clave opaca que identifica una misma operación para reintentos seguros. */
        "Idempotency-Key": components["parameters"]["IdempotencyKey"];
      };
      path: {
        reportID: string;
        action: "dismiss" | "hide";
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          reason_code:
            | "sin_infraccion"
            | "contenido_inadecuado"
            | "duplicado"
            | "error_registro";
          correlation_id: string;
        };
      };
    };
    responses: {
      /** @description Decisión de moderación persistida */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReviewReportEnvelope"];
        };
      };
      403: components["responses"]["Forbidden"];
    };
  };
  listTerminalLocalNotices: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Eventos terminales locales sin secretos ni contenido SMTP */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalNoticeListEnvelope"];
        };
      };
    };
  };
  reopenLocalNotice: {
    parameters: {
      query?: never;
      header: {
        /** @description Clave opaca que identifica una misma operación para reintentos seguros. */
        "Idempotency-Key": components["parameters"]["IdempotencyKey"];
      };
      path: {
        noticeID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          reason_code: "smtp_restaurado" | "reintento_operativo";
          correlation_id: string;
        };
      };
    };
    responses: {
      /** @description Reintento idempotente reutilizado */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalNoticeRecoveryEnvelope"];
        };
      };
      /** @description Nuevo ciclo durable */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalNoticeRecoveryEnvelope"];
        };
      };
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listLocalTerms: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Operación completada. */
      200: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["TermsResponse"];
        };
      };
      400: components["responses"]["BadRequest"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  registerAccount: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["RegisterRequest"];
      };
    };
    responses: {
      /** @description Operación completada. */
      201: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AccountCreated"];
        };
      };
      /** @description Operación completada. */
      202: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AccountCreated"];
        };
      };
      400: components["responses"]["BadRequest"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      405: components["responses"]["MethodNotAllowed"];
      409: components["responses"]["Conflict"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  reissueVerification: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["EmailRequest"];
      };
    };
    responses: {
      /** @description Operación completada. */
      204: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content?: never;
      };
      400: components["responses"]["BadRequest"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  verifyEmail: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["VerificationRequest"];
      };
    };
    responses: {
      /** @description Operación completada. */
      204: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content?: never;
      };
      400: components["responses"]["BadRequest"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  getEmailVerificationLinkStatus: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["ActionLinkStatusRequest"];
      };
    };
    responses: {
      /** @description Estado del enlace de verificación. */
      200: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ActionLinkStatusResponse"];
        };
      };
      400: components["responses"]["BadRequest"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
    };
  };
  requestPasswordRecovery: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["EmailRequest"];
      };
    };
    responses: {
      /** @description Respuesta genérica anti-enumeración. */
      202: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["RecoveryAccepted"];
        };
      };
      400: components["responses"]["BadRequest"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
    };
  };
  consumePasswordRecovery: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["RecoveryConsumeRequest"];
      };
    };
    responses: {
      /** @description Clave actualizada y sesiones revocadas. */
      204: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      400: components["responses"]["BadRequest"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  getPasswordRecoveryLinkStatus: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["ActionLinkStatusRequest"];
      };
    };
    responses: {
      /** @description Estado del enlace de recuperación. */
      200: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ActionLinkStatusResponse"];
        };
      };
      400: components["responses"]["BadRequest"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
    };
  };
  changePassword: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["PasswordChangeRequest"];
      };
    };
    responses: {
      /** @description Clave actualizada; sesiones revocadas. El cliente debe borrar el token y volver a iniciar sesión. */
      204: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  getOwnProfile: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Perfil propio. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["Profile"];
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Perfil aún no creado. */
      404: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  updateOwnProfile: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["ProfileUpdate"];
      };
    };
    responses: {
      /** @description Perfil propio actualizado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["Profile"];
        };
      };
      401: components["responses"]["Unauthorized"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getOwnSyntheticProfilePhoto: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Metadata propia. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json":
            | {
                /** Format: uuid */
                id: string;
                /** @enum {string} */
                fixture_code: "synthetic-png-v1";
                /** @enum {string} */
                mime_type: "image/png";
                sha256: string;
                size_bytes: number;
                /** Format: date-time */
                created_at: string;
              }
            | components["schemas"]["SpaceMediaPhoto"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  generateOwnSyntheticProfilePhoto: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path?: never;
      cookie?: never;
    };
    requestBody?: {
      content: {
        "multipart/form-data": {
          /** Format: binary */
          image: string;
        };
      };
    };
    responses: {
      /** @description Foto propia generada. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": Record<string, never>;
        };
      };
      401: components["responses"]["Unauthorized"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  removeOwnSyntheticProfilePhoto: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Retirada con limpieza privada recuperable. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  getOwnSyntheticProfilePhotoContent: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Imagen privada propia (PNG sintético histórico o avatar WebP). */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "image/png": string;
          "image/webp": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  getOwnSyntheticPayoutAccount: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Cuenta fake propia. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": Record<string, never>;
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  createOrReplaceOwnSyntheticPayoutAccount: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Cuenta simulada creada o reemplazada. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": Record<string, never>;
        };
      };
      401: components["responses"]["Unauthorized"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  revokeOwnSyntheticPayoutAccount: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Cuenta fake revocada; no existe transferencia real. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  listOwnRightsRequests: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Solicitudes propias. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["RightsRequestList"];
        };
      };
      401: components["responses"]["Unauthorized"];
    };
  };
  requestDataRight: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["RightsRequestCreate"];
      };
    };
    responses: {
      /** @description Solicitud recibida para revisión. */
      202: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["RightsRequest"];
        };
      };
      401: components["responses"]["Unauthorized"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  exportOwnIdentityData: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Exportación propia local. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["OwnIdentityExport"];
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Cuenta propia no encontrada. */
      404: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  downloadCompleteOwnDataArchive: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Archivo ZIP propio local. */
      200: {
        headers: {
          /** @description Nombre de descarga del ZIP. */
          "Content-Disposition"?: string;
          /** @description no-store. */
          "Cache-Control"?: string;
          [name: string]: unknown;
        };
        content: {
          "application/zip": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Cuenta propia no encontrada. */
      404: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  reviewLocalSuppressionBlockers: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        request_id: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Evaluación local persistida o reutilizada; ninguna supresión ejecutada. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            /** Format: uuid */
            request_id: string;
            /** @enum {string} */
            outcome: "bloqueada" | "elegible" | "revision_incompleta";
            obligations_detected: (
              | "reserva_activa"
              | "pago_o_devolucion_pendiente"
              | "disputa_abierta"
              | "liquidacion_pendiente"
            )[];
            /** @description fuente_disputas_no_modelada solo puede aparecer al reproducir evaluaciones históricas guardadas antes de V24. */
            pending_checks: (
              | "fuente_disputas_no_modelada"
              | "matriz_retencion_historicos_incompleta"
            )[];
            /** Format: date-time */
            reviewed_at: string;
            reused: boolean;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Se requiere rol administrador. */
      403: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Solicitud de supresión no encontrada. */
      404: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Clave idempotente reutilizada por otro revisor o solicitud ya no revisable. */
      409: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Clave idempotente ausente o inválida. */
      422: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  executeLocalSyntheticSuppression: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        request_id: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Resultado bloqueado, pendiente de archivos o completado; los reintentos reutilizan el resultado persistido. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            /** Format: uuid */
            request_id: string;
            /** @enum {string} */
            status: "bloqueada" | "limpieza_pendiente" | "completada";
            /** @enum {string} */
            outcome:
              | "bloqueada"
              | "limpieza_pendiente"
              | "baja_local_con_minimizacion_y_retencion_residual";
            /** @enum {string} */
            decision_code:
              | "supresion_bloqueada_por_obligaciones"
              | "baja_elegible_privacidad_local_v1";
            obligations_detected: (
              | "reserva_activa"
              | "pago_o_devolucion_pendiente"
              | "disputa_abierta"
              | "liquidacion_pendiente"
            )[];
            removed: string[];
            retained: string[];
            pending_files: number;
            /** Format: date-time */
            started_at: string;
            /** Format: date-time */
            completed_at?: string;
            reused: boolean;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Se requiere rol administrador. */
      403: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Solicitud de supresión no encontrada. */
      404: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Clave idempotente incompatible o solicitud no ejecutable. */
      409: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Clave idempotente ausente o inválida. */
      422: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description La ejecución se completó, pero el registro local externo requiere resincronización; repetir la misma clave no duplica la baja. */
      503: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  purgeExpiredLocalPrivacyData: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Conteos estructurados del lote procesado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            scanned: number;
            purged: number;
            deferred: number;
            outbox_events_purged?: number;
            terms_purged?: number;
            rights_purged?: number;
            quotes_purged?: number;
            verifications_purged?: number;
            evidence_files_queued?: number;
            evidence_files_removed?: number;
            evidence_files_deferred?: number;
            contracts_purged?: number;
            publication_history_purged?: number;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Se requiere rol administrador. */
      403: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Servicio local de privacidad no disponible. */
      503: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  listTerminalLocalCredentialNotices: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Eventos terminales recuperables. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            items: {
              /** Format: uuid */
              event_id: string;
              error_code: string;
              total_attempts: number;
              cycle_number: number;
              cycle_attempts: number;
              /** Format: date-time */
              failed_at: string;
              /** Format: date-time */
              remove_at: string;
            }[];
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Se requiere rol administrador. */
      403: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  reopenTerminalLocalCredentialNotice: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        event_id: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          reason_code: "smtp_restaurado" | "reintento_operativo";
        };
      };
    };
    responses: {
      /** @description Ciclo abierto o resultado idempotente reutilizado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            /** Format: uuid */
            event_id: string;
            /** @enum {string} */
            state: "pendiente";
            /** @enum {string} */
            reason_code: "smtp_restaurado" | "reintento_operativo";
            idempotency_key: string;
            cycle_number: number;
            total_attempts: number;
            cycle_attempts: number;
            /** Format: date-time */
            started_at: string;
            reused: boolean;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Se requiere rol administrador. */
      403: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Evento inexistente o no recuperable. */
      404: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Evento entregado, cancelado, destinatario inactivo o clave incompatible. */
      409: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      /** @description Motivo o clave idempotente inválidos. */
      422: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  listPendingLocalSuppressionRequests: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Solicitudes de supresión en revisión. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            items: {
              /** Format: uuid */
              request_id: string;
              /** Format: date-time */
              created_at: string;
            }[];
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      /** @description Se requiere rol administrador. */
      403: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  listOwnVerifications: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Casos propios. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationList"];
        };
      };
      401: components["responses"]["Unauthorized"];
    };
  };
  startSyntheticVerification: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["VerificationCreate"];
      };
    };
    responses: {
      /** @description Caso sintético en revisión. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationCase"];
        };
      };
      401: components["responses"]["Unauthorized"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listOwnSyntheticEligibility: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Estado local KYC/KYB por separado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationEligibilityList"];
        };
      };
      401: components["responses"]["Unauthorized"];
    };
  };
  getOwnVerification: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Caso propio. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationCase"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  listOwnVerificationHistory: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Historial ordenado del caso propio. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationHistoryList"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  retryRejectedVerification: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        id: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["VerificationRetry"];
      };
    };
    responses: {
      /** @description Reintento nuevo y sintético. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationCase"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listOwnSyntheticEvidence: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Evidencias sintéticas propias. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SyntheticEvidenceList"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  uploadSyntheticEvidence: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["SyntheticEvidenceCreate"];
      };
    };
    responses: {
      /** @description Metadatos del fixture persistido. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SyntheticEvidence"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getOwnSyntheticEvidenceContent: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
        evidenceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description PNG sintético. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "image/png": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  deleteOwnSyntheticEvidence: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
        evidenceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Fixture eliminado. */
      204: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  reviewSyntheticEvidenceContent: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
        evidenceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description PNG sintético. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "image/png": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  deleteReviewedSyntheticEvidence: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
        evidenceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Fixture eliminado. */
      204: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  listEvidenceForReview: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Evidencias sintéticas del caso. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SyntheticEvidenceList"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  listPendingVerifications: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Casos en revisión. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationList"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
    };
  };
  listRejectedVerifications: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Casos rechazados sintéticos. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationList"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
    };
  };
  reviewSyntheticVerification: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        id: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["VerificationReview"];
      };
    };
    responses: {
      /** @description Caso sintético resuelto. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationCase"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  revokeSyntheticEligibility: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        id: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          reason_code:
            "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada";
        };
      };
    };
    responses: {
      /** @description Elegibilidad revocada o resultado idempotente persistido. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["VerificationCase"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  login: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["LoginRequest"];
      };
    };
    responses: {
      /** @description Operación completada. */
      200: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LoginResponse"];
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  currentSession: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Operación completada. */
      200: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SessionResponse"];
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  listSpaceCategories: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Categorías disponibles. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceCategoryList"];
        };
      };
      401: components["responses"]["Unauthorized"];
    };
  };
  getSpaceAttributeProfile: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        categoryCode: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Perfil versionado de características. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceAttributeProfile"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  getVersionedSpaceAttributeProfile: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        categoryCode: string;
        version: number;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Perfil versionado solicitado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceAttributeProfile"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  listOwnSpaces: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Espacios propios. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceDraftList"];
        };
      };
      401: components["responses"]["Unauthorized"];
    };
  };
  createOwnSpaceDraft: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["SpaceDraftInput"];
      };
    };
    responses: {
      /** @description Borrador creado. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceDraft"];
        };
      };
      401: components["responses"]["Unauthorized"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getOwnSpaceDraft: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Espacio propio. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceDraft"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  updateOwnSpaceDraft: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["SpaceDraftInput"];
      };
    };
    responses: {
      /** @description Borrador actualizado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceDraft"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  deleteOwnPublication: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Publicación retirada o recibo idempotente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceDeletionReceipt"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  setOwnSpacePublicationState: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["SpacePublicationInput"];
      };
    };
    responses: {
      /** @description Estado de publicación actualizado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceDraft"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  updateOwnPublishedSpaceContent: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["PublishedSpaceContentInput"];
      };
    };
    responses: {
      /** @description Datos vigentes de la publicación propia. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceDraft"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listOwnSyntheticSpaceGallery: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Galería sintética propia y límite vigente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            items: (
              | components["schemas"]["SyntheticSpaceGalleryPhoto"]
              | components["schemas"]["SpaceMediaPhoto"]
            )[];
            /** @constant */
            max_photos: 10;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  createSyntheticOwnSpaceGalleryPhoto: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: {
      content: {
        "multipart/form-data": {
          /** Format: binary */
          image: string;
        };
      };
    };
    responses: {
      /** @description Resultado reutilizado de un reintento idempotente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json":
            | components["schemas"]["SyntheticSpaceGalleryPhotoEnvelope"]
            | components["schemas"]["SpaceMediaPhotoEnvelope"];
        };
      };
      /** @description Imagen sintética o foto real normalizada a WebP. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json":
            | components["schemas"]["SyntheticSpaceGalleryPhotoEnvelope"]
            | components["schemas"]["SpaceMediaPhotoEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
      /** @description Storage privado temporalmente no disponible. */
      503: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["Error"];
        };
      };
    };
  };
  selectOwnSyntheticSpaceCover: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          photo_id: string;
        };
      };
    };
    responses: {
      /** @description Portada seleccionada. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SyntheticSpaceCover"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getOwnSyntheticSpaceGalleryPhoto: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Metadatos de una imagen activa propia. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json":
            | components["schemas"]["SyntheticSpaceGalleryPhoto"]
            | components["schemas"]["SpaceMediaPhoto"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  updateOwnSpaceMediaPhoto: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          alt_text?: string;
          visible?: boolean;
          position?: number;
        };
      };
    };
    responses: {
      /** @description Metadata de imagen real actualizada. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SpaceMediaPhoto"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  removeOwnSyntheticSpaceGalleryPhoto: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Resultado idempotente de la retirada. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SyntheticSpaceGalleryRemoval"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  setOwnSyntheticSpaceGalleryPhotoPublication: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          public_in_catalog: boolean;
        };
      };
    };
    responses: {
      /** @description Estado de publicación de la imagen propia. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["SyntheticSpaceGalleryPhoto"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  getOwnSyntheticSpaceGalleryPhotoContent: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Contenido PNG sintético. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "image/png": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      /** @description Storage privado temporalmente no disponible. */
      503: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["Error"];
        };
      };
    };
  };
  getOwnSpaceMediaVariant: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        photoId: string;
        variant: "display" | "thumbnail";
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Variante WebP privada */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "image/webp": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      /** @description Storage privado temporalmente no disponible. */
      503: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
    };
  };
  queryOwnDraftAvailability: {
    parameters: {
      query: {
        from: components["schemas"]["UtcTimestamp"];
        to: components["schemas"]["UtcTimestamp"];
      };
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Resultado de disponibilidad del espacio propio. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AvailabilityResult"];
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  configureOwnDraftAvailability: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["AvailabilityConfig"];
      };
    };
    responses: {
      /** @description Zona horaria configurada. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AvailabilityConfigResponse"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getOwnSpaceTariff: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Tarifa vigente */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["Rate"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  updateOwnSpaceTariff: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["RateInput"];
      };
    };
    responses: {
      /** @description Nueva versión */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["Rate"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listOwnSpaceTariffVersions: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Historial de tarifas */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["RateList"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  createPrivateOwnDraftPriceSimulation: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["PriceSimulationInput"];
      };
    };
    responses: {
      /** @description Simulación privada guardada */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["PriceSimulation"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getOwnPrivatePriceSimulation: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        simulationId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Snapshot de simulación */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["PriceSimulation"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  listOwnDraftManualBlocks: {
    parameters: {
      query: {
        from: components["schemas"]["UtcTimestamp"];
        to: components["schemas"]["UtcTimestamp"];
      };
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Calendario manual del borrador propio. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ManualBlockList"];
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  createOwnDraftManualBlock: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["ManualBlockInput"];
      };
    };
    responses: {
      /** @description Bloqueo manual creado. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["ManualBlock"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getOwnDraftTimeZone: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Zona horaria del calendario. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AvailabilityConfigResponse"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  deleteOwnDraftManualBlock: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
        blockId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Bloqueo desactivado. */
      204: {
        headers: {
          [name: string]: unknown;
        };
        content?: never;
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  getAuthorizedLocalBookingFixture: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Fixture privado con el banner de ensayo. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingFixtureEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  searchAuthorizedLocalSyntheticCatalog: {
    parameters: {
      query?: {
        /** @description Texto literal insensible a mayúsculas en título */
        q?: string;
        /** @description Superficie mínima inclusiva en metros cuadrados. */
        min_area_m2?: number;
        category_code?:
          | "oficina"
          | "sala_multiproposito"
          | "bodega"
          | "estacionamiento"
          | "local_flexible"
          | "stand"
          | "quincho"
          | "parcela_eventos";
        start_at?: string;
        end_at?: string;
        /** @description Mínimo inclusivo del total estimado CLP para el intervalo. */
        min_total_clp?: number;
        /** @description Máximo inclusivo del total estimado CLP para el intervalo. */
        max_total_clp?: number;
        /** @description Versión de perfil seleccionada; requerida junto con attributes y category_code. */
        profile_version?: number;
        /** @description Objeto JSON URL-encoded de atributos tipados exactos; enum_list usa contención de elementos. */
        attributes?: string;
        /** @description Centro sintético; requiere longitude y radius_km. */
        latitude?: number;
        /** @description Centro sintético; requiere latitude y radius_km. */
        longitude?: number;
        /** @description Radio inclusivo permitido. Requiere latitude y longitude. */
        radius_km?: 1 | 3 | 5 | 10 | 25;
        /** @description Cantidad de resultados, predeterminado 5. */
        page_size?: number;
        /** @description Continuación opaca vinculada a cuenta, filtros, tamaño y orden de esta búsqueda. */
        cursor?: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Resultados de catálogo local con banner de ensayo. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalCatalogEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getAuthorizedLocalSyntheticCatalogDetail: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Detalle del fixture con aviso local. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalCatalogDetailEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  getAuthorizedLocalFixtureAvailabilityOptions: {
    parameters: {
      query: {
        /** @description Fecha local única consultada en la zona IANA del espacio; no descarga el horizonte entero. */
        date: string;
        /** @description Unidades compatibles con la modalidad vigente del espacio; hora 1/2/4 transcurridas, día 1/2/3 calendario, mes exactamente 1. */
        duration: 1 | 2 | 3 | 4;
      };
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Modalidad, zona IANA e intervalos UTC libres para la fecha elegida. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalAvailabilityOptionsEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getLocalBusinessClock: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Instante lógico UTC confirmado por Go. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBusinessClockEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
    };
  };
  getOwnSyntheticHourlyWeeklyHours: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Configuración semanal del espacio propio. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalWeeklyHoursEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  replaceOwnSyntheticHourlyWeeklyHours: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          enabled: boolean;
          days: components["schemas"]["LocalWeeklyDay"][];
        };
      };
    };
    responses: {
      /** @description Configuración semanal persistida. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalWeeklyHoursEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  quoteAuthorizedLocalBooking: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          space_id: string;
          /** Format: date-time */
          start_at: string;
          /** Format: date-time */
          end_at: string;
        };
      };
    };
    responses: {
      /** @description Cotización snapshot de ensayo local. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingQuoteEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listOwnLocalTrialReservations: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Historial y aviso ENSAYO LOCAL — SIN COBRO REAL. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingListEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
    };
  };
  requestLocalTrialReservation: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          quote_id: string;
        };
      };
    };
    responses: {
      /** @description Retención en ocupacion y reserva pendiente de pago dentro de una transacción. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingReservationEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  getOwnLocalTrialReservation: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Estado, snapshot e historial autorizado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingDetailEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  simulateLocalTrialPayment: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          outcome: "exito" | "rechazo" | "sin_respuesta";
        };
      };
    };
    responses: {
      /** @description Resultado de ensayo con banner no-cobro. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingReservationEnvelope"];
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      /** @description Timeout simulado; la reserva sigue pendiente. */
      504: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingTimeoutEnvelope"];
        };
      };
    };
  };
  getOwnLocalSettlement: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Estado e importes separados de la liquidación fake. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalSettlementEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  requestLocalFakeSettlement: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          outcome: "exito" | "rechazo" | "sin_respuesta";
        };
      };
    };
    responses: {
      /** @description Resultado fake persistido o incierto para la misma operación. */
      202: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalSettlementEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  reconcileLocalFakeSettlement: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          outcome: "exito" | "rechazo";
        };
      };
    };
    responses: {
      /** @description Resultado persistido */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalSettlementEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  getOwnLocalSettlementReceipt: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Comprobante versionado de ensayo. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalSettlementReceiptEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  getOwnSyntheticReservationGuarantee: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Estado financiero local y operaciones durables. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalGuaranteeEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  executeLocalSyntheticGuaranteeOperation: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          kind: "autorizacion" | "captura" | "liberacion";
          amount_clp: number;
          /**
           * @description Rechazo confirmado solo se admite en autorización.
           * @enum {string}
           */
          outcome: "exito" | "rechazo" | "sin_respuesta";
        };
      };
    };
    responses: {
      /** @description Operación fake persistida. */
      202: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalGuaranteeOperationEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  reconcileLocalGuaranteeFakeResult: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          kind: "autorizacion" | "captura" | "liberacion";
          /**
           * @description Rechazo solo se admite para autorización.
           * @enum {string}
           */
          outcome: "exito" | "rechazo";
        };
      };
    };
    responses: {
      /** @description Resultado fake reconciliado o reutilizado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalGuaranteeOperationEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  recordAdminLocalFinancialDecision: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: uuid */
          claim_id: string;
          /** @enum {string} */
          outcome: "acogido" | "rechazado";
          /** @description El Backend limita el importe a la autorización fake confirmada. */
          deduction_clp: number;
          /** @enum {string} */
          reason_code:
            "dano_acreditado" | "faltante_acreditado" | "sin_deduccion";
          /** Format: uuid */
          evidence_id?: string;
        };
      };
    };
    responses: {
      /** @description Decisión persistida e idempotente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalFinancialDecisionEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  receiveAuthenticatedLocalPaymentEvent: {
    parameters: {
      query?: never;
      header: {
        "X-Local-Payment-Signature": string;
      };
      path?: never;
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          event_id: string;
          /** Format: uuid */
          operation_id: string;
          /** @enum {string} */
          outcome: "exito_simulado" | "rechazo_simulado";
        };
      };
    };
    responses: {
      /** @description Evento autenticado y persistido; su aplicación durable queda pendiente. */
      202: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            data: {
              /** @constant */
              accepted: true;
              reused: boolean;
            };
            /** @constant */
            safety_notice: "ENSAYO LOCAL — SIN COBRO REAL";
          };
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      409: components["responses"]["Conflict"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  decideLocalTrialReservation: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          decision: "aprobar" | "rechazar";
          /** @description Obligatorio únicamente para rechazar; se conserva en el historial. */
          reason?: string;
        } & unknown;
      };
    };
    responses: {
      /** @description Aprobación o rechazo del participante anfitrión. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingReservationEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  previewLocalTrialReservationCancellation: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Política y elegibilidad local actual de cancelación. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingCancellationPreviewEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  cancelLocalTrialReservation: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @description Motivo opcional; no modifica el cálculo. */
          reason?: string;
        };
      };
    };
    responses: {
      /** @description Cancelación atómica con estado de devolución separado y aviso local no durable. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingCancellationResultEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  simulateLocalTrialRefund: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          outcome: "exito" | "fallo" | "sin_respuesta";
        };
      };
    };
    responses: {
      /** @description Resultado local fake y estado separado de devolución. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingRefundResultEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
      /** @description Timeout simulado; la obligación permanece pendiente. */
      504: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalBookingRefundTimeoutEnvelope"];
        };
      };
    };
  };
  listOwnLocalReservationMessages: {
    parameters: {
      query?: {
        before?: number | string;
        limit?: number;
      };
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Página de texto sintético de la conversación. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReservationMessagesPageEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  sendOwnLocalReservationMessage: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          body: string;
        };
      };
    };
    responses: {
      /** @description Mensaje persistido; replay idéntico devuelve el mismo mensaje. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReservationMessageEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      /** @description Estado terminal */
      409: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["Error"];
        };
      };
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  advanceOwnLocalReservationMessageReadCursor: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** Format: int64 */
          through_sequence: number | string;
        };
      };
    };
    responses: {
      /** @description Cursor propio persistido y monotónico. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalReservationReadCursorEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listOwnReservationTrialDisputes: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservation_id: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Incidencias propias. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            items: components["schemas"]["LocalTrialDispute"][];
            /** @constant */
            scope: "local_synthetic_disputes";
          };
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      500: components["responses"]["InternalServerError"];
    };
  };
  openOwnHostedTrialDispute: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservation_id: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @constant */
          reason_code: "ensayo_privacidad";
        };
      };
    };
    responses: {
      /** @description Reintento idempotente; registro existente con reused=true. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalTrialDispute"];
        };
      };
      /** @description Incidencia abierta con historial inicial. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalTrialDispute"];
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      500: components["responses"]["InternalServerError"];
    };
  };
  getOwnTrialDisputeHistory: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        dispute_id: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Historial persistente por secuencia para cualquiera de las partes. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            items: components["schemas"]["LocalTrialDisputeTransition"][];
          };
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      500: components["responses"]["InternalServerError"];
    };
  };
  listOpenLocalTrialDisputes: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Incidencias abiertas. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            items: components["schemas"]["LocalTrialDispute"][];
          };
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      500: components["responses"]["InternalServerError"];
    };
  };
  closeLocalTrialDispute: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        dispute_id: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": {
          /** @enum {string} */
          reason_code: "ensayo_finalizado" | "registro_erroneo" | "duplicada";
        };
      };
    };
    responses: {
      /** @description Incidencia cerrada; actor, fecha y motivo se conservan en historial. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalTrialDispute"];
        };
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      500: components["responses"]["InternalServerError"];
    };
  };
  logout: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Operación completada. */
      204: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content?: never;
      };
      400: components["responses"]["BadRequest"];
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      405: components["responses"]["MethodNotAllowed"];
      415: components["responses"]["UnsupportedMediaType"];
      422: components["responses"]["UnprocessableEntity"];
      429: components["responses"]["TooManyRequests"];
      500: components["responses"]["InternalServerError"];
      503: components["responses"]["ServiceUnavailable"];
    };
  };
  createOrGetSyntheticReservationContract: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Contrato local existente o recién generado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            data: components["schemas"]["LocalSyntheticContract"];
            notice: string;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  getOwnSyntheticContract: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        contractId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Estado, snapshot e historial para participantes. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            data?: components["schemas"]["LocalSyntheticContract"];
            notice?: string;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  signSyntheticContractAsParticipant: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        contractId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Transición de firma persistida. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            data?: components["schemas"]["LocalSyntheticContract"];
            notice?: string;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  rejectSyntheticContractVersion: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        contractId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Rechazo registrado en el historial. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": {
            data?: components["schemas"]["LocalSyntheticContract"];
            notice?: string;
          };
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  downloadSignedSyntheticContract: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        contractId: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Artefacto PDF marcado como ensayo sin validez jurídica. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/pdf": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
    };
  };
  recordSyntheticCheckIn: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["LocalRentalOperationInput"];
      };
    };
    responses: {
      /** @description Reintento idempotente; devuelve el registro existente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationEnvelope"];
        };
      };
      /** @description Check-in registrado con evidencia sintética. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  recordSyntheticCheckOut: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["LocalRentalOperationInput"];
      };
    };
    responses: {
      /** @description Reintento idempotente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationEnvelope"];
        };
      };
      /** @description Check-out registrado; plazo derivado del instante persistido. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  recordSyntheticHostReception: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["LocalRentalOperationInput"];
      };
    };
    responses: {
      /** @description Reintento idempotente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationEnvelope"];
        };
      };
      /** @description Recepción/observación registrada con evidencia sintética. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listSyntheticRentalOperations: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Historial propio de la reserva. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationListEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  downloadSyntheticRentalOperationEvidence: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
        evidenceID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description PNG generado por el Backend. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "image/png": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  createSyntheticDamageClaimDefenseEvidence: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": Record<string, never>;
      };
    };
    responses: {
      /** @description Reintento idempotente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationEnvelope"];
        };
      };
      /** @description Evidencia PNG sintética generada por Backend. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalRentalOperationEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getOwnSyntheticDamageClaim: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Reclamo y descargo existentes. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
    };
  };
  openSyntheticHostDamageClaim: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["LocalDamageClaimInput"];
      };
    };
    responses: {
      /** @description Reintento idempotente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimEnvelope"];
        };
      };
      /** @description Reclamo sintético abierto y transición persistida. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  submitSyntheticDamageClaimDefense: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["LocalDamageClaimInput"];
      };
    };
    responses: {
      /** @description Reintento idempotente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimDefenseEnvelope"];
        };
      };
      /** @description Descargo sintético registrado. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimDefenseEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listAdminLocalAuditEvents: {
    parameters: {
      query: {
        from: string;
        /** @description Exclusivo; mismo instante normalizado a UTC. */
        until: string;
        actor_id?: string;
        resource_type?: string;
        resource_id?: string;
        action?: string;
        result?: "exito" | "rechazo";
        page_size?: number;
        cursor?: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Página minimizada sin conteo total; registra una auditoría por petición. */
      200: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalAuditPageEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  exportAdminLocalAuditEvents: {
    parameters: {
      query: {
        from: string;
        /** @description Exclusivo; máximo 31 días. */
        until: string;
        actor_id?: string;
        resource_type?: string;
        resource_id?: string;
        action?: string;
        result?: "exito" | "rechazo";
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Archivo JSON minimizado. */
      200: {
        headers: {
          "X-Request-ID": components["headers"]["X-Request-ID"];
          "Content-Disposition"?: string;
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalAuditExport"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      /** @description Parámetros inválidos o más de 10.000 eventos; no se devuelve exportación parcial. */
      422: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["Error"];
        };
      };
    };
  };
  getAdminLocalAccountByID: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Proyección mínima de la cuenta. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalAccountEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listAdminLocalPublishedSpaces: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Publicaciones activas ordenadas por actualización descendente e ID. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalPublishedSpacesEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
    };
  };
  getAdminLocalPublishedSpace: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        spaceID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Proyección mínima de la publicación activa. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalPublishedSpaceEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  blockLocalAccount: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["AdminLocalBlockInput"];
      };
    };
    responses: {
      /** @description Bloqueo aplicado atómicamente. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalBlockEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  unblockLocalAccount: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        accountID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["AdminLocalUnblockInput"];
      };
    };
    responses: {
      /** @description Cuenta habilitada para nueva autenticación. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalBlockEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getAdminLocalReservationsReport: {
    parameters: {
      query: {
        /** @description Inclusivo; RFC3339 con offset compatible con timezone; el instante de salida se normaliza a UTC. */
        from: string;
        /** @description Exclusivo; RFC3339 con offset compatible con timezone; máximo 31 días calendario; el instante de salida se normaliza a UTC. */
        until: string;
        /** @description Zona IANA que valida los offsets y el límite calendario. */
        timezone: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Conteos por estado vigente con aviso de ensayo. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalReservationsReportEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getAdminLocalFinanceReport: {
    parameters: {
      query: {
        /** @description Inclusivo; RFC3339 con offset compatible con timezone; el instante de salida se normaliza a UTC. */
        from: string;
        /** @description Exclusivo; RFC3339 con offset compatible con timezone; máximo 31 días calendario; el instante de salida se normaliza a UTC. */
        until: string;
        timezone: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Movimientos confirmados separados de obligaciones al instante de generación. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalFinanceReportEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getAdminLocalDisputesReport: {
    parameters: {
      query: {
        /** @description Inclusivo; RFC3339 con offset compatible con timezone; salida UTC. */
        from: string;
        /** @description Exclusivo; máximo 31 días calendario; salida UTC. */
        until: string;
        timezone: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Conteos agregados sin datos personales. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalDisputesReportEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  listAdminLocalReservations: {
    parameters: {
      query?: {
        reservation_id?: string;
        state?: string;
        created_from?: string;
        /** @description Hasta exclusivo */
        created_to?: string;
        page_size?: number;
        cursor?: string;
      };
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Página sin conteo total. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalReservationPageEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
  getAdminLocalReservation: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        reservationID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Estado */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["AdminLocalReservationDetailEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  listOpenLocalSyntheticDamageClaims: {
    parameters: {
      query?: never;
      header?: never;
      path?: never;
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Reclamos abiertos. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimListEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
    };
  };
  getAdminLocalSyntheticDamageClaim: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        claimID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description Detalle de reclamo para revisión. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  getAdminLocalDamageClaimEvidence: {
    parameters: {
      query?: never;
      header?: never;
      path: {
        claimID: string;
        evidenceID: string;
      };
      cookie?: never;
    };
    requestBody?: never;
    responses: {
      /** @description PNG sintético privado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "image/png": string;
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
    };
  };
  resolveLocalSyntheticDamageClaim: {
    parameters: {
      query?: never;
      header: {
        "Idempotency-Key": string;
      };
      path: {
        claimID: string;
      };
      cookie?: never;
    };
    requestBody: {
      content: {
        "application/json": components["schemas"]["LocalDamageClaimResolutionInput"];
      };
    };
    responses: {
      /** @description Reintento idempotente reutilizado. */
      200: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimEnvelope"];
        };
      };
      /** @description Reclamo resuelto; reserva y registros financieros no se modifican. */
      201: {
        headers: {
          [name: string]: unknown;
        };
        content: {
          "application/json": components["schemas"]["LocalDamageClaimEnvelope"];
        };
      };
      401: components["responses"]["Unauthorized"];
      403: components["responses"]["Forbidden"];
      404: components["responses"]["NotFound"];
      409: components["responses"]["Conflict"];
      422: components["responses"]["UnprocessableEntity"];
    };
  };
}
