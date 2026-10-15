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
export type Publication = { publication_id: string; owner_id: string; owner_name: string | null; owner_email: string; category_code: string; category_name: string; title: string; description:string; area_m2:string; capacity:number; usage_rules:string; address:string; state: string; rate_unit: 'hora'|'dia'|'mes'; base_price_clp: string; updated_at: string; publicly_visible: boolean; public_cover_available: boolean };
export type PublicationPage = { items: Publication[]; next_cursor: string; matching_total: number; generated_at: string };
export type PublicationStateUpdate = { state: 'activa'|'oculta'; expected_updated_at: string; reason: string };
export type PublicationContentUpdate = { title: string; description: string; capacity: number; usage_rules: string; base_price_clp: string; expected_updated_at: string; reason: string };

export type Page = {
  items: Account[];
  next_cursor: string;
  matching_total: number;
  generated_at: string;
};

export type Overview = {
  total_accounts: number;
  administrative_blocked: number;
  by_state: Record<string, number>;
  by_role: Record<string, number>;
  generated_at: string;
  definition: string;
};
