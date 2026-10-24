import type { Metadata } from "next";

import Link from "next/link";
import { notFound } from "next/navigation";
import { Suspense } from "react";
import { MediaGallery } from "@/app/product/[slug]/media-gallery";
import { ProductFeatures } from "@/app/product/[slug]/product-features";
import { ProductCard } from "@/components/product-card";
import { ProductGridSkeleton } from "@/components/product-grid-skeleton";
import {
  getSpaceBySlug,
  getSpaces,
  formatDemoClp,
} from "@/lib/marketplace-fixtures";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const space = getSpaceBySlug(slug);
  return space
    ? {
        title: space.name,
        description: space.summary,
        alternates: { canonical: `/product/${space.slug}` },
      }
    : {
        title: "Espacio no encontrado",
        robots: { index: false, follow: true },
      };
}

export default async function SpacePage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const space = getSpaceBySlug(slug);
  if (!space) notFound();
  const related = getSpaces({ category: space.categorySlug })
    .data.filter((item) => item.id !== space.id)
    .slice(0, 3);

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <nav
        aria-label="Ruta de navegación"
        className="mb-6 text-sm text-muted-foreground"
      >
        <Link href="/" className="hover:text-foreground">
          Inicio
        </Link>
        <span className="px-2" aria-hidden="true">
          /
        </span>
        <Link href="/products" className="hover:text-foreground">
          Espacios
        </Link>
        <span className="px-2" aria-hidden="true">
          /
        </span>
        <span aria-current="page" className="text-foreground">
          {space.name}
        </span>
      </nav>
      <div className="lg:grid lg:grid-cols-2 lg:gap-16">
        <Suspense fallback={<ProductGridSkeleton className="lg:grid-cols-1" />}>
          <MediaGallery images={[...space.images]} productName={space.name} />
        </Suspense>
        <div className="mt-8 space-y-8 lg:mt-0">
          <div className="space-y-3">
            <Link
              href={`/products?category=${space.categorySlug}`}
              className="inline-flex min-h-6 items-center text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground hover:text-foreground"
            >
              {space.category}
            </Link>
            <h1 className="text-4xl font-medium tracking-tight text-foreground lg:text-5xl">
              {space.name}
            </h1>
            <p className="text-muted-foreground">
              {space.comuna}, {space.city} · {space.capacity}
            </p>
          </div>
          <div className="rounded-2xl border border-border p-6 sm:p-8">
            <div className="flex flex-wrap items-baseline gap-2">
              <span className="text-3xl font-semibold">
                {formatDemoClp(space.rateClpPerDay)}
              </span>
              <span className="text-muted-foreground">/ día</span>
            </div>
            <p className="mt-2 text-sm text-muted-foreground">
              Tarifa ilustrativa; no es una oferta ni una cotización.
            </p>
            <button
              type="button"
              disabled
              className="mt-6 inline-flex h-12 w-full items-center justify-center rounded-full bg-foreground px-6 font-medium text-background opacity-60"
            >
              Reservas no disponibles en esta demo
            </button>
          </div>
          <section>
            <h2 className="text-lg font-medium">Sobre este espacio</h2>
            <p className="mt-3 leading-relaxed text-muted-foreground">
              {space.description}
            </p>
          </section>
          <p className="border-t border-border pt-5 text-xs leading-relaxed text-muted-foreground">
            Ficha local sintética. No representa una publicación real, un
            propietario, disponibilidad confirmada ni condiciones contractuales.
          </p>
        </div>
      </div>
      <ProductFeatures
        features={space.amenities.map((amenity) => ({
          title: amenity,
          description: "Atributo ilustrativo de esta ficha sintética.",
        }))}
      />
      {related.length > 0 && (
        <section className="mt-20 border-t border-border pt-12">
          <div className="mb-8 flex items-end justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
                También en {space.category}
              </p>
              <h2 className="mt-2 text-2xl font-medium sm:text-3xl">
                Más espacios
              </h2>
            </div>
            <Link
              href={`/products?category=${space.categorySlug}`}
              className="text-sm text-muted-foreground underline underline-offset-4 hover:text-foreground"
            >
              Ver categoría
            </Link>
          </div>
          <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
            {related.map((item, index) => (
              <ProductCard key={item.id} space={item} priority={index === 0} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
