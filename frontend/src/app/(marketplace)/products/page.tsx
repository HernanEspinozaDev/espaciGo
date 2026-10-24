import type { Metadata } from "next";
import { Suspense } from "react";
import { ListingPagination } from "@/components/listing-pagination";
import { ProductCard } from "@/components/product-card";
import { ProductGridSkeleton } from "@/components/product-grid-skeleton";
import {
  ProductFilters,
  ProductFiltersMobile,
} from "@/components/sections/product-filters";
import {
  getSpaceFacets,
  getSpaces,
  type SpaceFilters,
} from "@/lib/marketplace-fixtures";
import { SortLinks, SortSelect } from "@/app/products/products-sort-select";

const PAGE_SIZE = 9;
const sortOptions = [
  { value: "newest", label: "Destacados" },
  { value: "price-asc", label: "Menor tarifa" },
  { value: "price-desc", label: "Mayor tarifa" },
  { value: "name", label: "Nombre" },
] as const;

export async function generateMetadata({
  searchParams,
}: {
  searchParams: Promise<{ page?: string }>;
}): Promise<Metadata> {
  const { page } = await searchParams;
  return {
    title:
      page && Number(page) > 1
        ? `Espacios · página ${page}`
        : "Explorar espacios",
    description: "Explora un catálogo local de espacios de demostración.",
    alternates: { canonical: "/products" },
  };
}

export default async function ProductsPage({
  searchParams,
}: {
  searchParams: Promise<SpaceFilters>;
}) {
  const filters = await searchParams;
  const currentPage = Math.max(1, Number(filters.page) || 1);
  const result = getSpaces(filters);
  const totalPages = Math.ceil(result.total / PAGE_SIZE);
  const spaces = result.data.slice(
    (currentPage - 1) * PAGE_SIZE,
    currentPage * PAGE_SIZE,
  );
  const facets = getSpaceFacets();

  return (
    <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 sm:py-16 lg:px-8">
      <header className="mb-10">
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          Catálogo de muestra
        </p>
        <h1 className="mt-3 text-3xl font-medium tracking-tight sm:text-4xl">
          Explorar espacios
        </h1>
        <p className="mt-2 text-muted-foreground">
          Encuentra un espacio por categoría, ubicación o tarifa diaria
          ilustrativa.
        </p>
      </header>
      <Suspense fallback={<ProductGridSkeleton className="lg:grid-cols-3" />}>
        <div className="lg:grid lg:grid-cols-[16rem_minmax(0,1fr)] lg:gap-10">
          <ProductFilters facets={facets} />
          <div>
            <div className="mb-8 flex items-center justify-between gap-3 lg:hidden">
              <ProductFiltersMobile facets={facets} />
              <SortSelect options={sortOptions} />
            </div>
            <div className="mb-8 hidden flex-wrap items-center gap-3 lg:flex">
              <span className="text-sm text-muted-foreground">Ordenar:</span>
              <SortLinks options={sortOptions} />
            </div>
            {spaces.length ? (
              <>
                <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 xl:grid-cols-3">
                  {spaces.map((space, index) => (
                    <ProductCard
                      key={space.id}
                      space={space}
                      priority={index === 0}
                    />
                  ))}
                </div>
                <ListingPagination
                  basePath="/products"
                  currentPage={currentPage}
                  totalPages={totalPages}
                  filters={filters}
                />
              </>
            ) : (
              <div className="py-24 text-center">
                <p className="text-lg text-muted-foreground">
                  No hay espacios que coincidan con estos filtros.
                </p>
                <p className="mt-2 text-sm text-muted-foreground">
                  Prueba otra categoría, comuna o rango de tarifa.
                </p>
              </div>
            )}
          </div>
        </div>
      </Suspense>
    </div>
  );
}
