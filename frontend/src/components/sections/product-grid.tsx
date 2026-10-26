import Link from "next/link";
import type { PublicSpace } from "@/features/public-catalog/domain/types";
import { PublicCatalogCard } from "@/features/public-catalog/ui/public-catalog-card";

export type ProductGridProps = {
  title?: string;
  description?: string;
  spaces: readonly PublicSpace[];
  showViewAll?: boolean;
  viewAllHref?: string;
};

export function ProductGrid({
  title = "Publicaciones del catálogo",
  description = "Publicaciones que la API pública de EspaciGo entrega como elegibles.",
  spaces,
  showViewAll = true,
  viewAllHref = "/explorar",
}: ProductGridProps) {
  return (
    <section className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-24 lg:px-8">
      <div className="mb-10 flex items-end justify-between gap-6">
        <div>
          <h2 className="text-2xl font-medium text-foreground sm:text-3xl">
            {title}
          </h2>
          <p className="mt-2 max-w-2xl text-muted-foreground">{description}</p>
        </div>
        {showViewAll ? (
          <Link
            href={viewAllHref}
            className="hidden min-h-11 items-center gap-1 whitespace-nowrap text-sm font-medium text-muted-foreground transition-colors hover:text-foreground sm:inline-flex"
          >
            Ver catálogo <span aria-hidden="true">→</span>
          </Link>
        ) : null}
      </div>
      {spaces.length ? (
        <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
          {spaces.map((space, index) => (
            <PublicCatalogCard
              key={space.space_id}
              space={space}
              priority={index === 0}
            />
          ))}
        </div>
      ) : (
        <p className="border-t border-border py-10 text-muted-foreground">
          No hay publicaciones públicas en esta selección.
        </p>
      )}
      {showViewAll ? (
        <div className="mt-8 text-center sm:hidden">
          <Link
            href={viewAllHref}
            className="inline-flex min-h-11 items-center gap-1 text-sm font-medium hover:underline"
          >
            Ver catálogo <span aria-hidden="true">→</span>
          </Link>
        </div>
      ) : null}
    </section>
  );
}
