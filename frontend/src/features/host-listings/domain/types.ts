import type { components } from "@/lib/api/generated/openapi";

export type RateUnit = "hora" | "dia" | "mes";
export type ListingState = "borrador" | "activa" | "oculta" | "eliminada";

export interface Category {
  readonly code: string;
  readonly name: string;
}

export type AttributeType =
  "boolean" | "integer" | "number" | "enum" | "enum_list";

export interface CategoryAttributeDefinition {
  readonly code: string;
  readonly label: string;
  readonly description?: string;
  readonly type: AttributeType;
  readonly required?: boolean;
  readonly order: number;
  readonly unit?: string;
  readonly options?: readonly string[];
  readonly minimum?: number;
  readonly maximum?: number;
  readonly step?: number;
  readonly filter_candidate: boolean;
}

export interface CategoryProfileSchema {
  readonly category_code: string;
  readonly schema_version: number;
  readonly attributes: readonly CategoryAttributeDefinition[];
}

export interface ListingSummary {
  readonly id: string;
  readonly category_code: string;
  readonly category_name: string;
  readonly title: string;
  readonly state: ListingState;
  readonly base_price_clp: number;
  readonly rate_unit: RateUnit;
  readonly area_m2: number;
  readonly capacity: number;
  readonly cover_photo_id?: string | null;
  readonly time_zone?: string | null;
  readonly region_code?: string;
  readonly comuna_code?: string;
}

export interface ListingDetail {
  readonly id: string;
  readonly category_code: string;
  readonly category_name: string;
  readonly title: string;
  readonly description: string;
  readonly area_m2: number;
  readonly capacity: number;
  readonly usage_rules: string;
  readonly rate_unit: RateUnit;
  readonly base_price_clp: number;
  readonly address: string;
  readonly state: ListingState;
  readonly time_zone?: string | null;
  readonly region_code?: string;
  readonly comuna_code?: string;
  readonly attribute_schema_version: number;
  readonly attributes: Readonly<Record<string, unknown>>;
  readonly cover_photo_id?: string | null;
}

export interface ListingDraftInput {
  category_code: string;
  title: string;
  description: string;
  area_m2: number;
  capacity: number;
  usage_rules: string;
  rate_unit: RateUnit;
  base_price_clp: number;
  address: string;
  region_code?: string;
  comuna_code?: string;
  attribute_schema_version?: number;
  attributes?: Record<string, unknown>;
}

export interface ContentUpdateInput {
  title?: string;
  description?: string;
  capacity?: number;
  usage_rules?: string;
  base_price_clp?: number | string;
  region_code?: string;
  comuna_code?: string;
}

export interface PricingInput {
  rate_unit: RateUnit;
  base_price: number;
}

export interface PricingHistoryItem {
  readonly space_id: string;
  readonly version: number;
  readonly rate_unit: RateUnit;
  readonly base_price: number;
  readonly currency: "CLP";
  readonly created_at: string;
}

export interface ScheduleSlot {
  readonly day_of_week: number;
  readonly start_time: string;
  readonly end_time: string;
}

export interface ScheduleInput {
  timezone: string;
  weekly_slots?: ScheduleSlot[];
  enabled?: boolean;
}

export interface LocalWeeklyPeriod {
  open: string;
  close: string;
}

export interface LocalWeeklyDay {
  weekday: number;
  periods: LocalWeeklyPeriod[];
}

export interface ScheduleData {
  readonly space_id: string;
  readonly enabled: boolean;
  readonly time_zone: string;
  readonly days: readonly LocalWeeklyDay[];
}

export interface GalleryPhoto {
  readonly id: string;
  readonly space_id?: string;
  readonly mime_type: string;
  readonly sha256: string;
  readonly size_bytes: number;
  readonly created_at: string;
  readonly is_synthetic: boolean;
  readonly is_cover: boolean;
  readonly visible: boolean;
  readonly position: number;
  readonly alt_text?: string;
}

export interface PhotoUploadInput {
  image?: Blob | File;
  caption?: string;
  is_cover?: boolean;
}

export interface ListingDeletionReceipt {
  readonly space_id: string;
  readonly state: "eliminada";
  readonly occurred_at: string;
  readonly removed: readonly string[];
  readonly preserved: readonly string[];
  readonly files_pending_cleanup: number;
  readonly reused: boolean;
}

export type OpenApiSpaceDraft = components["schemas"]["SpaceDraft"];
export type OpenApiSpaceCategory = components["schemas"]["SpaceCategory"];
export type OpenApiSpaceAttributeProfile =
  components["schemas"]["SpaceAttributeProfile"];
export type OpenApiRate = components["schemas"]["Rate"];
export type OpenApiLocalWeeklyHours = components["schemas"]["LocalWeeklyHours"];
