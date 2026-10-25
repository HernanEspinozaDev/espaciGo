import Link from "next/link";
import type { TermsResponse } from "../domain/types";
import { publicCatalog } from "../infrastructure/catalog-runtime";
import { CatalogUnavailable } from "./catalog-states";

export async function LegalDocument({
  type,
}: {
  type: "terminos" | "privacidad";
}) {
  const result = await publicCatalog.terms();
  if (!result.ok) {
    return (
      <section className="mx-auto max-w-4xl px-4 py-12 sm:px-6 sm:py-20 lg:px-8">
        <nav
          aria-label="Breadcrumb"
          className="mb-7 text-sm text-muted-foreground"
        >
          <Link href="/" className="hover:text-foreground">
            Inicio
          </Link>
          <span aria-hidden="true"> / </span>
          <span aria-current="page">
            {type === "terminos"
              ? "Términos y condiciones"
              : "Política de privacidad"}
          </span>
        </nav>
        <CatalogUnavailable
          title={
            result.error.status === 503
              ? "Los documentos legales no están disponibles en este momento"
              : "No pudimos cargar el documento legal"
          }
        />
      </section>
    );
  }
  const item = result.value.items.find((entry) => entry.type === type);
  if (!item) {
    return (
      <section className="mx-auto max-w-4xl px-4 py-12 sm:px-6 sm:py-20 lg:px-8">
        <h1 className="text-3xl font-medium">Documento no disponible</h1>
        <p className="mt-4 text-muted-foreground">
          Go no entregó este documento en la respuesta pública.
        </p>
      </section>
    );
  }
  const document = item.document;
  if (
    !document ||
    typeof document.title !== "string" ||
    !Array.isArray(document.sections)
  ) {
    return (
      <section className="mx-auto max-w-4xl px-4 py-12 sm:px-6 sm:py-20 lg:px-8">
        <h1 className="text-3xl font-medium">Contenido legal no disponible</h1>
        <p className="mt-4 text-muted-foreground">
          Go confirmó el documento, pero esta respuesta no incluye su contenido.
          No se mostrará texto legal sustituto.
        </p>
      </section>
    );
  }

  return (
    <article className="mx-auto max-w-4xl px-4 py-12 sm:px-6 sm:py-20 lg:px-8">
      <nav
        aria-label="Breadcrumb"
        className="mb-7 text-sm text-muted-foreground"
      >
        <Link href="/" className="hover:text-foreground">
          Inicio
        </Link>
        <span aria-hidden="true"> / </span>
        <span aria-current="page">{document.title}</span>
      </nav>
      {item.synthetic ? (
        <p
          role="note"
          className="mb-6 rounded-xl border border-border bg-secondary/40 p-4 text-sm text-muted-foreground"
        >
          Documento sintético de ensayo local; no es texto legal productivo.
        </p>
      ) : null}
      <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
        {document.scope} · {document.status}
      </p>
      <h1 className="mt-3 text-4xl font-medium tracking-tight sm:text-5xl">
        {document.title}
      </h1>
      <p className="mt-4 text-sm text-muted-foreground">
        Versión informada por Go: {item.code}
      </p>
      <div className="mt-10 space-y-9 border-t border-border pt-8">
        {document.sections.map((section, index) => (
          <section key={`${index}-${section.heading}`}>
            <h2 className="text-xl font-medium">{section.heading}</h2>
            <p className="mt-3 whitespace-pre-wrap leading-relaxed text-muted-foreground">
              {section.body}
            </p>
          </section>
        ))}
      </div>
    </article>
  );
}

export type LocalLegalDocument = TermsResponse["items"][number];
