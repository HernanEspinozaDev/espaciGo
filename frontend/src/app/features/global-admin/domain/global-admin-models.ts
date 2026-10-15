export type Account = {
  account_id: string;
  email: string;
  display_name: string | null;
  phone: string | null;
  profile_updated_at: string | null;
  state: string;
  created_at: string;
  administrative_blocked: boolean;
  roles: string[];
  publication_count: number;
  reservation_count: number;
  incident_count: number;
};

export type ProfileUpdate = { display_name: string; phone: string | null; expected_updated_at: string; reason: string };
export type ProfileUpdateResult = { account_id: string; email: string; display_name: string; phone: string | null; updated_at: string };
export type RoleUpdate = { granted: boolean; reason: string };
export type SessionRevocation = { reason: string };
export type AccountBlockUpdate = { blocked: boolean; reason: string };
export type AttributeDefinition = {code:string;label:string;description?:string;type:'boolean'|'integer'|'number'|'enum'|'enum_list';unit?:string;options?:string[];minimum?:number;maximum?:number;step?:number};
export type Publication = { publication_id: string; owner_id: string; owner_name: string | null; owner_email: string; category_code: string; category_name: string; title: string; description:string; area_m2:string; capacity:number; usage_rules:string; address:string; state: string; rate_unit: 'hora'|'dia'|'mes'; base_price_clp: string; updated_at: string; publicly_visible: boolean; public_cover_available: boolean; attribute_schema_version:number; attributes:Record<string,unknown>; attribute_schema:{category_code:string;schema_version:number;attributes:AttributeDefinition[]} };
export type PublicationPage = { items: Publication[]; next_cursor: string; matching_total: number; generated_at: string };
export type AdminPublicationReason = 'correccion_perfil'|'revision_administrativa'|'gestion_operativa';
export type PublicationStateUpdate = { state: 'activa'|'oculta'; expected_updated_at: string; reason: AdminPublicationReason };
export type PublicationContentUpdate = { title: string; description: string; capacity: number; usage_rules: string; base_price_clp: string; attribute_schema_version:number; attributes:Record<string,unknown>; expected_updated_at: string; reason: AdminPublicationReason };
export type PublicationRetireUpdate = { owner_id: string; expected_updated_at: string; reason: 'moderacion_contenido'|'riesgo_seguridad'|'solicitud_titular'|'revision_administrativa' };
export type PublicationPhoto = {id:string;fixture_code:string;mime_type:string;sha256:string;size_bytes:number;created_at:string;public_in_catalog:boolean;state?:string;cover?:boolean;file_pending_cleanup?:boolean};
export type PublicationPhotoMutation = {owner_id:string;enabled?:boolean;expected_updated_at:string;reason:string};
export type PublicationSchedule = {space_id:string;time_zone:string;enabled:boolean;days:{weekday:number;periods:{open:string;close:string}[]}[]};
export type PublicationScheduleUpdate = {owner_id:string;enabled:boolean;days:PublicationSchedule['days'];expected_updated_at:string;reason:AdminPublicationReason};

export type Page = {
  items: Account[];
  next_cursor: string;
  matching_total: number;
  generated_at: string;
};

export type AccountResourceKind = 'reservations' | 'incidents';
export type AccountResource = { id: string; kind: string; space_id: string; reservation_id: string; state: string; participant_role: string; occurred_at: string };
export type AccountResourcePage = { items: AccountResource[]; next_cursor: string; matching_total: number; generated_at: string };

export type Overview = {
  total_accounts: number;
  administrative_blocked: number;
  by_state: Record<string, number>;
  by_role: Record<string, number>;
  generated_at: string;
  definition: string;
};
