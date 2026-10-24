import { searchSpaces } from "@/lib/marketplace-fixtures";

export type SearchSuggestion = {
  id: string;
  slug: string;
  name: string;
  summary: string;
  image: string;
};

export async function searchSuggest(
  query: string,
): Promise<SearchSuggestion[]> {
  return searchSpaces(query)
    .slice(0, 5)
    .map((space) => ({
      id: space.id,
      slug: space.slug,
      name: space.name,
      summary: `${space.category} · ${space.comuna}, ${space.city}`,
      image: space.images[0],
    }));
}
