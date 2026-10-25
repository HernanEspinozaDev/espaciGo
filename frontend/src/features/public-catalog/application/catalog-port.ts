import type {
  CatalogQuery,
  CatalogResult,
  PublicCategoryFilters,
  PublicLocations,
  PublicSpaceCategories,
  PublicSpaceDetail,
  PublicSpacePage,
  TermsResponse,
} from "../domain/types";

export interface PublicCatalogPort {
  listSpaces(query: CatalogQuery): Promise<CatalogResult<PublicSpacePage>>;
  listCategories(): Promise<CatalogResult<PublicSpaceCategories>>;
  getCategoryFilters(
    code: string,
  ): Promise<CatalogResult<PublicCategoryFilters>>;
  listLocations(query?: {
    q?: string;
    region_code?: string;
    limit?: number;
  }): Promise<CatalogResult<PublicLocations>>;
  getSpace(spaceId: string): Promise<CatalogResult<PublicSpaceDetail>>;
  listTerms(): Promise<CatalogResult<TermsResponse>>;
}
