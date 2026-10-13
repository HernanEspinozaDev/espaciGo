import type { SpaceAttributeProfile, SpaceCategoryList } from '../../../../core/contracts/models';
export type ClpAmount = string;
export type PublicSpace = Pick<CatalogItem, 'space_id'|'category_code'|'category_name'|'title'|'description'|'area_m2'|'rate_unit'|'base_price_clp'|'currency'|'estimated_total_clp'|'region_code'|'region_name'|'comuna_code'|'comuna_name'>;
export type PublicSpaceDetail = PublicSpace & { usage_rules:string; features?:{code:string;label:string;value:boolean}[]; images?:{image_id:string;is_cover:boolean}[] };
export type PublicCategory = {code:string;name:string;order:number};
export type PublicCatalogFilters = {q?:string;category_code?:string;rate_unit?:string;min_unit_price_clp?:string;max_unit_price_clp?:string;region_code?:string;comuna_code?:string;min_area_m2?:string;attributes?:Record<string,boolean>;start_at?:string;end_at?:string;min_total_clp?:string;max_total_clp?:string;sort?:string;page_size?:number;cursor?:string};
export type PublicLocation={region_code:string;region_name:string;comuna_code:string;comuna_name:string};
export type PublicAttributeFilter={code:string;label:string;type:'boolean';profile_version:number};
export type PublicSpacePage = {items:PublicSpace[];next_cursor?:string};
export type CatalogItem = {
  space_id: string;
  category_code: string;
  category_name: string;
  title: string;
  description: string;
  area_m2: number;
  rate_unit: 'hora' | 'dia' | 'mes';
  base_price_clp: ClpAmount;
  currency: 'CLP';
  time_zone: string;
  profile_version: number;
  profile: Record<string, unknown>;
  attributes: Record<string, unknown>;
  available?: boolean;
  estimated_total_clp?: ClpAmount | null;
  region_code?: string | null;
  region_name?: string | null;
  comuna_code?: string | null;
  comuna_name?: string | null;
  distance_km?: number;
  distance_kind?: 'direct';
  map_position?: { x: number; y: number };
};
export type CatalogEnvelope = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: { items: CatalogItem[]; next_cursor?: string };
};
export type AvailabilityInterval = { start_at: string; end_at: string };
export type BusinessClock = { data: { now: string } };
export type AvailabilityOptions = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: {
    space_id: string;
    time_zone: string;
    rate_unit: CatalogItem['rate_unit'];
    items: AvailabilityInterval[];
  };
};
export type BookingQuote = {
  id: string;
  space_id: string;
  rate_version: number;
  rate_unit: CatalogItem['rate_unit'];
  unit_price_clp: ClpAmount;
  currency: 'CLP';
  units: number;
  subtotal_clp: ClpAmount;
  start_at: string;
  end_at: string;
  time_zone: string;
  conditions: string;
  category_code: string;
  profile_version: number;
  profile_values: Record<string, unknown>;
  cancellation_policy_version: string;
  guarantee_policy_version: string;
  guarantee_currency: 'CLP';
  guarantee_expected_clp: ClpAmount;
  settlement_policy_version: string;
  settlement_commission_bps: number;
  created_at: string;
  expires_at: string;
};
export type QuoteEnvelope = {
  safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL';
  data: BookingQuote;
};
export type CatalogFilters = {
  q?: string;
  category_code?: string;
  min_area_m2?: string;
  start_at?: string;
  end_at?: string;
  min_total_clp?: string;
  max_total_clp?: string;
  latitude?: string;
  longitude?: string;
  radius_km?: string;
  attributes?: Record<string, unknown>;
  page_size?: number;
  cursor?: string;
};
export class ExploreApiError extends Error {
  constructor(
    message: string,
    readonly status?: number,
  ) {
    super(message);
    this.name = 'ExploreApiError';
  }
}

export abstract class ExplorePort {
  abstract publicSearch(filters:PublicCatalogFilters):Promise<PublicSpacePage>;
  abstract publicDetail(spaceId:string):Promise<PublicSpaceDetail>;
  abstract publicImageUrl(spaceId:string,imageId:string):string;
  abstract publicCategories():Promise<PublicCategory[]>;
  abstract publicLocations():Promise<PublicLocation[]>;
  abstract publicAttributeFilters(categoryCode:string):Promise<PublicAttributeFilter[]>;
  abstract search(token: string, filters: CatalogFilters): Promise<CatalogEnvelope>;
  abstract detail(token: string, spaceId: string): Promise<CatalogItem>;
  abstract categories(token: string): Promise<SpaceCategoryList>;
  abstract profile(token: string, categoryCode: string): Promise<SpaceAttributeProfile>;
  abstract profileVersion(token: string, categoryCode: string): Promise<number>;
  abstract businessClock(token: string): Promise<BusinessClock>;
  abstract availability(
    token: string,
    spaceId: string,
    date: string,
    duration: number,
  ): Promise<AvailabilityOptions>;
  abstract quote(
    token: string,
    spaceId: string,
    startAt: string,
    endAt: string,
  ): Promise<QuoteEnvelope>;
}
