"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { AlertCircle, ArrowRight, RotateCcw } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useDocumentVerification } from "@/features/document-verification/ui/use-document-verification";
import type { DocumentVerificationCase } from "@/features/document-verification/domain/document-verification";
import { isAbortError } from "@/lib/api/errors";

type IdentityStatusView =
  | { readonly kind: "loading" }
  | { readonly kind: "error" }
  | {
      readonly kind: "ready";
      readonly identityState: DocumentVerificationCase["state"] | null;
      readonly decisionMessage: string | null;
    };

const identityCaseMessages: Record<DocumentVerificationCase["state"], string> =
  {
    borrador: "Tu solicitud de identidad está en borrador.",
    enviada: "Tu solicitud fue enviada para revisión.",
    en_revision: "Tu solicitud está en revisión documental.",
    requiere_antecedentes:
      "Necesitamos que corrijas o completes tus documentos.",
    aprobada:
      "La revisión documental quedó aprobada. La vigencia efectiva la determina el servidor.",
    rechazada:
      "La solicitud fue rechazada. Consulta los detalles en documentos.",
    vencida: "La solicitud venció. Consulta los pasos para continuar.",
    revocada:
      "La revisión fue revocada. La vigencia efectiva la determina el servidor.",
  };

interface AccountVerificationCardProps {
  readonly showDeclarationLink?: boolean;
}

export function AccountVerificationCard({
  showDeclarationLink = true,
}: AccountVerificationCardProps) {
  const { snapshot: authSnapshot } = useAuth();

  if (authSnapshot.status !== "authenticated") {
    return null;
  }

  const sessionKey = JSON.stringify([
    authSnapshot.status,
    authSnapshot.accountId,
    [...authSnapshot.roles].sort(),
    [...authSnapshot.capabilities].sort(),
    authSnapshot.restrictedMode,
    authSnapshot.expiresAt,
  ]);

  return (
    <AuthenticatedAccountVerificationCard
      key={sessionKey}
      showDeclarationLink={showDeclarationLink}
    />
  );
}

function AuthenticatedAccountVerificationCard({
  showDeclarationLink,
}: {
  readonly showDeclarationLink: boolean;
}) {
  const { loadCases } = useDocumentVerification();
  const [status, setStatus] = useState<IdentityStatusView>({ kind: "loading" });
  const [reloadToken, setReloadToken] = useState(0);
  const reload = useCallback(() => {
    setStatus({ kind: "loading" });
    setReloadToken((value) => value + 1);
  }, []);

  useEffect(() => {
    window.addEventListener("focus", reload);
    return () => window.removeEventListener("focus", reload);
  }, [reload]);

  useEffect(() => {
    let cancelled = false;
    void loadCases()
      .then((cases) => {
        if (cancelled) return;
        const identityCase = cases.find((item) => item.purpose === "identity");
        const identityState = identityCase?.state ?? null;
        setStatus({
          kind: "ready",
          // The owner-cases endpoint is ordered by most recently updated first.
          identityState,
          decisionMessage:
            identityState === "requiere_antecedentes"
              ? (identityCase?.owner_decision_message ?? null)
              : null,
        });
      })
      .catch((error: unknown) => {
        if (!cancelled && !isAbortError(error)) setStatus({ kind: "error" });
      });
    return () => {
      cancelled = true;
    };
  }, [loadCases, reloadToken]);

  return (
    <Card
      aria-labelledby="account-verification-title"
      className="border-amber-200/80 bg-amber-50/30 dark:border-amber-900/50 dark:bg-amber-950/15"
    >
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <CardTitle
              id="account-verification-title"
              className="text-base sm:text-lg"
            >
              Verificación de la cuenta
            </CardTitle>
          </div>
        </div>
        <CardDescription className="text-sm text-foreground/80 mt-1">
          Consulta el estado de tu revisión documental propia. La declaración
          legal y la revisión son pasos separados.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {status.kind === "loading" && (
            <p role="status" className="text-sm text-muted-foreground">
              Consultando el estado de la revisión documental…
            </p>
          )}
          {status.kind === "error" && (
            <div className="flex flex-wrap items-center justify-between gap-3">
              <p
                role="alert"
                className="flex items-center gap-2 text-sm text-destructive"
              >
                <AlertCircle className="size-4 shrink-0" aria-hidden="true" />
                No pudimos consultar el estado de tu revisión documental.
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={reload}
                className="gap-1.5"
              >
                <RotateCcw className="size-3.5" aria-hidden="true" />
                Reintentar consulta
              </Button>
            </div>
          )}
          {status.kind === "ready" && !status.identityState && (
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
              <p role="status" className="text-sm text-muted-foreground">
                Aún no has iniciado una revisión documental propia.
              </p>
              <Button asChild size="sm" variant="outline" className="gap-1.5">
                <Link href="/mi-cuenta/documentos">
                  Iniciar verificación documental
                  <ArrowRight className="size-4" aria-hidden="true" />
                </Link>
              </Button>
            </div>
          )}
          {status.kind === "ready" && status.identityState && (
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
              <p role="status" className="text-sm text-muted-foreground">
                {status.decisionMessage ??
                  identityCaseMessages[status.identityState]}
              </p>
              <Button asChild size="sm" variant="outline" className="gap-1.5">
                <Link href="/mi-cuenta/documentos">
                  {status.identityState === "requiere_antecedentes"
                    ? "Corregir documentos"
                    : "Ver estado de solicitud"}
                  <ArrowRight className="size-4" aria-hidden="true" />
                </Link>
              </Button>
            </div>
          )}
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <p className="text-xs text-muted-foreground">
              La vigencia efectiva se determina en el servidor; esta tarjeta no
              reemplaza los requisitos de cada operación.
            </p>
            {showDeclarationLink && (
              <Button
                asChild
                size="sm"
                className="shrink-0 gap-1.5 font-medium"
              >
                <Link href="/mi-cuenta/verificaciones">
                  Ver declaración
                  <ArrowRight className="size-4" aria-hidden="true" />
                </Link>
              </Button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
