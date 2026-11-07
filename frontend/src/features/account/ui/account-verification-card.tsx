"use client";

import { useEffect } from "react";
import Link from "next/link";
import {
  ShieldAlert,
  ShieldCheck,
  Clock,
  ArrowRight,
  Loader2,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useVerifications } from "@/features/verifications/ui/use-verifications";

export function AccountVerificationCard() {
  const { snapshot: authSnapshot } = useAuth();
  const { snapshot, loadOverview } = useVerifications();

  useEffect(() => {
    if (authSnapshot.status === "authenticated") {
      void loadOverview().catch(() => undefined);
    }
  }, [authSnapshot.status, loadOverview]);

  if (authSnapshot.status !== "authenticated") {
    return null;
  }

  const kycCase = snapshot.verifications.find((v) => v.type === "kyc");
  const isVerified = kycCase?.state === "aprobada";
  const isPending = kycCase?.state === "en_revision";
  const isRejected = kycCase?.state === "rechazada";

  return (
    <Card
      aria-labelledby="account-verification-title"
      className={
        isVerified
          ? "border-emerald-200 bg-emerald-50/20 dark:border-emerald-900/40 dark:bg-emerald-950/10"
          : isPending
            ? "border-amber-200 bg-amber-50/20 dark:border-amber-900/40 dark:bg-amber-950/10"
            : "border-amber-200/80 bg-amber-50/30 dark:border-amber-900/50 dark:bg-amber-950/15"
      }
    >
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            {isVerified ? (
              <ShieldCheck
                className="size-5 text-emerald-800 dark:text-emerald-400"
                aria-hidden="true"
              />
            ) : isPending ? (
              <Clock
                className="size-5 text-amber-600 dark:text-amber-400"
                aria-hidden="true"
              />
            ) : (
              <ShieldAlert
                className="size-5 text-amber-600 dark:text-amber-500"
                aria-hidden="true"
              />
            )}
            <CardTitle
              id="account-verification-title"
              className="text-base sm:text-lg"
            >
              Verificación de la cuenta
            </CardTitle>
          </div>

          {snapshot.loading && !kycCase ? (
            <Badge variant="outline" className="text-muted-foreground gap-1.5">
              <Loader2 className="size-3 animate-spin" />
              Consultando estado...
            </Badge>
          ) : isVerified ? (
            <Badge className="bg-emerald-800 text-white hover:bg-emerald-800 font-medium">
              Cuenta verificada
            </Badge>
          ) : isPending ? (
            <Badge
              variant="outline"
              className="border-amber-500 text-amber-700 dark:text-amber-300 font-medium"
            >
              En revisión
            </Badge>
          ) : (
            <Badge
              variant="outline"
              className="border-amber-500 bg-amber-100 text-amber-800 dark:bg-amber-950/50 dark:text-amber-300 font-semibold"
            >
              Cuenta no verificada
            </Badge>
          )}
        </div>
        <CardDescription className="text-sm text-foreground/80 mt-1">
          {isVerified
            ? "Tu identidad ha sido verificada correctamente. Puedes arrendar, publicar espacios y operar de forma segura en EspaciGo."
            : isPending
              ? "Tu verificación de identidad está en proceso de revisión. Te notificaremos apenas esté confirmada."
              : "Verifica tu identidad ingresando tus datos oficiales y realizando la comprobación con Didit para habilitar arriendos y cobros seguros."}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {!isVerified && (
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 pt-2">
            <p className="text-xs text-muted-foreground">
              {isPending
                ? "Puedes consultar los detalles de la revisión en la sección de verificaciones."
                : "El proceso se realiza con tu cédula de identidad chilena y validación biométrica en pocos minutos."}
            </p>
            <Button asChild size="sm" className="shrink-0 gap-1.5 font-medium">
              <Link href="/mi-cuenta/verificaciones">
                {isPending
                  ? "Ver estado de verificación"
                  : isRejected
                    ? "Reintentar verificación"
                    : "Verificar cuenta"}
                <ArrowRight className="size-4" aria-hidden="true" />
              </Link>
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
