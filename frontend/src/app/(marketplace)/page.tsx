import type { Metadata } from "next";
import { Hero } from "@/components/sections/hero";
import { ProductGrid } from "@/components/sections/product-grid";
import { publicCatalog } from "@/features/public-catalog/infrastructure/catalog-runtime";
import { CategoryRail } from "@/features/public-catalog/ui/category-rail";
import {
  CatalogUnavailable,
  EmptyCatalog,
} from "@/features/public-catalog/ui/catalog-states";
import { OwnerCallout } from "@/features/public-catalog/ui/catalog-sections";
import { HowItWorksPreview } from "@/features/public-catalog/ui/how-it-works-preview";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Arriendos residenciales y espacios flexibles",
  description:
    "Encuentra publicaciones públicas para vivir, trabajar, guardar, reunirte o celebrar. Explora y revisa la información disponible en EspaciGo.",
  alternates: { canonical: "/" },
};

export default async function Home() {
  const [categoriesResult, spacesResult, residentialResult] = await Promise.all(
    [
      publicCatalog.categories(),
      publicCatalog.spaces({ attribute: [], page_size: 6 }),
      publicCatalog.spaces({
        category_code: "inmuebles",
        attribute: [],
        page_size: 3,
      }),
    ],
  );

  return (
    <>
      <Hero />
      {categoriesResult.ok ? (
        categoriesResult.value.items.length ? (
          <CategoryRail categories={categoriesResult.value.items} />
        ) : (
          <section className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
            <p className="text-muted-foreground">
              No hay categorías activas disponibles en este momento.
            </p>
          </section>
        )
      ) : (
        <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <CatalogUnavailable
            title="No pudimos cargar las categorías"
            retryHref="/categorias"
          />
        </section>
      )}

      {spacesResult.ok ? (
        spacesResult.value.items.length ? (
          <ProductGrid
            title="Publicaciones del catálogo"
            description="Publicaciones que Go entrega como elegibles para exploración pública."
            spaces={spacesResult.value.items.slice(0, 6)}
          />
        ) : (
          <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <EmptyCatalog
              title="Aún no hay publicaciones públicas para mostrar"
              description="Vuelve a explorar más tarde o revisa las categorías disponibles."
            />
          </section>
        )
      ) : (
        <section className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <CatalogUnavailable title="No pudimos cargar las publicaciones" />
        </section>
      )}

      <section className="bg-secondary/20">
        <div className="mx-auto max-w-7xl px-4 py-14 sm:px-6 sm:py-20 lg:px-8">
          <div className="mb-8 max-w-3xl">
            <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
              Arriendos residenciales
            </p>
            <h2 className="mt-3 text-3xl font-medium tracking-tight sm:text-4xl">
              Arrienda una casa
            </h2>
            <p className="mt-4 leading-relaxed text-muted-foreground">
              Explora casas, departamentos, habitaciones, cabañas y casas de
              veraneo cuando existan publicaciones públicas en el catálogo.
            </p>
          </div>
          {residentialResult.ok ? (
            residentialResult.value.items.length ? (
              <ProductGrid
                title="Publicaciones residenciales"
                description="Resultados reales de la categoría inmuebles; la modalidad y tarifa corresponden a cada publicación."
                spaces={residentialResult.value.items.slice(0, 3)}
                viewAllHref="/arriendos"
              />
            ) : (
              <EmptyCatalog
                title="Aún no hay publicaciones residenciales"
                description="La categoría admite distintas formas de arriendo, pero hoy no hay anuncios residenciales públicos que mostrar."
                ownerCta
              />
            )
          ) : (
            <CatalogUnavailable
              title="No pudimos consultar los arriendos residenciales"
              retryHref="/arriendos"
            />
          )}
        </div>
      </section>

      <HowItWorksPreview />
      <OwnerCallout />
    </>
  );
}
