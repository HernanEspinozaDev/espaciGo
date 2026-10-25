import type { RawCatalogParams } from "@/features/public-catalog/domain/types";
import { redirectLegacyCatalog } from "@/features/public-catalog/application/legacy-redirect";

export default async function LegacyProductsPage({
  searchParams,
}: {
  searchParams: Promise<RawCatalogParams>;
}) {
  await redirectLegacyCatalog(await searchParams);
}
