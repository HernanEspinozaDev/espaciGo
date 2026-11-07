"use client";

import { useState, type FormEvent } from "react";
import {
  AlertCircle,
  Clock,
  ExternalLink,
  FileCheck2,
  Loader2,
  RotateCcw,
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
import { Badge } from "@/components/ui/badge";
import { useAccount } from "@/features/account/ui/use-account";
import { useVerifications } from "./ui/use-verifications";
import { createDiditIntent } from "./verification-api";
import { formatRut, isValidRut } from "./rut-utils";

const CONSENT_VERSION = "kyc-didit-cl-2026-11-06";

function isAdult(date: string): boolean {
  const birth = new Date(`${date}T00:00:00`);
  if (!date || Number.isNaN(birth.getTime())) return false;
  const today = new Date();
  const age =
    today.getFullYear() -
    birth.getFullYear() -
    (today.getMonth() < birth.getMonth() ||
    (today.getMonth() === birth.getMonth() && today.getDate() < birth.getDate())
      ? 1
      : 0);
  return age >= 18 && birth <= today;
}

function getInitialNames(displayName?: string | null): {
  first: string;
  last: string;
} {
  if (!displayName) return { first: "", last: "" };
  const parts = displayName.trim().split(/\s+/);
  if (parts.length >= 2) {
    return {
      first: parts.slice(0, -1).join(" "),
      last: parts[parts.length - 1] ?? "",
    };
  }
  return { first: parts[0] ?? "", last: "" };
}

interface VerificationFormProps {
  readonly initialFirst: string;
  readonly initialLast: string;
  readonly onSaveProfile: (fullName: string) => Promise<void>;
}

function VerificationIdentityForm({
  initialFirst,
  initialLast,
  onSaveProfile,
}: VerificationFormProps) {
  const [firstName, setFirstName] = useState(initialFirst);
  const [lastName, setLastName] = useState(initialLast);
  const [dob, setDob] = useState("");
  const [rut, setRut] = useState("");
  const [consent, setConsent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    const trimmedFirst = firstName.trim();
    const trimmedLast = lastName.trim();

    if (!trimmedFirst || !trimmedLast) {
      setError("Por favor ingresa tus nombres y apellidos legales completos.");
      return;
    }

    if (!isValidRut(rut) || !isAdult(dob)) {
      setError(
        "Revisa el RUN/RUT y confirma que la fecha de nacimiento corresponda a una persona mayor de 18 años.",
      );
      return;
    }

    if (!consent) {
      setError(
        "Debes aceptar el consentimiento de tratamiento de datos para continuar.",
      );
      return;
    }

    setBusy(true);
    try {
      const fullDisplayName = `${trimmedFirst} ${trimmedLast}`;
      await onSaveProfile(fullDisplayName);

      const intent = await createDiditIntent({
        first_name: trimmedFirst,
        last_name: trimmedLast,
        date_of_birth: dob,
        rut: rut.replace(/\./g, "").toUpperCase(),
        consent: true,
        consent_version: CONSENT_VERSION,
      });

      if (intent.session_url) {
        window.location.assign(intent.session_url);
      } else {
        throw new Error("No se recibió URL de sesión de Didit");
      }
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "No pudimos iniciar la verificación. Inténtalo nuevamente más tarde.",
      );
      setBusy(false);
    }
  }

  return (
    <form className="space-y-5" onSubmit={submit}>
      {error && (
        <div
          role="alert"
          className="flex items-center gap-2 rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20"
        >
          <AlertCircle className="size-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label htmlFor="id-first-name">Nombres legales completos</Label>
          <Input
            id="id-first-name"
            name="first_name"
            value={firstName}
            onChange={(e) => setFirstName(e.target.value)}
            autoComplete="given-name"
            placeholder="Ej: Juan Andrés"
            required
            maxLength={120}
            disabled={busy}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="id-last-name">Apellidos legales completos</Label>
          <Input
            id="id-last-name"
            name="last_name"
            value={lastName}
            onChange={(e) => setLastName(e.target.value)}
            autoComplete="family-name"
            placeholder="Ej: Pérez González"
            required
            maxLength={120}
            disabled={busy}
          />
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label htmlFor="id-dob">Fecha de nacimiento</Label>
          <Input
            id="id-dob"
            name="date_of_birth"
            type="date"
            value={dob}
            onChange={(e) => setDob(e.target.value)}
            required
            max={new Date().toISOString().slice(0, 10)}
            disabled={busy}
          />
          <p className="text-xs text-muted-foreground">
            Debes ser mayor de 18 años.
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="id-rut">RUN / RUT chileno</Label>
          <Input
            id="id-rut"
            name="rut"
            inputMode="text"
            autoComplete="off"
            value={rut}
            onChange={(event) => setRut(formatRut(event.target.value))}
            placeholder="12.345.678-K"
            required
            aria-describedby="rut-help-text"
            aria-invalid={rut.length > 0 && !isValidRut(rut)}
            disabled={busy}
          />
          <p id="rut-help-text" className="text-xs text-muted-foreground">
            Formato con puntos y guion (ej: 12.345.678-K). No utilices el número
            de documento.
          </p>
        </div>
      </div>

      <div className="rounded-lg border border-border bg-muted/30 p-4 text-sm space-y-1.5">
        <div className="flex items-center gap-2 font-medium">
          <FileCheck2 className="size-4 text-primary" />
          <span>Documento admitido y validación</span>
        </div>
        <p className="text-xs text-muted-foreground">
          Cédula de Identidad chilena vigente (ambos lados) y prueba biométrica
          facial en vivo a través de Didit.
        </p>
      </div>

      <label className="flex items-start gap-3 text-sm cursor-pointer">
        <input
          name="consent"
          type="checkbox"
          checked={consent}
          onChange={(e) => setConsent(e.target.checked)}
          required
          disabled={busy}
          className="mt-1 size-4 rounded border-border text-primary focus:ring-ring"
        />
        <span className="text-xs leading-relaxed text-muted-foreground">
          Consiento expresamente el tratamiento de mis datos de identidad y
          biométricos por EspaciGo y Didit para verificar mi identidad oficial.
          Entiendo que se capturarán imágenes de mi cédula y una prueba
          biométrica facial. Este consentimiento es independiente de
          comunicaciones comerciales.
        </span>
      </label>

      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4 border-t border-border pt-4">
        <p className="text-xs text-muted-foreground">
          Al continuar, serás redirigido de forma segura a Didit para completar
          el escaneo.
        </p>
        <Button
          type="submit"
          disabled={busy}
          className="shrink-0 gap-2 font-semibold"
        >
          {busy ? (
            <>
              <Loader2 className="size-4 animate-spin" />
              Iniciando verificación…
            </>
          ) : (
            <>
              Guardar y verificar identidad con Didit
              <ExternalLink className="size-4" />
            </>
          )}
        </Button>
      </div>
    </form>
  );
}

export function VerificationFlow() {
  const { snapshot: accountSnapshot, updateProfile } = useAccount();
  const { snapshot: verificationSnapshot, loadOverview } = useVerifications();

  const [showFormAnyway, setShowFormAnyway] = useState(false);

  const kycCase = verificationSnapshot.verifications.find(
    (v) => v.type === "kyc",
  );
  const isApproved = kycCase?.state === "aprobada";
  const isPending = kycCase?.state === "en_revision";

  const handleSaveProfile = async (fullDisplayName: string) => {
    if (
      !accountSnapshot.profile?.displayName ||
      accountSnapshot.profile.displayName !== fullDisplayName
    ) {
      try {
        await updateProfile({
          displayName: fullDisplayName,
        });
      } catch {
        // Error auxiliar no bloqueante
      }
    }
  };

  const initialNames = getInitialNames(accountSnapshot.profile?.displayName);

  if (isApproved && !showFormAnyway) {
    return (
      <Card
        aria-labelledby="identity-flow-title"
        className="border-emerald-200 bg-emerald-50/20 dark:border-emerald-900/40 dark:bg-emerald-950/10"
      >
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ShieldCheck
                className="size-5 text-emerald-800 dark:text-emerald-400"
                aria-hidden="true"
              />
              <CardTitle id="identity-flow-title">
                Verificación de usuario (Identidad)
              </CardTitle>
            </div>
            <Badge className="bg-emerald-800 text-white font-medium">
              Identidad verificada
            </Badge>
          </div>
          <CardDescription>
            Tu identidad personal ya ha sido validada oficialmente mediante
            Didit y se encuentra activa.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-foreground/80">
            Tu cuenta cumple con todas las normas de seguridad de EspaciGo para
            realizar arriendos, recibir pagos y publicar espacios.
          </p>
          <div className="pt-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowFormAnyway(true)}
              className="text-xs"
            >
              Actualizar o volver a verificar datos
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (isPending && !showFormAnyway) {
    return (
      <Card
        aria-labelledby="identity-flow-title"
        className="border-amber-200 bg-amber-50/20 dark:border-amber-900/40 dark:bg-amber-950/10"
      >
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Clock
                className="size-5 text-amber-600 dark:text-amber-400"
                aria-hidden="true"
              />
              <CardTitle id="identity-flow-title">
                Verificación de usuario (Identidad)
              </CardTitle>
            </div>
            <Badge
              variant="outline"
              className="border-amber-500 text-amber-700 dark:text-amber-300 font-medium"
            >
              En revisión
            </Badge>
          </div>
          <CardDescription>
            Tu verificación de identidad se encuentra actualmente en proceso de
            validación.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-foreground/80">
            Estamos procesando los antecedentes y la confirmación de Didit.
            Puedes revisar el avance en la sección de historial abajo.
          </p>
          <div className="flex items-center gap-3 pt-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => void loadOverview()}
              className="gap-1.5"
            >
              <RotateCcw className="size-3.5" />
              Consultar estado actual
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowFormAnyway(true)}
              className="text-xs text-muted-foreground"
            >
              Ingresar nuevos datos
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card aria-labelledby="identity-flow-title">
      <CardHeader>
        <div className="flex items-center gap-2">
          <ShieldCheck className="size-5 text-primary" aria-hidden="true" />
          <CardTitle id="identity-flow-title">
            Verificación de usuario (Identidad personal)
          </CardTitle>
        </div>
        <CardDescription>
          Ingresa tus datos tal como aparecen en tu carnet de identidad chileno.
          Luego de guardarlos, serás dirigido a Didit para realizar la
          validación biométrica y escaneo del documento.
        </CardDescription>
      </CardHeader>

      <CardContent>
        <VerificationIdentityForm
          key={accountSnapshot.profile?.updatedAt ?? "initial"}
          initialFirst={initialNames.first}
          initialLast={initialNames.last}
          onSaveProfile={handleSaveProfile}
        />
      </CardContent>
    </Card>
  );
}
