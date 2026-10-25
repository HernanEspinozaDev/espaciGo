import Link from "next/link";

export function HowItWorksPreview() {
  const steps = [
    {
      number: "01",
      title: "Explora",
      description: "Busca por categoría, comuna y características publicadas.",
    },
    {
      number: "02",
      title: "Revisa",
      description:
        "Consulta la información pública, las reglas y la tarifa declarada.",
    },
    {
      number: "03",
      title: "Compara",
      description:
        "Una estimación de tarifa no confirma disponibilidad ni reserva el espacio.",
    },
  ];
  return (
    <section className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-24 lg:px-8">
      <div className="max-w-2xl">
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          Una búsqueda clara
        </p>
        <h2 className="mt-3 text-3xl font-medium tracking-tight sm:text-4xl">
          Explora antes de decidir
        </h2>
        <p className="mt-5 text-muted-foreground">
          EspaciGo reúne categorías residenciales y espacios flexibles. La
          tarifa y modalidad dependen de cada publicación.
        </p>
      </div>
      <ol className="mt-10 grid gap-5 md:grid-cols-3">
        {steps.map((step) => (
          <li key={step.number} className="border-t border-border pt-5">
            <span className="text-xs tabular-nums text-muted-foreground">
              {step.number}
            </span>
            <h3 className="mt-5 text-xl font-medium">{step.title}</h3>
            <p className="mt-2 leading-relaxed text-muted-foreground">
              {step.description}
            </p>
          </li>
        ))}
      </ol>
      <Link
        href="/como-funciona"
        className="mt-8 inline-flex min-h-11 items-center border-b border-foreground/40 text-sm font-medium hover:border-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        Cómo funciona EspaciGo
      </Link>
    </section>
  );
}
