import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { ProductCard } from "@/components/product-card";
import { getSpaces } from "@/lib/marketplace-fixtures";

type ProductGridProps = {
  title?: string;
  description?: string;
  limit?: number;
  showViewAll?: boolean;
  viewAllHref?: string;
};

export function ProductGrid({
  title = "Espacios destacados",
  description = "Una selección de espacios de muestra para inspirar tu próximo proyecto.",
  limit = 6,
  showViewAll = true,
  viewAllHref = "/products",
}: ProductGridProps) {
  const { data: spaces } = getSpaces();
  const displayedSpaces = spaces.slice(0, limit);

  return (
    <section
      id="products"
      className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-24 lg:px-8"
    >
      <div className="mb-12 flex items-end justify-between gap-6">
        <div>
          <h2 className="text-2xl font-medium text-foreground sm:text-3xl">
            {title}
          </h2>
          <p className="mt-2 text-muted-foreground">{description}</p>
        </div>
        {showViewAll && (
          <Link
            href={viewAllHref}
            className="hidden items-center gap-1 whitespace-nowrap text-sm font-medium text-muted-foreground transition-colors hover:text-foreground sm:inline-flex"
          >
            Ver todos <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        )}
      </div>
      <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
        {displayedSpaces.map((space, index) => (
          <ProductCard key={space.id} space={space} priority={index === 0} />
        ))}
      </div>
      {showViewAll && (
        <div className="mt-12 text-center sm:hidden">
          <Link
            href={viewAllHref}
            className="inline-flex min-h-11 items-center gap-1 text-sm font-medium text-muted-foreground hover:text-foreground"
          >
            Ver todos los espacios{" "}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </div>
      )}
    </section>
  );
}
