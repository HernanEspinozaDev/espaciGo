"use client";

import { useEffect } from "react";
import Link from "next/link";

export default function VerificationReturnPage() {
  useEffect(() => {
    // Remove legacy callback parameters; they are not used for authorization or lookup.
    window.history.replaceState(
      window.history.state,
      "",
      window.location.pathname,
    );
  }, []);

  return (
    <section className="mx-auto max-w-xl space-y-5">
      <h1 className="text-2xl font-semibold">Este retorno ya no está activo</h1>
      <p role="status">
        El flujo de verificación anterior dejó de estar disponible. La revisión
        documental propia aún no está habilitada; puedes consultar tu
        declaración guardada en Mi cuenta.
      </p>
      <div className="flex gap-3">
        <Link
          className="rounded-md border border-border px-4 py-2"
          href="/mi-cuenta"
        >
          Volver a Mi cuenta
        </Link>
        <Link
          className="rounded-md bg-primary px-4 py-2 text-primary-foreground"
          href="/mi-cuenta/verificaciones"
        >
          Revisar o reintentar
        </Link>
      </div>
    </section>
  );
}
