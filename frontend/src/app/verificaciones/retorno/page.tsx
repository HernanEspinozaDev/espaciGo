"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  getDiditIntent,
  type DiditStatusResponse,
} from "@/features/verifications/verification-api";
import { VerificationBadge } from "@/features/verifications/verification-badge";

function VerificationReturnContent() {
  const params = useSearchParams();
  const [result, setResult] = useState<DiditStatusResponse | null>(null);
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(true);
  const attemptId = params.get("attempt_id");
  const sessionId = params.get("session_id");

  useEffect(() => {
    // Remove provider tokens before any network request; never treat them as authorization.
    window.history.replaceState(
      window.history.state,
      "",
      window.location.pathname,
    );
    let cancelled = false;
    async function poll() {
      if (!attemptId || !sessionId) {
        setError(true);
        setLoading(false);
        return;
      }
      for (let count = 0; count < 10 && !cancelled; count += 1) {
        try {
          const status = await getDiditIntent(attemptId);
          if (cancelled) return;
          setResult(status);
          if (
            ["verified", "rejected", "in_review"].includes(status.badge_status)
          ) {
            setLoading(false);
            return;
          }
        } catch {
          if (count === 9 && !cancelled) setError(true);
        }
        if (count < 9)
          await new Promise((resolve) =>
            window.setTimeout(resolve, 2000 + count * 1000),
          );
      }
      if (!cancelled) setLoading(false);
    }
    void poll();
    return () => {
      cancelled = true;
    };
  }, [attemptId, sessionId]);

  return (
    <section className="mx-auto max-w-xl space-y-5">
      <h1 className="text-2xl font-semibold">Resultado de identidad</h1>
      {loading ? (
        <p role="status">Estamos comprobando el resultado de forma segura…</p>
      ) : null}
      {result ? (
        <div className="space-y-3">
          <VerificationBadge status={result.badge_status} />
          {result.badge_status === "rejected" ? (
            <p>
              No pudimos verificar la identidad. Puedes revisar tus datos e
              intentarlo nuevamente.
            </p>
          ) : null}
          {result.badge_status === "in_review" ? (
            <p>
              Tu información requiere una revisión adicional. La identidad aún
              no está verificada.
            </p>
          ) : null}
        </div>
      ) : null}
      {error ? (
        <p role="alert" className="text-red-800">
          No fue posible confirmar el resultado. Puedes volver a tu cuenta e
          intentarlo más tarde.
        </p>
      ) : null}
      <div className="flex gap-3">
        <Link
          className="rounded-md border border-border px-4 py-2"
          href="/dashboard"
        >
          Volver al panel
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

export default function VerificationReturnPage() {
  return (
    <Suspense
      fallback={
        <section className="mx-auto max-w-xl space-y-5">
          <h1 className="text-2xl font-semibold">Resultado de identidad</h1>
          <p role="status">Estamos comprobando el resultado de forma segura…</p>
        </section>
      }
    >
      <VerificationReturnContent />
    </Suspense>
  );
}
