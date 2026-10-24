import { ArrowRightIcon, BellRing } from "lucide-react";

export function Newsletter() {
  return (
    <section className="overflow-hidden bg-foreground text-background">
      <div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-2xl text-center">
          <div className="mx-auto mb-6 flex h-12 w-12 items-center justify-center rounded-full bg-background/10">
            <BellRing className="h-6 w-6" aria-hidden="true" />
          </div>
          <h2 className="text-3xl font-medium tracking-tight sm:text-4xl lg:text-5xl">
            Mantente al día
          </h2>
          <p className="mx-auto mt-4 max-w-md text-lg leading-relaxed text-background/70">
            Más adelante podrás recibir novedades sobre nuevos espacios y
            funcionalidades.
          </p>
          <div className="mx-auto mt-10 flex max-w-md flex-col gap-4 sm:flex-row">
            <input
              aria-label="Correo electrónico"
              disabled
              placeholder="Tu correo electrónico"
              className="h-12 w-full flex-1 rounded-full border border-background/20 bg-background/10 px-5 text-background placeholder:text-background/50 disabled:cursor-not-allowed"
            />
            <button
              type="button"
              disabled
              className="inline-flex h-12 shrink-0 items-center justify-center gap-2 rounded-full bg-background px-8 font-medium text-foreground opacity-60"
            >
              Próximamente{" "}
              <ArrowRightIcon className="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
          <p className="mt-4 text-xs text-background/60">
            Demostración visual; no se recopilan datos.
          </p>
        </div>
      </div>
    </section>
  );
}
