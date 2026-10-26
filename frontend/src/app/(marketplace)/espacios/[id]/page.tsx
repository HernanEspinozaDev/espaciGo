import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MediaGallery } from "@/app/product/[slug]/media-gallery";
import { ProductFeatures } from "@/app/product/[slug]/product-features";
import {
  isPublicUuid,
  publicGalleryImages,
} from "@/features/public-catalog/domain/media";
import {
  formatPublicClp,
  rateUnitLabel,
  safePublicText,
} from "@/features/public-catalog/domain/presentation";
import { publicCatalog } from "@/features/public-catalog/infrastructure/catalog-runtime";
import { CatalogUnavailable } from "@/features/public-catalog/ui/catalog-states";

export const dynamic = "force-dynamic";
type PageProps = { params: Promise<{ id: string }> };

function categoryHref(code: string) {
  return `/categorias/${encodeURIComponent(code)}`;
}

function jsonLd(value: unknown) {
  return JSON.stringify(value)
    .replace(/</g, "\\u003c")
    .replace(/>/g, "\\u003e")
    .replace(/&/g, "\\u0026");
}

export async function generateMetadata({
  params,
}: PageProps): Promise<Metadata> {
  const { id } = await params;
  if (!isPublicUuid(id))
    return {
      title: "Espacio no encontrado",
      robots: { index: false, follow: true },
    };
  const result = await publicCatalog.space(id);
  if (!result.ok)
    return {
      title: "Espacio no disponible",
      robots: { index: false, follow: true },
    };
  const space = result.value;
  return {
    title: safePublicText(space.title),
    description: safePublicText(space.description).slice(0, 180),
    alternates: { canonical: `/espacios/${encodeURIComponent(id)}` },
  };
}

export default async function PublicSpacePage({ params }: PageProps) {
  const { id } = await params;
  if (!isPublicUuid(id)) notFound();
  const result = await publicCatalog.space(id);
  if (!result.ok) {
    if (result.error.kind === "not-found") notFound();
    return (
      <section className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
        <CatalogUnavailable
          title="No pudimos cargar esta publicación"
          retryHref={`/espacios/${encodeURIComponent(id)}`}
        />
      </section>
    );
  }
  const space = result.value;
  const title = safePublicText(space.title);
  const description = safePublicText(space.description);
  const rules = safePublicText(space.usage_rules);
  const images = publicGalleryImages(space.space_id, space.images);
  const price = formatPublicClp(space.base_price_clp);
  const estimate = formatPublicClp(space.estimated_total_clp);
  const features = space.features.filter((feature) => feature.value);
  const placeJsonLd = {
    "@context": "https://schema.org",
    "@type": "Place",
    name: title,
    description,
  };

  return (
    <article className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <nav
        aria-label="Breadcrumb"
        className="mb-7 text-sm text-muted-foreground"
      >
        <Link href="/" className="hover:text-foreground">
          Inicio
        </Link>
        <span aria-hidden="true" className="px-2">
          /
        </span>
        <Link href="/explorar" className="hover:text-foreground">
          Explorar
        </Link>
        <span aria-hidden="true" className="px-2">
          /
        </span>
        <Link
          href={categoryHref(space.category_code)}
          className="hover:text-foreground"
        >
          {space.category_name}
        </Link>
        <span aria-hidden="true" className="px-2">
          /
        </span>
        <span aria-current="page" className="text-foreground">
          {title}
        </span>
      </nav>
      <div className="lg:grid lg:grid-cols-2 lg:gap-16">
        <MediaGallery
          key={space.space_id}
          images={images}
          productName={title}
        />
        <div className="mt-8 space-y-8 lg:mt-0">
          <header className="space-y-3">
            <Link
              href={categoryHref(space.category_code)}
              className="inline-flex min-h-8 items-center text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground hover:text-foreground"
            >
              {space.category_name}
            </Link>
            <h1 className="text-4xl font-medium tracking-tight text-foreground lg:text-5xl">
              {title}
            </h1>
            <p className="text-muted-foreground">
              {[space.comuna_name, space.region_name]
                .filter(Boolean)
                .join(", ")}
              {space.area_m2 > 0 ? ` · ${space.area_m2} m²` : ""}
            </p>
          </header>
          <section
            aria-label="Tarifa publicada"
            className="rounded-2xl border border-border p-6 sm:p-8"
          >
            <div className="flex flex-wrap items-baseline gap-2">
              <span className="text-3xl font-semibold">
                {price ?? "Tarifa no disponible"}
              </span>
              {price ? (
                <span className="text-muted-foreground">
                  / {rateUnitLabel(space.rate_unit)}
                </span>
              ) : null}
            </div>
            {estimate ? (
              <p className="mt-3 text-sm text-muted-foreground">
                Estimación para el intervalo consultado: {estimate}.
              </p>
            ) : null}
            <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
              La tarifa y cualquier estimación no confirman disponibilidad y no
              constituyen cotización ni reserva.
            </p>
          </section>
          <section>
            <h2 className="text-lg font-medium">Sobre esta publicación</h2>
            <p className="mt-3 whitespace-pre-wrap leading-relaxed text-muted-foreground">
              {description || "La publicación no incluye una descripción."}
            </p>
          </section>
          {rules ? (
            <section>
              <h2 className="text-lg font-medium">Reglas de uso informadas</h2>
              <p className="mt-3 whitespace-pre-wrap leading-relaxed text-muted-foreground">
                {rules}
              </p>
            </section>
          ) : null}
          <p className="border-t border-border pt-5 text-xs leading-relaxed text-muted-foreground">
            La ficha muestra únicamente información pública minimizada. No
            incluye dirección exacta, datos del propietario, contacto privado,
            disponibilidad confirmada, reseñas ni acciones de reserva o pago.
          </p>
        </div>
      </div>
      {features.length ? (
        <ProductFeatures
          features={features.map((feature) => ({
            title: safePublicText(feature.label),
            description: "Característica declarada en la ficha pública.",
          }))}
        />
      ) : null}
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: jsonLd(placeJsonLd) }}
      />
    </article>
  );
}
