"use client";

import { useEffect, useState, type FormEvent } from "react";
import { AlertCircle, CheckCircle2, Loader2, User } from "lucide-react";
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
import { useAccount } from "./use-account";
import { normalizeChileanPhone, validateProfileInput } from "../domain/profile";

interface ProfileFormProps {
  readonly initialName: string;
  readonly initialPhone: string;
  readonly isSubmitting: boolean;
  readonly hasProfile: boolean;
  readonly onSubmit: (name: string, phone: string) => Promise<void>;
}

function ProfileForm({
  initialName,
  initialPhone,
  isSubmitting,
  hasProfile,
  onSubmit,
}: ProfileFormProps) {
  const [name, setName] = useState(initialName);
  const [phone, setPhone] = useState(initialPhone);
  const [localError, setLocalError] = useState<string | null>(null);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLocalError(null);

    const trimmedName = name.trim();
    const trimmedPhone = phone.trim();

    const validation = validateProfileInput({
      displayName: trimmedName,
      phone: trimmedPhone || null,
    });

    if (!validation.valid) {
      const firstError =
        validation.errors.displayName ??
        validation.errors.phone ??
        "Datos inválidos";
      setLocalError(firstError);
      return;
    }

    await onSubmit(trimmedName, trimmedPhone);
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {localError && (
        <div
          role="alert"
          className="flex items-center gap-2 rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20"
        >
          <AlertCircle className="size-4 shrink-0" />
          <span>{localError}</span>
        </div>
      )}

      <div className="space-y-2">
        <Label htmlFor="profile-name">Nombre visible</Label>
        <Input
          id="profile-name"
          name="name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Ej: Juan Pérez"
          required
          disabled={isSubmitting}
          autoComplete="name"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="profile-phone">
          Teléfono móvil (Chile - 9 dígitos)
        </Label>
        <Input
          id="profile-phone"
          name="phone"
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
          placeholder="912345678 ó +56 9 1234 5678"
          disabled={isSubmitting}
          autoComplete="tel"
        />
        <p className="text-xs text-muted-foreground">
          Formato aceptado: 9 dígitos comenzando con 9 (ej. 912345678 o +56 9
          1234 5678).
        </p>
      </div>

      <div className="pt-2 flex justify-end">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting && (
            <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
          )}
          {hasProfile ? "Guardar cambios" : "Crear perfil"}
        </Button>
      </div>
    </form>
  );
}

export function ProfileCard() {
  const { snapshot: authSnapshot } = useAuth();
  const { snapshot, loadOverview, updateProfile } = useAccount();
  const [clientError, setClientError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (authSnapshot.status === "authenticated") {
      loadOverview().catch(() => {
        // Ignored: snapshot captures error or 404 handled gracefully
      });
    }
  }, [authSnapshot.status, loadOverview]);

  const handleFormSubmit = async (
    trimmedName: string,
    trimmedPhone: string,
  ) => {
    if (authSnapshot.status !== "authenticated") return;
    setClientError(null);
    setSuccessMessage(null);

    const normalizedPhone = normalizeChileanPhone(trimmedPhone);

    try {
      setIsSubmitting(true);
      await updateProfile({
        displayName: trimmedName,
        phone: normalizedPhone ?? undefined,
      });
      setSuccessMessage("Perfil guardado correctamente.");
    } catch (err) {
      setClientError(
        err instanceof Error ? err.message : "Error al guardar el perfil",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const isProfileEmpty = !snapshot.profile && !snapshot.loading;

  return (
    <Card aria-labelledby="profile-title">
      <CardHeader>
        <div className="flex items-center gap-2">
          <User className="size-5 text-primary" aria-hidden="true" />
          <CardTitle id="profile-title">Información de perfil</CardTitle>
        </div>
        <CardDescription>
          Gestiona tu nombre visible y número de contacto móvil de Chile (+56 9
          XXXXXXXX).
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isProfileEmpty && (
          <div
            role="status"
            className="mb-4 rounded-md bg-amber-50 p-3 text-sm text-amber-800 border border-amber-200 dark:bg-amber-950/30 dark:border-amber-800 dark:text-amber-300"
          >
            Tu perfil aún no está configurado. Completa tus datos para crearlo.
          </div>
        )}

        {clientError && (
          <div
            role="alert"
            className="mb-4 flex items-center gap-2 rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20"
          >
            <AlertCircle className="size-4 shrink-0" />
            <span>{clientError}</span>
          </div>
        )}

        {successMessage && (
          <div
            role="status"
            className="mb-4 flex items-center gap-2 rounded-md bg-emerald-50 p-3 text-sm text-emerald-800 border border-emerald-200 dark:bg-emerald-950/30 dark:border-emerald-800 dark:text-emerald-300"
          >
            <CheckCircle2 className="size-4 shrink-0" />
            <span>{successMessage}</span>
          </div>
        )}

        {authSnapshot.status === "authenticated" ? (
          <ProfileForm
            key={snapshot.profile?.updatedAt ?? "empty"}
            initialName={snapshot.profile?.displayName ?? ""}
            initialPhone={snapshot.profile?.phone ?? ""}
            isSubmitting={isSubmitting || snapshot.loading}
            hasProfile={Boolean(snapshot.profile)}
            onSubmit={handleFormSubmit}
          />
        ) : (
          <p className="text-sm text-muted-foreground">
            Inicia sesión para ver y editar tu perfil.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
