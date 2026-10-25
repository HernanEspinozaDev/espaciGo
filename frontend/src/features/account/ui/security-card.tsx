"use client";

import { useState, type FormEvent } from "react";
import {
  AlertCircle,
  CheckCircle2,
  KeyRound,
  Loader2,
  Lock,
  ShieldCheck,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/features/auth/ui/use-auth";
import { inspectPassword } from "@/features/auth/domain/password-policy";
import { apiClient } from "@/lib/api/client";
import { normalizeApiError } from "@/lib/api/errors";

export function SecurityCard() {
  const { accessToken } = useAuth();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!accessToken) return;

    setError(null);
    setSuccess(null);

    if (newPassword !== confirmPassword) {
      setError("Las contraseñas nuevas no coinciden.");
      return;
    }

    const policyCheck = inspectPassword(newPassword);
    if (!policyCheck.valid) {
      setError(
        "La contraseña debe tener mínimo 8 caracteres (máx. 72 bytes UTF-8), al menos una mayúscula, un número y un símbolo o signo de puntuación.",
      );
      return;
    }

    try {
      setIsSubmitting(true);
      const result = await apiClient.POST("/auth/password/change", {
        headers: {
          Authorization: ["Bearer", accessToken].join(" "),
        },
        body: {
          current_password: currentPassword,
          new_password: newPassword,
          confirm_password: confirmPassword,
        },
      });

      if (!result.response.ok) {
        throw normalizeApiError(result.response, result.error);
      }

      setSuccess("Tu contraseña ha sido actualizada con éxito.");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al cambiar la contraseña",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card aria-labelledby="security-password-title">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Lock className="size-5 text-primary" aria-hidden="true" />
            <CardTitle id="security-password-title">
              Actualizar Contraseña
            </CardTitle>
          </div>
          <CardDescription>
            Asegura tu cuenta cambiando tu clave periódicamente. Mínimo 8
            caracteres, con mayúscula, número y símbolo.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error && (
            <div
              role="alert"
              className="mb-4 flex items-center gap-2 rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20"
            >
              <AlertCircle className="size-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {success && (
            <div
              role="status"
              className="mb-4 flex items-center gap-2 rounded-md bg-emerald-50 p-3 text-sm text-emerald-800 border border-emerald-200 dark:bg-emerald-950/30 dark:border-emerald-800 dark:text-emerald-300"
            >
              <CheckCircle2 className="size-4 shrink-0" />
              <span>{success}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4 max-w-md">
            <div className="space-y-2">
              <Label htmlFor="current-password">Contraseña actual</Label>
              <Input
                id="current-password"
                name="currentPassword"
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                required
                disabled={isSubmitting}
                autoComplete="current-password"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="new-password">Nueva contraseña</Label>
              <Input
                id="new-password"
                name="newPassword"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                required
                disabled={isSubmitting}
                autoComplete="new-password"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="confirm-password">
                Confirmar nueva contraseña
              </Label>
              <Input
                id="confirm-password"
                name="confirmPassword"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                disabled={isSubmitting}
                autoComplete="new-password"
              />
            </div>

            <div className="pt-2 flex justify-start">
              <Button type="submit" disabled={isSubmitting || !accessToken}>
                {isSubmitting && (
                  <Loader2
                    className="mr-2 size-4 animate-spin"
                    aria-hidden="true"
                  />
                )}
                Cambiar contraseña
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card aria-labelledby="security-status-title">
        <CardHeader>
          <div className="flex items-center gap-2">
            <ShieldCheck className="size-5 text-primary" aria-hidden="true" />
            <CardTitle id="security-status-title">
              Estado de Seguridad de la Sesión
            </CardTitle>
          </div>
          <CardDescription>
            Protección de credenciales y aislamiento de sesión activa.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-3 rounded-lg border p-4 bg-muted/30">
            <KeyRound className="size-5 text-muted-foreground shrink-0" />
            <div className="text-sm">
              <p className="font-medium">Sesión criptográficamente firmada</p>
              <p className="text-muted-foreground text-xs mt-0.5">
                Tus tokens de acceso residen en memoria protegida del cliente
                durante la sesión activa.
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
