import { redirect } from "next/navigation";
import {
  parseCatalogQuery,
  serializeCatalogQuery,
} from "../domain/query-state";
import type { PublicCategoryFilter, RawCatalogParams } from "../domain/types";
import { publicCatalog } from "../infrastructure/catalog-runtime";

export async function redirectLegacyCatalog(params: RawCatalogParams) {
  let safeParams = { ...params };
  const categoryValue = params.category_code;
  const categoryCode =
    typeof categoryValue === "string" ? categoryValue : undefined;
  let filters: readonly PublicCategoryFilter[] = [];
  if (categoryCode) {
    const [categories, profile] = await Promise.all([
      publicCatalog.categories(),
      publicCatalog.categoryFilters(categoryCode),
    ]);
    if (
      categories.ok &&
      !categories.value.items.some((category) => category.code === categoryCode)
    ) {
      safeParams = {
        ...safeParams,
        category_code: undefined,
        attribute: undefined,
      };
    } else if (profile.ok) {
      filters = profile.value.items;
    }
  }
  const query = parseCatalogQuery(safeParams, filters);
  const suffix = serializeCatalogQuery(query).toString();
  redirect(suffix ? `/explorar?${suffix}` : "/explorar");
}
