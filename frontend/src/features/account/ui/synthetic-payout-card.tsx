"use client";

import { useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  CreditCard,
  FlaskConical,
  Loader2,
  ShieldCheck,
  Trash2,
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
import { useAccount } from "./use-account";

export function SyntheticPayoutCard() {
  const { snapshot: authSnapshot } = useAuth();
  const { snapshot, createOrReplacePayoutAccount, revokePayoutAccount } =
    useAccount();
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleCreate = async () => {
    if (authSnapshot.status !== "authenticated") return;
    setError(null);
    setMessage(null);
    setLoadingAction("create");
    try {
      const idempotencyKey = crypto.randomUUID();
      await createOrReplacePayoutAccount(idempotencyKey);
      setMessage("Cuenta de cobro sintética configurada para ensayos.");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Error al configurar la cuenta de cobro sintética",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const handleRevoke = async () => {
    if (authSnapshot.status !== "authenticated") return;
    setError(null);
    setMessage(null);
    setLoadingAction("revoke");
    try {
      const idempotencyKey = crypto.randomUUID();
      await revokePayoutAccount(idempotencyKey);
      setMessage("Cuenta de cobro sintética revocada.");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Error al revocar la cuenta de cobro",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const payout = snapshot.payoutAccount;

  return (
    <Card aria-labelledby="payout-title">
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <CreditCard className="size-5 text-primary" aria-hidden="true" />
            <CardTitle id="payout-title">Cuenta de cobro (Payout)</CardTitle>
          </div>
          {payout && (
            <Badge variant="secondary" className="gap-1">
              <ShieldCheck className="size-3.5 text-emerald-600" />
              {payout.status}
            </Badge>
          )}
        </div>
        <CardDescription>
          Canal simulado para recepción de fondos y liquidaciones en entorno de
          ensayo.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div
          role="note"
          className="flex items-start gap-2 rounded-md bg-amber-50 p-3 text-xs text-amber-900 border border-amber-200 dark:bg-amber-950/30 dark:border-amber-800 dark:text-amber-200"
        >
          <FlaskConical className="size-4 shrink-0 mt-0.5 text-amber-600" />
          <div>
            <span className="font-semibold">Entorno de ensayo:</span> No se
            utilizan ni requieren cuentas bancarias reales ni RUT personal.
            Cualquier depósito o acreditación se realiza con identificadores
            ficticios en sandbox.
          </div>
        </div>

        {error && (
          <div
            role="alert"
            className="flex items-center gap-2 rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20"
          >
            <AlertCircle className="size-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {message && (
          <div
            role="status"
            className="flex items-center gap-2 rounded-md bg-emerald-50 p-3 text-sm text-emerald-800 border border-emerald-200 dark:bg-emerald-950/30 dark:border-emerald-800 dark:text-emerald-300"
          >
            <CheckCircle2 className="size-4 shrink-0" />
            <span>{message}</span>
          </div>
        )}

        {payout ? (
          <div className="space-y-3 rounded-lg border p-4 bg-muted/40">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-muted-foreground">Aviso de ensayo:</span>{" "}
                <span className="font-medium">{payout.notice}</span>
              </div>
              <div>
                <span className="text-muted-foreground">Ref. Ficticia:</span>{" "}
                <span className="font-medium font-mono">
                  {payout.referenceId}
                </span>
              </div>
            </div>
            <div className="pt-2 flex justify-end">
              <Button
                type="button"
                variant="destructive"
                onClick={handleRevoke}
                disabled={
                  Boolean(loadingAction) ||
                  authSnapshot.status !== "authenticated"
                }
              >
                {loadingAction === "revoke" ? (
                  <Loader2
                    className="mr-2 size-4 animate-spin"
                    aria-hidden="true"
                  />
                ) : (
                  <Trash2 className="mr-2 size-4" aria-hidden="true" />
                )}
                Revocar cuenta de cobro
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 rounded-lg border border-dashed p-4">
            <p className="text-sm text-muted-foreground">
              No tienes ninguna cuenta de cobro configurada.
            </p>
            <Button
              type="button"
              onClick={handleCreate}
              disabled={
                Boolean(loadingAction) ||
                authSnapshot.status !== "authenticated"
              }
            >
              {loadingAction === "create" ? (
                <Loader2
                  className="mr-2 size-4 animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <CreditCard className="mr-2 size-4" aria-hidden="true" />
              )}
              Crear cuenta de cobro sintética
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
