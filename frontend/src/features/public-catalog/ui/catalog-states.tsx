import Link from "next/link";

export function CatalogUnavailable({
  title = "No pudimos cargar el catálogo",
  retryHref = "/explorar",
}: {
  title?: string;
  retryHref?: string;
}) {
  return (
    <section
      role="status"
      className="border-y border-border py-16 sm:py-20"
      aria-labelledby="catalog-unavailable-title"
    >
      <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
        Servicio temporalmente no disponible
      </p>
      <h2
        id="catalog-unavailable-title"
        className="mt-3 max-w-2xl text-3xl font-medium tracking-tight sm:text-4xl"
      >
        {title}
      </h2>
      <p className="mt-4 max-w-xl text-muted-foreground">
        Inténtalo nuevamente más tarde. Este estado no significa que el catálogo
        esté vacío.
      </p>
      <Link
        href={retryHref}
        className="mt-7 inline-flex min-h-11 items-center rounded-full border border-border px-5 text-sm font-medium hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        Volver a intentar
      </Link>
    </section>
  );
}

export function EmptyCatalog({
  title,
  description,
  ownerCta = false,
}: {
  title: string;
  description: string;
  ownerCta?: boolean;
}) {
  return (
    <section className="border-y border-border py-16 sm:py-20">
      <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
        Sin publicaciones
      </p>
      <h2 className="mt-3 max-w-2xl text-3xl font-medium tracking-tight sm:text-4xl">
        {title}
      </h2>
      <p className="mt-4 max-w-xl text-muted-foreground">{description}</p>
      {ownerCta ? (
        <Link
          href="/anfitriones"
          className="mt-7 inline-flex min-h-11 items-center rounded-full bg-foreground px-5 text-sm font-medium text-background hover:bg-foreground/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          Conoce cómo ofrecer tu inmueble
        </Link>
      ) : null}
    </section>
  );
}
