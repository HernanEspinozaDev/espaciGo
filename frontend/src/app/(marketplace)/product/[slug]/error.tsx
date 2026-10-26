"use client";

import { RouteErrorFallback } from "@/components/route-error";

export default function ProductError({
  error,
  retry,
}: {
  error: Error & { digest?: string };
  retry: () => void;
}) {
  return (
    <RouteErrorFallback
      title="No pudimos cargar este espacio"
      description="Puede ser un problema temporal. Inténtalo de nuevo o sigue explorando el catálogo."
      error={error}
      retry={retry}
    />
  );
}
