import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Cómo funciona EspaciGo",
  description:
    "Explora publicaciones públicas de EspaciGo y entiende qué significan las tarifas y estimaciones disponibles.",
  alternates: { canonical: "/como-funciona" },
};

const steps = [
  {
    title: "Explora el catálogo público",
    text: "Busca por categoría, ubicación y filtros disponibles. Solo se muestran publicaciones que la API pública de EspaciGo entrega como elegibles.",
  },
  {
    title: "Revisa la información publicada",
    text: "Cada ficha presenta los datos públicos minimizados que entrega EspaciGo. No expone contacto privado, ubicación exacta, disponibilidad confirmada ni reseñas.",
  },
  {
    title: "Interpreta las tarifas con cuidado",
    text: "La tarifa base corresponde a la unidad informada por cada publicación. Una estimación para un intervalo no confirma disponibilidad y no constituye cotización ni reserva.",
  },
];

export default function HowItWorksPage() {
  return (
    <article className="mx-auto max-w-6xl px-4 py-12 sm:px-6 sm:py-20 lg:px-8">
      <nav
        aria-label="Breadcrumb"
        className="mb-8 text-sm text-muted-foreground"
      >
        <Link href="/" className="hover:text-foreground">
          Inicio
        </Link>
        <span aria-hidden="true"> / </span>
        <span aria-current="page">Cómo funciona</span>
      </nav>
      <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
        Exploración pública
      </p>
      <h1 className="mt-4 max-w-4xl text-4xl font-medium tracking-tight sm:text-6xl">
        Encuentra un espacio para vivir, trabajar, guardar, reunirte o celebrar.
      </h1>
      <p className="mt-6 max-w-3xl text-lg leading-relaxed text-muted-foreground">
        EspaciGo reúne publicaciones residenciales y de espacios flexibles
        cuando están habilitadas para consulta pública. La creación de anuncios
        y el flujo de reserva no están disponibles en esta versión.
      </p>
      <ol className="mt-12 grid gap-5 md:grid-cols-3">
        {steps.map((step, index) => (
          <li
            key={step.title}
            className="rounded-2xl border border-border p-6 sm:p-8"
          >
            <p className="text-sm tabular-nums text-muted-foreground">
              {String(index + 1).padStart(2, "0")}
            </p>
            <h2 className="mt-8 text-xl font-medium">{step.title}</h2>
            <p className="mt-3 leading-relaxed text-muted-foreground">
              {step.text}
            </p>
          </li>
        ))}
      </ol>
      <div className="mt-10 flex flex-wrap gap-3">
        <Link
          href="/explorar"
          className="inline-flex min-h-12 items-center rounded-full bg-foreground px-6 text-sm font-medium text-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        >
          Explorar arriendos
        </Link>
        <Link
          href="/anfitriones"
          className="inline-flex min-h-12 items-center rounded-full border border-border px-6 text-sm font-medium hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        >
          Conoce cómo ofrecer tu espacio
        </Link>
      </div>
    </article>
  );
}
