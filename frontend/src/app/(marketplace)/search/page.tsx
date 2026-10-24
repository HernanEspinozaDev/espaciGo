import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { ProductCard } from "@/components/product-card";
import { SearchPageInput } from "@/components/search/search-page-input";
import {
  ActiveFilters,
  CategorySidebar,
  MobileControls,
  SortSelect,
} from "@/app/search/search-controls";
import { SearchPagination } from "@/app/search/search-pagination";
import { getSortFromParams, type SortKey } from "@/app/search/sort";
import {
  getSpaces,
  searchSpaces,
  spaceCategories,
  type SpaceFilters,
} from "@/lib/marketplace-fixtures";

const PAGE_SIZE = 9;

export async function generateMetadata({
  searchParams,
}: {
  searchParams: Promise<{ q?: string }>;
}): Promise<Metadata> {
  const { q } = await searchParams;
  return {
    title: q ? `Búsqueda: ${q}` : "Buscar espacios",
    description: q
      ? `Resultados locales para ${q}.`
      : "Busca espacios en el catálogo de muestra.",
    alternates: { canonical: "/search" },
    robots: { index: false, follow: true },
  };
}

function SearchResults({
  query,
  filters,
  sort,
}: {
  query: string;
  filters: SpaceFilters;
  sort: SortKey;
}) {
  const result = getSpaces({ ...filters, q: query, sort });
  const currentPage = Math.max(1, Number(filters.page) || 1);
  const spaces = result.data.slice(
    (currentPage - 1) * PAGE_SIZE,
    currentPage * PAGE_SIZE,
  );
  const totalPages = Math.ceil(result.total / PAGE_SIZE);

  if (spaces.length === 0) {
    return (
      <div>
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          Sin resultados
        </p>
        <h2 className="mt-3 max-w-2xl text-3xl font-medium leading-[1.1] tracking-tight text-foreground sm:text-5xl">
          No encontramos “{query}”
        </h2>
        <p className="mt-6 max-w-md text-sm text-muted-foreground">
          Prueba una palabra más corta o elimina algún filtro.
        </p>
      </div>
    );
  }

  return (
    <div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-10 sm:grid-cols-2 sm:gap-x-6 sm:gap-y-14 lg:grid-cols-3 lg:gap-x-8">
        {spaces.map((space, index) => (
          <ProductCard key={space.id} space={space} priority={index === 0} />
        ))}
      </div>
      <SearchPagination
        currentPage={currentPage}
        totalPages={totalPages}
        query={query}
        sort={sort}
        category={filters.category}
      />
    </div>
  );
}

function EmptyQuery() {
  return (
    <div className="border-t border-border pt-12 sm:pt-20">
      <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
        Buscar
      </p>
      <h2 className="mt-3 max-w-3xl text-4xl font-medium leading-[1.05] tracking-tight text-foreground sm:text-6xl">
        ¿Qué espacio estás buscando?
      </h2>
      <div className="mt-12">
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          O explora por categoría
        </p>
        <ul className="mt-4 flex flex-wrap gap-x-6 gap-y-3 text-lg">
          {spaceCategories.map((category) => (
            <li key={category.slug}>
              <Link
                href={`/products?category=${category.slug}`}
                className="border-b border-foreground/30 pb-0.5 text-foreground"
              >
                {category.name}
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

export default async function SearchPage({
  searchParams,
}: {
  searchParams: Promise<SpaceFilters>;
}) {
  const filters = await searchParams;
  const query = filters.q?.trim() ?? "";
  const sort = getSortFromParams(filters.sort);
  const count = query ? getSpaces({ ...filters, q: query, sort }).total : 0;
  const category = filters.category;
  const categories = spaceCategories.map((item) => ({
    id: item.slug,
    ...item,
  }));
  const activeCategory = category
    ? categories.find((item) => item.slug === category)
    : undefined;
  const filterActive = searchSpaces(query).length > 0;

  return (
    <section className="mx-auto max-w-7xl px-4 pb-24 sm:px-6 lg:px-8">
      <div className="pb-4 pt-8 sm:pb-6 sm:pt-12">
        <div className="mx-auto max-w-3xl">
          <SearchPageInput initialQuery={query} />
        </div>
      </div>
      {query ? (
        <>
          <header className="pb-8 pt-10 sm:pb-12 sm:pt-14">
            <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
                  Resultados para
                </p>
                <h1 className="mt-3 max-w-[14ch] text-4xl font-medium leading-[1.05] tracking-tight text-foreground sm:text-5xl lg:text-6xl">
                  {query}
                </h1>
                <div className="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2">
                  <span className="text-xs uppercase tracking-[0.18em] tabular-nums text-muted-foreground">
                    {count} {count === 1 ? "espacio" : "espacios"}
                  </span>
                  {activeCategory && (
                    <ActiveFilters
                      categories={categories}
                      activeCategory={activeCategory.slug}
                    />
                  )}
                  {!filterActive && (
                    <span className="text-xs text-muted-foreground">
                      Datos sintéticos locales
                    </span>
                  )}
                </div>
              </div>
              <div className="hidden sm:block">
                <SortSelect value={sort} />
              </div>
            </div>
          </header>
          <MobileControls
            categories={categories}
            activeCategory={category}
            sort={sort}
            totalCount={count}
          />
          <div className="grid gap-x-10 gap-y-12 pt-8 sm:grid-cols-[180px_1fr] sm:gap-x-12 sm:pt-0 lg:grid-cols-[200px_1fr] lg:gap-x-16">
            <aside className="hidden sm:block">
              <div className="sticky top-24">
                <CategorySidebar
                  categories={categories}
                  activeCategory={category}
                  totalCount={count}
                />
              </div>
            </aside>
            <Suspense
              fallback={
                <div className="grid grid-cols-2 gap-6 lg:grid-cols-3">
                  {Array.from({ length: 6 }, (_, i) => (
                    <div
                      key={i}
                      className="aspect-square animate-pulse bg-secondary motion-reduce:animate-none"
                    />
                  ))}
                </div>
              }
            >
              <SearchResults query={query} filters={filters} sort={sort} />
            </Suspense>
          </div>
        </>
      ) : (
        <EmptyQuery />
      )}
    </section>
  );
}
