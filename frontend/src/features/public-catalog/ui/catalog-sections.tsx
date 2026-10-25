import Link from "next/link";
import type { CatalogQuery, PublicSpace } from "../domain/types";
import { PublicCatalogCard } from "./public-catalog-card";
import { NextCatalogPage } from "./next-catalog-page";
import { EmptyCatalog } from "./catalog-states";

export function PublicCatalogGrid({
  spaces,
  query,
  nextCursor,
  actionPath = "/explorar",
  emptyTitle = "Todavía no hay publicaciones que mostrar",
  emptyDescription = "Prueba con otra búsqueda o vuelve más tarde.",
  ownerCta = false,
}: {
  spaces: readonly PublicSpace[];
  query: CatalogQuery;
  nextCursor?: string;
  actionPath?: string;
  emptyTitle?: string;
  emptyDescription?: string;
  ownerCta?: boolean;
}) {
  if (spaces.length === 0) {
    return (
      <EmptyCatalog
        title={emptyTitle}
        description={emptyDescription}
        ownerCta={ownerCta}
      />
    );
  }
  return (
    <>
      <div className="grid grid-cols-1 gap-x-6 gap-y-10 sm:grid-cols-2 sm:gap-x-8 lg:grid-cols-3 lg:gap-y-14">
        {spaces.map((space, index) => (
          <PublicCatalogCard
            key={space.space_id}
            space={space}
            priority={index === 0}
          />
        ))}
      </div>
      <NextCatalogPage
        query={query}
        nextCursor={nextCursor}
        href={actionPath}
      />
    </>
  );
}

export function OwnerCallout({
  title = "¿Tienes una casa de veraneo? Publícala y arriéndala en EspaciGo.",
  description = "También puedes ofrecer una casa, departamento, habitación, cabaña o un espacio flexible. Crear y publicar anuncios todavía no está habilitado en esta versión.",
}: {
  title?: string;
  description?: string;
}) {
  return (
    <section className="bg-foreground text-background">
      <div className="mx-auto flex max-w-7xl flex-col gap-7 px-4 py-16 sm:px-6 sm:py-20 lg:flex-row lg:items-end lg:justify-between lg:px-8">
        <div className="max-w-3xl">
          <p className="text-xs uppercase tracking-[0.18em] text-background/65">
            Para propietarios y anfitriones
          </p>
          <h2 className="mt-3 text-3xl font-medium tracking-tight sm:text-4xl">
            {title}
          </h2>
          <p className="mt-4 max-w-2xl leading-relaxed text-background/75">
            {description}
          </p>
        </div>
        <Link
          href="/anfitriones"
          className="inline-flex min-h-12 shrink-0 items-center justify-center rounded-full bg-background px-6 text-sm font-medium text-foreground transition-colors hover:bg-background/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-background"
        >
          Conoce cómo ofrecer tu espacio
        </Link>
      </div>
    </section>
  );
}
