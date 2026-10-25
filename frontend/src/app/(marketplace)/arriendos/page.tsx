import type { Metadata } from "next";
import Link from "next/link";
import { hasCatalogFilters } from "@/features/public-catalog/domain/query-state";
import { parseCatalogQuery } from "@/features/public-catalog/domain/query-state";
import type {
  PublicCategoryFilter,
  RawCatalogParams,
} from "@/features/public-catalog/domain/types";
import { publicCatalog } from "@/features/public-catalog/infrastructure/catalog-runtime";
import { CatalogFilters } from "@/features/public-catalog/ui/catalog-filters";
import { CatalogSortControls } from "@/features/public-catalog/ui/catalog-sort-controls";
import {
  CatalogUnavailable,
  EmptyCatalog,
} from "@/features/public-catalog/ui/catalog-states";
import { PublicCatalogGrid } from "@/features/public-catalog/ui/catalog-sections";
import { PublicSearchForm } from "@/features/public-catalog/ui/public-search-form";

export const dynamic = "force-dynamic";

export async function generateMetadata({
  searchParams,
}: {
  searchParams: Promise<RawCatalogParams>;
}): Promise<Metadata> {
  const query = parseCatalogQuery(await searchParams);
  const raw = await searchParams;
  const hasRawAttributes = Boolean(
    raw.attribute &&
    (Array.isArray(raw.attribute) ? raw.attribute.length : raw.attribute),
  );
  const filtered =
    hasRawAttributes ||
    hasCatalogFilters({ ...query, category_code: undefined });
  return {
    title: "Arriendos de casas, departamentos y cabañas",
    description:
      "Explora publicaciones residenciales públicas de casas, departamentos, habitaciones, cabañas y casas de veraneo en EspaciGo.",
    alternates: { canonical: "/arriendos" },
    robots: filtered
      ? { index: false, follow: true }
      : { index: true, follow: true },
  };
}

export default async function ResidentialRentalsPage({
  searchParams,
}: {
  searchParams: Promise<RawCatalogParams>;
}) {
  const raw = await searchParams;
  const [categoriesResult, locationsResult] = await Promise.all([
    publicCatalog.categories(),
    publicCatalog.locations({ limit: 400 }),
  ]);
  if (
    categoriesResult.ok &&
    !categoriesResult.value.items.some(
      (category) => category.code === "inmuebles",
    )
  ) {
    return (
      <section className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
        <nav
          aria-label="Breadcrumb"
          className="mb-7 text-sm text-muted-foreground"
        >
          <Link href="/" className="hover:text-foreground">
            Inicio
          </Link>
          <span aria-hidden="true"> / </span>
          <span aria-current="page">Arriendos residenciales</span>
        </nav>
        <h1 className="text-4xl font-medium tracking-tight">
          Arriendos residenciales
        </h1>
        <p className="mt-4 max-w-2xl text-muted-foreground">
          La categoría residencial no está activa en el catálogo público actual.
        </p>
      </section>
    );
  }
  if (!categoriesResult.ok) {
    return (
      <section className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
        <CatalogUnavailable
          title="No pudimos verificar la categoría residencial"
          retryHref="/arriendos"
        />
      </section>
    );
  }

  const [profileResult] = await Promise.all([
    publicCatalog.categoryFilters("inmuebles"),
  ]);
  let categoryFilters: readonly PublicCategoryFilter[] = [];
  const profileUnavailable =
    !profileResult.ok && profileResult.error.status !== 404;
  if (profileResult.ok) categoryFilters = profileResult.value.items;
  const { query, result } = await publicCatalog.spacesFromParams(
    { ...raw, category_code: "inmuebles" },
    categoryFilters,
  );
  const locations = locationsResult.ok ? locationsResult.value.items : [];

  return (
    <section className="mx-auto max-w-7xl px-4 pb-24 pt-8 sm:px-6 sm:pt-12 lg:px-8">
      <nav
        aria-label="Breadcrumb"
        className="mb-7 text-sm text-muted-foreground"
      >
        <Link href="/" className="hover:text-foreground">
          Inicio
        </Link>
        <span aria-hidden="true"> / </span>
        <span aria-current="page">Arriendos residenciales</span>
      </nav>
      <header className="mb-9 max-w-4xl border-b border-border pb-9">
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          Categoría inmuebles conectada al catálogo público
        </p>
        <h1 className="mt-3 text-4xl font-medium tracking-tight sm:text-5xl">
          Arrienda una casa
        </h1>
        <p className="mt-4 text-lg leading-relaxed text-muted-foreground">
          Busca casas, departamentos, habitaciones, estudios, cabañas y casas de
          veraneo. Solo aparecen publicaciones que Go habilita para consulta
          pública; la categoría no garantiza que exista inventario.
        </p>
        <PublicSearchForm initialQuery={query.q ?? ""} />
      </header>
      <div className="grid gap-8 lg:grid-cols-[16rem_minmax(0,1fr)] lg:gap-10">
        <CatalogFilters
          actionPath="/arriendos"
          categories={categoriesResult.value.items}
          locations={locations}
          filters={categoryFilters}
          query={query}
          fixedCategoryCode="inmuebles"
        />
        <div className="min-w-0">
          <div className="mb-7 flex flex-col gap-4 border-b border-border pb-5 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-muted-foreground">
              Publicaciones residenciales públicas
            </p>
            <CatalogSortControls actionPath="/arriendos" query={query} />
          </div>
          {!locationsResult.ok || profileUnavailable ? (
            <p
              role="status"
              className="mb-5 rounded-xl border border-border p-4 text-sm text-muted-foreground"
            >
              Algunos filtros no están disponibles temporalmente; el catálogo
              residencial se consulta igualmente en Go.
            </p>
          ) : null}
          {!result.ok ? (
            result.error.kind === "invalid" ? (
              <section role="alert" className="border-y border-border py-12">
                <h2 className="text-2xl font-medium">
                  No pudimos aplicar esos filtros
                </h2>
                <p className="mt-3 text-muted-foreground">
                  Restablece los filtros y vuelve a intentarlo.
                </p>
                <Link
                  href="/arriendos"
                  className="mt-6 inline-flex min-h-11 items-center rounded-full bg-foreground px-5 text-sm font-medium text-background"
                >
                  Restablecer filtros
                </Link>
              </section>
            ) : (
              <CatalogUnavailable
                title="No pudimos cargar los arriendos residenciales"
                retryHref="/arriendos"
              />
            )
          ) : result.value.items.length ? (
            <PublicCatalogGrid
              spaces={result.value.items}
              query={query}
              nextCursor={result.value.next_cursor}
              actionPath="/arriendos"
              emptyTitle="No hay más publicaciones residenciales"
              emptyDescription="No se recibieron más resultados para este cursor."
              ownerCta
            />
          ) : (
            <EmptyCatalog
              title="Aún no hay publicaciones residenciales"
              description="La categoría inmuebles está activa, pero ahora no hay anuncios públicos elegibles para mostrar. La oferta puede incluir casas, departamentos, habitaciones, cabañas y casas de veraneo."
              ownerCta
            />
          )}
        </div>
      </div>
    </section>
  );
}
