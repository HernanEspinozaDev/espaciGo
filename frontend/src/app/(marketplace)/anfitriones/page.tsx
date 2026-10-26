import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Para propietarios y anfitriones",
  description:
    "Conoce el propósito de EspaciGo para ofrecer espacios y arriendos residenciales. La creación y publicación de anuncios aún no está habilitada.",
  alternates: { canonical: "/anfitriones" },
};

export default function HostsPage() {
  return (
    <article className="mx-auto max-w-5xl px-4 py-12 sm:px-6 sm:py-20 lg:px-8">
      <nav
        aria-label="Breadcrumb"
        className="mb-8 text-sm text-muted-foreground"
      >
        <Link href="/" className="hover:text-foreground">
          Inicio
        </Link>
        <span aria-hidden="true"> / </span>
        <span aria-current="page">Para anfitriones</span>
      </nav>
      <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
        Para propietarios y anfitriones
      </p>
      <h1 className="mt-4 max-w-4xl text-4xl font-medium tracking-tight sm:text-6xl">
        ¿Tienes una casa de veraneo? Publícala y arriéndala en EspaciGo.
      </h1>
      <p className="mt-6 max-w-3xl text-lg leading-relaxed text-muted-foreground">
        También queremos facilitar que las personas ofrezcan casas,
        departamentos, habitaciones, cabañas y otros espacios. En esta versión,
        crear y publicar anuncios todavía no está habilitado.
      </p>
      <div className="mt-10 rounded-2xl border border-border bg-secondary/30 p-6 sm:p-8">
        <h2 className="text-2xl font-medium">Qué puedes hacer ahora</h2>
        <p className="mt-3 max-w-2xl leading-relaxed text-muted-foreground">
          Puedes revisar el catálogo público e ingresar a EspaciGo. El ingreso
          no crea una publicación ni inicia una solicitud de arriendo.
        </p>
        <div className="mt-6 flex flex-wrap gap-3">
          <Link
            href="/acceso"
            className="inline-flex min-h-12 items-center rounded-full bg-foreground px-6 text-sm font-medium text-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            Ingresar a EspaciGo
          </Link>
          <Link
            href="/explorar"
            className="inline-flex min-h-12 items-center rounded-full border border-border px-6 text-sm font-medium hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            Explorar arriendos
          </Link>
        </div>
      </div>
      <p className="mt-8 text-sm text-muted-foreground">
        No hay formulario de captación o lista de espera disponible en esta
        versión.
      </p>
    </article>
  );
}
