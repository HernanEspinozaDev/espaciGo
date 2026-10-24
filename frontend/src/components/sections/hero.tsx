import { ArrowRightIcon, MapPin } from "lucide-react";
import Link from "next/link";

export function Hero() {
  return (
    <section className="relative overflow-hidden bg-secondary/30">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="py-16 sm:py-20 lg:py-28">
          <div className="max-w-2xl">
            <p className="mb-5 inline-flex items-center gap-2 text-sm font-medium text-muted-foreground">
              <MapPin className="h-4 w-4" aria-hidden="true" /> Espacios para
              trabajar, crear y reunirse
            </p>
            <h1 className="text-4xl font-medium tracking-tight text-foreground sm:text-5xl lg:text-6xl">
              Encuentra el lugar ideal para tu próxima idea
            </h1>
            <p className="mt-6 text-lg leading-relaxed text-muted-foreground sm:text-xl">
              Explora espacios de muestra para reuniones, talleres y proyectos
              en distintas ciudades. La disponibilidad y las tarifas reales aún
              no están conectadas.
            </p>
            <div className="mt-10 flex flex-col gap-4 sm:flex-row">
              <Link
                href="/products"
                className="inline-flex h-12 items-center justify-center gap-2 rounded-full bg-foreground px-8 text-base font-medium text-background transition-colors hover:bg-foreground/90"
              >
                Explorar espacios{" "}
                <ArrowRightIcon className="h-4 w-4" aria-hidden="true" />
              </Link>
              <Link
                href="#about"
                className="inline-flex h-12 items-center justify-center gap-2 rounded-full border border-border px-8 text-base font-medium transition-colors hover:bg-secondary"
              >
                Conocer EspaciGo
              </Link>
            </div>
          </div>
        </div>
      </div>
      <div
        aria-hidden="true"
        className="pointer-events-none absolute right-0 top-1/2 hidden h-full w-1/3 -translate-y-1/2 bg-linear-to-l from-secondary/50 to-transparent lg:block"
      />
    </section>
  );
}
