import { PackageSearchIcon } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Espacio no encontrado",
  robots: { index: false, follow: true },
};

export default function ProductNotFound() {
  return (
    <div className="flex min-h-[60dvh] flex-col items-center justify-center px-4 py-24 text-center">
      <PackageSearchIcon
        className="size-16 text-muted-foreground/50"
        strokeWidth={1.5}
      />
      <h1 className="mt-6 text-2xl font-medium tracking-tight">
        Espacio no encontrado
      </h1>
      <p className="mt-2 max-w-md text-sm text-muted-foreground">
        Este espacio no existe o ya no está disponible en este catálogo de
        muestra.
      </p>
      <div className="mt-8 flex items-center gap-4">
        <Link
          href="/products"
          className="inline-flex items-center rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background transition-colors hover:bg-foreground/90"
        >
          Explorar espacios
        </Link>
        <Link
          href="/"
          className="inline-flex items-center rounded-md border border-border px-4 py-2 text-sm font-medium transition-colors hover:bg-secondary"
        >
          Volver al inicio
        </Link>
      </div>
    </div>
  );
}
