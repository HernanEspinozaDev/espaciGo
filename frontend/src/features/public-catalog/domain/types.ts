import type { components, operations } from "@/lib/api/generated/openapi";

export type PublicSpace = components["schemas"]["PublicSpace"];
export type PublicSpaceDetail = components["schemas"]["PublicSpaceDetail"];
export type PublicSpaceImageRef = components["schemas"]["PublicSpaceImageRef"];
export type PublicSpaceCategory = components["schemas"]["PublicSpaceCategory"];
export type PublicLocation = components["schemas"]["PublicLocation"];
export type PublicCategoryFilter =
  components["schemas"]["PublicCategoryFilter"];
export type TermsResponse = components["schemas"]["TermsResponse"];
export type CatalogSort = NonNullable<
  NonNullable<operations["listPublicSpaces"]["parameters"]["query"]>["sort"]
>;

export type CatalogQuery = {
  q?: string;
  category_code?: string;
  region_code?: string;
  comuna_code?: string;
  min_area_m2?: string;
  attribute: string[];
  rate_unit?: PublicSpace["rate_unit"];
  min_unit_price_clp?: string;
  max_unit_price_clp?: string;
  start_at?: string;
  end_at?: string;
  min_total_clp?: string;
  max_total_clp?: string;
  sort?: CatalogSort;
  page_size?: number;
  cursor?: string;
};

export type RawCatalogParams = Record<string, string | string[] | undefined>;

export type CatalogFailure = {
  kind: "unavailable" | "invalid" | "not-found";
  status?: number;
  code?: string;
};

export type CatalogResult<T> =
  { ok: true; value: T } | { ok: false; error: CatalogFailure };

export type PublicSpacePage = components["schemas"]["PublicSpacePage"];
export type PublicSpaceCategories =
  components["schemas"]["PublicSpaceCategories"];
export type PublicCategoryFilters =
  components["schemas"]["PublicCategoryFilters"];
export type PublicLocations = components["schemas"]["PublicLocations"];
