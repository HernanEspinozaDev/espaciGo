import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { PublicSearchForm } from "@/features/public-catalog/ui/public-search-form";

export function Hero() {
  return (
    <section className="relative overflow-hidden bg-secondary/30">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="py-16 sm:py-20 lg:py-28">
          <div className="max-w-4xl">
            <p className="mb-5 inline-flex items-center gap-2 text-sm font-medium text-muted-foreground">
              Espacios para vivir, trabajar y hacer más
            </p>
            <h1 className="max-w-[18ch] text-4xl font-medium tracking-tight text-foreground sm:text-5xl lg:text-7xl">
              Arrienda el espacio que necesitas. Ofrece el espacio que tienes.
            </h1>
            <p className="mt-6 max-w-2xl text-lg leading-relaxed text-muted-foreground sm:text-xl">
              Encuentra publicaciones para vivir, trabajar, guardar, reunirte o
              celebrar. Casas, departamentos y espacios flexibles aparecen
              cuando el catálogo público los ofrece.
            </p>
            <PublicSearchForm />
            <div className="mt-7 flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-6">
              <Link
                href="/arriendos"
                className="inline-flex min-h-11 items-center gap-2 text-sm font-medium text-foreground underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                Arrienda una casa{" "}
                <ArrowRight className="size-4" aria-hidden="true" />
              </Link>
              <Link
                href="/anfitriones"
                className="inline-flex min-h-11 items-center text-sm font-medium text-muted-foreground underline-offset-4 hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                Conoce cómo ofrecer tu espacio
              </Link>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
