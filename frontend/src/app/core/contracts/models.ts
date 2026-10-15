/** Frontend projections of the Go contract. Pure TypeScript; no transport or Angular dependency.
 * Wire names are kept for structural mapping at the adapter boundary. Money is decoded exactly by the adapters. */
export type AccountCreated = {
  account_id: string;
  status: 'email_pending' | 'verification_pending';
  use_preference: 'ofrecer' | 'arrendar';
  message?: string;
};

export type AvailabilityConfigResponse = {
  space_id: string;
  time_zone: string;
};

export type AvailabilityResult = {
  space_id: string;
  time_zone: string;
  start_at: string;
  end_at: string;
  available: boolean;
};

export type LocalBookingCancellationPreview = {
  reservation_id: string;
  policy_version: 'local_flexible_v1';
  eligible: boolean;
  /**
   * Pago vence para pendiente_de_pago; inicio reservado para pagada/aprobada_host.
   */
  deadline: string;
  /**
   * Subtotal efectivamente confirmado por el fake; sin comisiones ni garantías.
   */
  amount_clp: number;
  currency: 'CLP';
  refund_label: 'Devolución simulada — sin movimiento de dinero';
  reason_code?:
    | 'sin_devolucion'
    | 'devolucion_simulada_completa'
    | 'inicio_alcanzado'
    | 'pago_fake_no_confirmado'
    | 'estado_no_cancelable';
};

export type LocalBookingDetail = LocalBookingReservation & {
  history: Array<LocalBookingTransition>;
};

export type LocalBookingReservation = {
  id: string;
  quote_id: string;
  space_id: string;
  host_id: string;
  renter_id: string;
  state:
    | 'pendiente_de_pago'
    | 'pagada'
    | 'aprobada_host'
    | 'firma_parcial'
    | 'lista_para_checkin'
    | 'en_curso'
    | 'finalizada'
    | 'cerrada'
    | 'en_disputa'
    | 'cancelada_por_firma'
    | 'cancelada_por_pago'
    | 'rechazada_arrendador'
    | 'vencida_pago'
    | 'vencida_host'
    | 'cancelada_arrendatario';
  rate_unit: 'hora' | 'dia' | 'mes';
  unit_price_clp: number;
  currency: 'CLP';
  units: number;
  subtotal_clp: number;
  start_at: string;
  end_at: string;
  time_zone: string;
  conditions: string;
  /**
   * Snapshot local tomado desde la cotización.
   */
  cancellation_policy_version: 'local_flexible_v1';
  guarantee_policy_version?: 'garantia_local_fija_v1' | null;
  guarantee_currency?: 'CLP' | null;
  /**
   * Ausente/null únicamente para reservas históricas previas a V40.
   */
  guarantee_expected_clp?: 50000 | null;
  /**
   * Ausente/null únicamente para reservas históricas previas a V44.
   */
  settlement_policy_version?: 'liquidacion_local_0pct_v1' | null;
  /**
   * Snapshot local; no asignado retroactivamente a reservas históricas.
   */
  settlement_commission_bps?: 0 | null;
  refund_id?: string;
  refund_operation_id?: string;
  refund_amount_clp?: number;
  refund_state?: 'pendiente' | 'completada';
  refund_last_result?: 'exito_simulado' | 'fallo_simulado' | 'sin_respuesta_simulada';
  refund_updated_at?: string;
  pay_expires_at: string;
  host_expires_at?: string | null;
  created_at: string;
  updated_at: string;
  unread_count: number;
};

export type LocalBookingTransition = {
  sequence: number;
  from?: string;
  to: string;
  actor_id?: string;
  reason: string;
  at: string;
};

export type LocalRentalOperation = {
  id: string;
  reservation_id: string;
  kind: 'checkin' | 'checkout' | 'recepcion' | 'descargo';
  actor_id: string;
  occurred_at: string;
  time_zone: string;
  synthetic_location: {
    source: 'synthetic-fixture-v1';
    location_code: 'santiago-demo-center-v1';
    latitude: -33.456;
    longitude: -70.6693;
  };
  comments?: string;
  observations?: string;
  result: 'registrada' | 'recepcion_conforme' | 'recepcion_con_observaciones';
  /**
   * Check-out registrado más 24 horas; basado en instante persistido.
   */
  claim_deadline_at?: string;
  sequence: number;
  reused?: boolean;
  evidence: Array<{
    id: string;
    content_url?: string;
    fixture_code: 'synthetic-png-v1';
    mime_type: 'image/png';
    sha256: string;
    size_bytes: number;
    created_at: string;
  }>;
};

export type LocalRentalOperationInput = {
  /**
   * Comentario sintético opcional de check-in o check-out.
   */
  comments?: string;
  /**
   * Observación opcional de recepción; no equivale a reclamo formal.
   */
  observations?: string;
};

export type LocalRentalOperationListEnvelope = {
  data: {
    items: Array<LocalRentalOperation>;
  };
  notice: 'ENSAYO LOCAL — EVIDENCIA SINTÉTICA; SIN VALIDEZ PROBATORIA REAL';
};

export type LocalReservationMessage = {
  id: string;
  reservation_id: string;
  author_id: string;
  sequence: number;
  body: string;
  created_at: string;
};

export type LocalReservationMessagesPageEnvelope = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: {
    items: Array<LocalReservationMessage>;
    older_cursor: number | null;
  };
};

export type LocalReservationReadCursorEnvelope = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: {
    read_through_sequence: number;
  };
};

export type LocalReview = {
  id: string;
  target_type?: 'espacio' | 'arrendatario';
  rating: number;
  state?: 'publicada' | 'reportada';
  comment?: string;
  created_at: string;
  reused?: boolean;
};

export type LocalReviewEnvelope = {
  data: LocalReview;
  notice: string;
};

export type LocalReviewListEnvelope = {
  data: {
    items: Array<LocalReview>;
  };
  notice: string;
};

export type LocalReviewReport = {
  id: string;
  review_id: string;
  space_id: string;
  review_comment?: string;
  reason_code: 'insultos_acoso' | 'datos_personales' | 'spam' | 'ajeno_experiencia';
  state: 'pendiente' | 'desestimada' | 'oculta';
  created_at: string;
  resolved_at?: string;
  resolution_reason_code?: string;
};

export type LocalReviewReportEnvelope = {
  data: LocalReviewReport;
  notice: string;
};

export type LocalSpaceReviewsEnvelope = {
  data: {
    items: Array<LocalReview>;
    average: number;
    count: number;
  };
  notice: string;
};

export type LocalSyntheticContract = {
  id: string;
  reservation_id: string;
  version: number;
  state: 'generado' | 'firma_parcial' | 'firmado' | 'anulado';
  /**
   * Snapshot JSON serializado; solo hechos de la reserva y aviso sintético.
   */
  snapshot: string;
  sha256: string;
  signatures: Array<{
    signer_id: string;
    role: 'anfitrion' | 'arrendatario';
    state: 'pendiente' | 'firmada' | 'rechazada';
    reason?: string;
    updated_at: string;
  }>;
  history: Array<{
    sequence: number;
    actor_id?: string;
    action: string;
    reason?: string;
    at: string;
  }>;
};

export type LocalWeeklyDay = {
  /**
   * Día ISO de la semana; lunes=1, domingo=7.
   */
  weekday: number;
  periods: Array<LocalWeeklyPeriod>;
};

export type LocalWeeklyHours = {
  space_id: string;
  enabled: boolean;
  time_zone: string;
  days: [
    LocalWeeklyDay,
    LocalWeeklyDay,
    LocalWeeklyDay,
    LocalWeeklyDay,
    LocalWeeklyDay,
    LocalWeeklyDay,
    LocalWeeklyDay,
  ];
};

export type LocalWeeklyHoursEnvelope = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: LocalWeeklyHours;
};

export type LocalWeeklyPeriod = {
  open: string;
  /**
   * 24:00 representa el límite final exclusivo del día y solo puede usarse al cerrar un tramo.
   */
  close: string;
};

export type LoginResponse = {
  account_id: string;
  roles: Array<'arrendatario' | 'arrendador' | 'administrador' | 'administrador_global'>;
  capabilities: AdminCapability[];
  /**
   * Bearer opaco; solo en esta respuesta, nunca URL/log. Mantener en memoria en mock.
   */
  readonly access_token: string;
  token_type: 'Bearer';
  expires_at: UtcTimestamp;
  /**
   * true si el Backend permite únicamente acceso restringido a reservas existentes.
   */
  restricted_mode: boolean;
};

export type ManualBlock = {
  id: string;
  space_id: string;
  start_at: string;
  end_at: string;
  time_zone: string;
  reason: string;
  created_at: string;
};

export type ManualBlockList = {
  space_id: string;
  time_zone: string;
  items: Array<{
    id: string;
    space_id: string;
    start_at: string;
    end_at: string;
    time_zone: string;
    reason: string;
    created_at: string;
  }>;
};

export type OwnIdentityExport = {
  scope: 'identidad_local_v1';
  account: {
    email: string;
    state: string;
    use_preference?: 'ofrecer' | 'arrendar' | null;
    created_at: string;
  };
  profile?: {
    display_name: string;
    phone?: string | null;
    updated_at: string;
  };
  roles: Array<'arrendatario' | 'arrendador' | 'administrador' | 'administrador_global'>;
  terms_acceptances: Array<{
    code: string;
    type: 'terminos' | 'privacidad' | 'politica_arrendador';
    accepted_at: string;
  }>;
  rights_requests: Array<RightsRequest>;
};

export type PriceSimulation = {
  id: string;
  space_id: string;
  rate_version: number;
  rate_unit: 'hora' | 'dia' | 'mes';
  base_price: number;
  billed_units: number;
  currency: 'CLP';
  base_subtotal: number;
  start_at: string;
  end_at: string;
  time_zone: string;
  private: true;
  created_at: string;
};

export type PriceSimulationInput = {
  start_at: string;
  end_at: string;
};

export type Profile = {
  display_name: string;
  phone?: string | null;
  updated_at: string;
};

export type ProfileUpdate = {
  display_name: string;
  phone?: string;
};

export type PublishedSpaceContentInput = {
  title?: string;
  description?: string;
  capacity?: number;
  usage_rules?: string;
  base_price_clp?: number;
  region_code?: string;
  comuna_code?: string;
};

export type Rate = {
  space_id: string;
  version: number;
  rate_unit: 'hora' | 'dia' | 'mes';
  base_price: number;
  currency: 'CLP';
  created_at: string;
};

export type RateInput = {
  rate_unit: 'hora' | 'dia' | 'mes';
  base_price: number;
};

export type RightsRequest = {
  id: string;
  kind: 'acceso' | 'supresion';
  state: 'recibida' | 'identidad_pendiente' | 'en_revision' | 'resuelta' | 'denegada_fundada';
  created_at: string;
};

export type RightsRequestList = {
  items: Array<RightsRequest>;
};

export type SessionResponse = {
  account_id: string;
  roles: Array<'arrendatario' | 'arrendador' | 'administrador' | 'administrador_global'>;
  capabilities: AdminCapability[];
  /**
   * La cuenta sigue bloqueada administrativamente y requiere operaciones de reserva existentes.
   */
  restricted_mode: boolean;
};

export type AdminCapability =
  | 'legacy.administration'
  | 'accounts.list'
  | 'accounts.read'
  | 'admin_access.manage';

export type SpaceAttributeProfile = {
  category_code: string;
  schema_version: number;
  attributes: Array<{
    code: string;
    label: string;
    description?: string;
    type: 'boolean' | 'integer' | 'number' | 'enum' | 'enum_list';
    order: number;
    unit?: string;
    options?: Array<string>;
    minimum?: number;
    maximum?: number;
    step?: number;
    filter_candidate: boolean;
  }>;
};

export type SpaceCategory = {
  code: string;
  name: string;
};

export type SpaceCategoryList = {
  items: Array<SpaceCategory>;
};

export type PublicLocation = {
  region_code: string;
  region_name: string;
  comuna_code: string;
  comuna_name: string;
};

export type PublicLocationList = {
  items: Array<PublicLocation>;
};

export type SpaceDeletionReceipt = {
  space_id: string;
  state: 'eliminada';
  occurred_at: string;
  removed: Array<string>;
  preserved: Array<string>;
  files_pending_cleanup: number;
  reused: boolean;
};

export type SpaceDraft = {
  id: string;
  category_code: string;
  category_name: string;
  title: string;
  description: string;
  area_m2: number;
  capacity: number;
  usage_rules: string;
  rate_unit: 'hora' | 'dia' | 'mes';
  base_price_clp: number;
  address: string;
  state: 'borrador' | 'activa' | 'oculta';
  /**
   * Zona IANA propia; puede ser nula en datos heredados pendientes de reparación.
   */
  time_zone?: string;
  region_code?: string;
  comuna_code?: string;
  attribute_schema_version: number;
  /**
   * Características declaradas y validadas por el perfil versionado.
   */
  attributes: {
    [key: string]: unknown;
  };
  /**
   * Identificador de portada sintética activa propia; no revela contenido ni URL del archivo privado.
   */
  cover_photo_id?: string;
};

export type SpaceDraftInput = {
  category_code: string;
  title: string;
  description: string;
  area_m2: number;
  capacity: number;
  usage_rules: string;
  rate_unit: 'hora' | 'dia' | 'mes';
  base_price_clp: number;
  /**
   * Dirección privada del borrador.
   */
  address: string;
  region_code?: string;
  comuna_code?: string;
  /**
   * Versión del perfil de atributos; se asume la vigente si se omite.
   */
  attribute_schema_version?: number;
  /**
   * Valores opcionales declarados; claves y tipos se validan contra el perfil de categoría.
   */
  attributes?: {
    [key: string]: unknown;
  };
};

export type SpaceDraftList = {
  items: Array<SpaceDraft>;
};

export type SpacePublicationInput = {
  state: 'activa' | 'oculta';
};

export type SyntheticEvidence = {
  id: string;
  verification_id: string;
  fixture_code: 'synthetic-png-v1';
  mime_type: 'image/png';
  size_bytes: number;
  sha256: string;
  created_at: string;
  /**
   * Falso cuando el archivo fue retirado por vencimiento y solo se conserva metadata temporal.
   */
  file_available: boolean;
};

export type SyntheticEvidenceList = {
  items: Array<SyntheticEvidence>;
};

export type SyntheticSpaceCover = {
  space_id: string;
  photo_id: string;
  selected: true;
};

export type SyntheticSpaceGalleryPhoto = {
  id: string;
  fixture_code: 'synthetic-png-v1';
  mime_type: 'image/png';
  sha256: string;
  size_bytes: number;
  created_at: string;
  public_in_catalog: boolean;
};

export type SyntheticSpaceGalleryRemoval = {
  photo_id: string;
  removed: true;
  reused: boolean;
};

export type TermsResponse = {
  items: Array<{
    id: string;
    code: string;
    type: 'terminos' | 'privacidad';
    sha256: string;
    published_at: UtcTimestamp;
    synthetic: true;
  }>;
};

export type UtcTimestamp = string;

export type VerificationCase = {
  id: string;
  type: 'kyc' | 'kyb';
  state: 'en_revision' | 'aprobada' | 'rechazada' | 'revocada';
  provider: 'local-fixture-v1';
  evidence_ref: string;
  retry_of?: string;
  reason_code?: string;
  correction_code?:
    | 'fixture_vigente_actualizado'
    | 'antecedentes_fixture_actualizados'
    | 'inicio_actividades_fixture_actualizadas'
    | 'legado_pre_v28_sin_codigo';
  revoked_at?: string | null;
  revocation_reason_code?: 'aprobacion_fixture_incorrecta' | 'revision_fixture_actualizada';
  created_at: string;
  resolved_at?: string | null;
};

export type VerificationEligibilityList = {
  scope: 'synthetic_local';
  items: [
    {
      type: 'kyc' | 'kyb';
      eligible: boolean;
      verification_id: string;
      granted_at: string | null;
      revoked_at: string | null;
      revocation_reason_code: string;
    },
    {
      type: 'kyc' | 'kyb';
      eligible: boolean;
      verification_id: string;
      granted_at: string | null;
      revoked_at: string | null;
      revocation_reason_code: string;
    },
  ];
};

export type VerificationHistoryList = {
  items: Array<{
    sequence: number;
    action:
      | 'solicitada'
      | 'revision_aprobada'
      | 'revision_rechazada'
      | 'subsanacion_solicitada'
      | 'elegibilidad_revocada'
      | 'elegibilidad_retirada_privacidad'
      | 'estado_observado_en_migracion';
    from_state?: string;
    to_state: 'en_revision' | 'aprobada' | 'rechazada' | 'revocada' | 'retirada_privacidad';
    reason_code?: string;
    occurred_at: string;
  }>;
};

export type VerificationList = {
  items: Array<VerificationCase>;
};
