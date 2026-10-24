import Link from "next/link";
import { spaceCategories } from "@/lib/marketplace-fixtures";

export function Footer() {
  const year = new Date().getFullYear();

  return (
    <footer className="border-t border-border bg-background">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col gap-8 py-12 sm:flex-row sm:gap-16 sm:py-16">
          <div className="sm:max-w-xs">
            <Link href="/" className="text-xl font-bold text-foreground">
              EspaciGo
            </Link>
            <p className="mt-4 text-sm leading-relaxed text-muted-foreground">
              Encuentra espacios para trabajar, crear y reunirse. Catálogo local
              de demostración.
            </p>
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">
              Categorías
            </h2>
            <ul className="mt-4 space-y-3">
              {spaceCategories.map((category) => (
                <li key={category.slug}>
                  <Link
                    href={`/products?category=${category.slug}`}
                    className="text-sm text-muted-foreground transition-colors hover:text-foreground"
                  >
                    {category.name}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">Explora</h2>
            <ul className="mt-4 space-y-3">
              <li>
                <Link
                  href="/products"
                  className="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                  Todos los espacios
                </Link>
              </li>
              <li>
                <Link
                  href="/search"
                  className="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                  Buscar
                </Link>
              </li>
              <li>
                <Link
                  href="/saas"
                  className="text-sm text-muted-foreground transition-colors hover:text-foreground"
                >
                  Vista de aplicación
                </Link>
              </li>
            </ul>
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">
              Información
            </h2>
            <p className="mt-4 max-w-xs text-sm leading-relaxed text-muted-foreground">
              Las fichas, imágenes y tarifas son sintéticas. No se ofrecen
              reservas ni pagos desde esta demo.
            </p>
          </div>
        </div>
        <div className="flex flex-col gap-4 border-t border-border py-6 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-muted-foreground">
            © {year} EspaciGo · demostración local
          </p>
          <Link
            href="/pricing"
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            Planes ilustrativos
          </Link>
        </div>
      </div>
    </footer>
  );
}
