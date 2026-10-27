"use client";

import Link from "next/link";
import { useEffect, useSyncExternalStore } from "react";
import { AuthShell } from "./auth-shell";
import { useSessionIdentity } from "./use-session-identity";
import {
  cancelScheduledClearVerificationCompleted,
  clearVerificationCompleted,
  getVerificationResultSnapshot,
  getServerVerificationResultSnapshot,
  scheduleClearVerificationCompleted,
  subscribeVerificationResult,
} from "../application/verification-result";

export function EmailVerifiedScreen() {
  const isVerified = useSyncExternalStore(
    subscribeVerificationResult,
    getVerificationResultSnapshot,
    getServerVerificationResultSnapshot,
  );
  const session = useSessionIdentity();
  const isAuthenticated = session.status === "authenticated";

  useEffect(() => {
    cancelScheduledClearVerificationCompleted();
    const clearOnHistoryExit = () => {
      if (window.location.pathname !== "/correo-verificado") {
        clearVerificationCompleted();
      }
    };
    window.addEventListener("popstate", clearOnHistoryExit);
    window.addEventListener("pagehide", clearVerificationCompleted);
    return () => {
      window.removeEventListener("popstate", clearOnHistoryExit);
      window.removeEventListener("pagehide", clearVerificationCompleted);
      scheduleClearVerificationCompleted();
    };
  }, []);

  if (!isVerified) {
    return (
      <AuthShell
        title="Verificación de correo"
        description="Abre el enlace que enviamos a tu correo para activar tu cuenta."
        secondaryQuestion="¿Necesitas ayuda?"
        secondaryLabel="Ir a acceso"
        secondaryHref="/acceso"
      >
        <div className="space-y-6">
          <p className="text-sm leading-relaxed text-gray-700">
            No encontramos una confirmación reciente en esta sesión. Si acabas
            de registrarte o solicitaste un nuevo enlace, abre el mensaje
            enviado a tu casilla de correo para continuar.
          </p>
          <div className="space-y-3">
            <Link
              href="/verificar-correo"
              className="inline-flex min-h-11 w-full items-center justify-center rounded-full bg-orange-700 px-4 py-2 text-center text-sm font-semibold text-white shadow-xs hover:bg-orange-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
            >
              Solicitar un nuevo enlace
            </Link>
            <Link
              href="/acceso"
              className="inline-flex min-h-11 w-full items-center justify-center rounded-full border border-gray-300 bg-white px-4 py-2 text-center text-sm font-medium text-gray-700 shadow-xs hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
            >
              Ir a acceso
            </Link>
          </div>
        </div>
      </AuthShell>
    );
  }

  const ctaHref = isAuthenticated ? "/mi-cuenta" : "/acceso";
  const ctaLabel = isAuthenticated ? "Continuar a Mi cuenta" : "Iniciar sesión";

  return (
    <AuthShell
      title="Correo verificado"
      onNavigateAway={clearVerificationCompleted}
      description="Tu cuenta de EspaciGo ya está lista. Inicia sesión para continuar."
      secondaryQuestion="¿Todo listo?"
      secondaryLabel="Volver al inicio"
      secondaryHref="/"
    >
      <div className="space-y-6">
        <p className="text-sm leading-relaxed text-gray-700">
          Tu dirección de correo electrónico ha sido confirmada con éxito.
          {isAuthenticated
            ? " Tu sesión está activa. Ya puedes continuar a tu cuenta."
            : " Inicia sesión para comenzar a usar tu cuenta en EspaciGo."}
        </p>
        <Link
          href={ctaHref}
          onNavigate={clearVerificationCompleted}
          className="inline-flex min-h-11 w-full items-center justify-center rounded-full bg-orange-700 px-4 py-2 text-center text-sm font-semibold text-white shadow-xs hover:bg-orange-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
        >
          {ctaLabel}
        </Link>
      </div>
    </AuthShell>
  );
}
