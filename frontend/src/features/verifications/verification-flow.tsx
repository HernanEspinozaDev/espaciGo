"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import {
  AlertCircle,
  CheckCircle2,
  FileCheck2,
  Loader2,
  Pencil,
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
import { useSessionIdentity } from "@/features/auth/ui/use-session-identity";
import { AccountVerificationCard } from "@/features/account/ui/account-verification-card";
import {
  getIdentityDeclaration,
  saveIdentityDeclaration,
  type IdentityDeclarationResponse,
} from "./verification-api";
import { formatRut, isValidRut, normalizeRut } from "./rut-utils";

const DECLARATION_CONSENT_VERSION = "id-consent-20261108";

function isAdult(date: string): boolean {
  const birth = new Date(`${date}T00:00:00Z`);
  if (!date || Number.isNaN(birth.getTime())) return false;
  const today = new Date();
  const age =
    today.getUTCFullYear() -
    birth.getUTCFullYear() -
    (today.getUTCMonth() < birth.getUTCMonth() ||
    (today.getUTCMonth() === birth.getUTCMonth() &&
      today.getUTCDate() < birth.getUTCDate())
      ? 1
      : 0);
  return age >= 18 && birth <= today;
}

interface VerificationIdentityFormProps {
  readonly isAuthenticated: boolean;
}

function VerificationIdentityForm({
  isAuthenticated,
}: VerificationIdentityFormProps) {
  const [savedDeclaration, setSavedDeclaration] =
    useState<IdentityDeclarationResponse | null>(null);
  const [editing, setEditing] = useState(true);
  const [loadingDeclaration, setLoadingDeclaration] = useState(isAuthenticated);
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadToken, setReloadToken] = useState(0);

  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [dob, setDob] = useState("");
  const [rut, setRut] = useState("");
  const [declarationConsent, setDeclarationConsent] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isAuthenticated) {
      return;
    }
    let cancelled = false;
    void getIdentityDeclaration()
      .then((current) => {
        if (cancelled) return;
        if (current) {
          setSavedDeclaration(current);
          setFirstName(current.first_name);
          setLastName(current.last_name);
          setDob(current.date_of_birth);
          setRut(formatRut(current.rut));
          setEditing(false);
        } else {
          setSavedDeclaration(null);
          setEditing(true);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setLoadFailed(true);
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingDeclaration(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, reloadToken]);

  const handleRetryLoad = useCallback(() => {
    setError(null);
    setLoadFailed(false);
    setLoadingDeclaration(true);
    setReloadToken((prev) => prev + 1);
  }, []);

  async function handleSaveDeclaration(event: FormEvent<HTMLFormElement>) {
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

    if (!declarationConsent) {
      setError(
        "Debes aceptar el consentimiento de declaración de identidad para guardar tus datos.",
      );
      return;
    }

    setSaving(true);
    try {
      const normalizedRut = normalizeRut(rut);
      await saveIdentityDeclaration({
        first_name: trimmedFirst,
        last_name: trimmedLast,
        date_of_birth: dob,
        rut: normalizedRut,
        id_country: "CHL",
        document_type: "ID",
        consent_given: true,
        consent_version: DECLARATION_CONSENT_VERSION,
      });
      const confirmed = await getIdentityDeclaration();
      if (!confirmed) {
        throw new Error(
          "No fue posible confirmar la lectura de los datos guardados.",
        );
      }
      setSavedDeclaration(confirmed);
      setFirstName(confirmed.first_name);
      setLastName(confirmed.last_name);
      setDob(confirmed.date_of_birth);
      setRut(formatRut(confirmed.rut));
      setEditing(false);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "No pudimos guardar tus datos de identidad. Inténtalo nuevamente.",
      );
    } finally {
      setSaving(false);
    }
  }

  if (loadingDeclaration) {
    return (
      <p role="status" className="text-sm text-muted-foreground">
        Consultando declaración de identidad guardada…
      </p>
    );
  }

  if (loadFailed) {
    return (
      <div className="space-y-4">
        <div
          role="alert"
          className="flex items-center gap-2 rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20"
        >
          <AlertCircle className="size-4 shrink-0" />
          <span>
            No pudimos consultar el estado de tu declaración de identidad.
          </span>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={handleRetryLoad}
          className="gap-1.5"
        >
          <RotateCcw className="size-3.5" />
          Reintentar consulta
        </Button>
      </div>
    );
  }

  if (savedDeclaration && !editing) {
    return (
      <div className="space-y-5">
        {error && (
          <div
            role="alert"
            className="flex items-center gap-2 rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20"
          >
            <AlertCircle className="size-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <div className="rounded-lg border border-emerald-200 bg-emerald-50/30 dark:border-emerald-900/40 dark:bg-emerald-950/10 p-4 space-y-3">
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2 text-sm font-semibold text-emerald-900 dark:text-emerald-300">
              <CheckCircle2 className="size-4 shrink-0" />
              <span>
                Datos guardados (revisión #{savedDeclaration.revision})
              </span>
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={saving}
              onClick={() => {
                setDeclarationConsent(false);
                setEditing(true);
              }}
              className="gap-1.5 text-xs"
            >
              <Pencil className="size-3.5" />
              Editar datos
            </Button>
          </div>

          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
            <div>
              <dt className="text-xs text-muted-foreground">Nombres legales</dt>
              <dd className="font-medium">{savedDeclaration.first_name}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">
                Apellidos legales
              </dt>
              <dd className="font-medium">{savedDeclaration.last_name}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">
                Fecha de nacimiento
              </dt>
              <dd className="font-medium">{savedDeclaration.date_of_birth}</dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">
                RUN / RUT chileno
              </dt>
              <dd className="font-medium">{formatRut(savedDeclaration.rut)}</dd>
            </div>
          </dl>
        </div>

        <AccountVerificationCard showDeclarationLink={false} />
      </div>
    );
  }

  return (
    <form className="space-y-5" onSubmit={handleSaveDeclaration}>
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
            maxLength={150}
            disabled={saving}
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
            maxLength={150}
            disabled={saving}
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
            disabled={saving}
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
            disabled={saving}
          />
          <p id="rut-help-text" className="text-xs text-muted-foreground">
            Formato con puntos y guion (ej: 12.345.678-K). No utilices el número
            de serie documental.
          </p>
        </div>
      </div>

      <div className="rounded-lg border border-border bg-muted/30 p-4 text-sm space-y-1.5">
        <div className="flex items-center gap-2 font-medium">
          <FileCheck2 className="size-4 text-primary" />
          <span>
            Documento admitido: Cédula de Identidad chilena (CHL / ID)
          </span>
        </div>
        <p className="text-xs text-muted-foreground">
          Tus datos legales se guardan cifrados y separados de tu nombre visible
          público. Guardar esta declaración no inicia una sesión externa.
        </p>
      </div>

      <label className="flex items-start gap-3 text-sm cursor-pointer">
        <input
          name="consent"
          type="checkbox"
          checked={declarationConsent}
          onChange={(e) => setDeclarationConsent(e.target.checked)}
          required
          disabled={saving}
          className="mt-1 size-4 rounded border-border text-primary focus:ring-ring"
        />
        <span className="text-xs leading-relaxed text-muted-foreground">
          Declaro que los datos corresponden a mi Cédula de Identidad chilena
          vigente y consiento su almacenamiento cifrado por EspaciGo para la
          verificación de mi identidad (máximo 30 días mientras esté pendiente).
        </span>
      </label>

      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4 border-t border-border pt-4">
        <p className="text-xs text-muted-foreground">
          Paso 1 de 2: guarda y confirma tus datos antes de continuar con la
          revisión documental propia.
        </p>
        <Button
          type="submit"
          disabled={saving}
          className="shrink-0 gap-2 font-semibold"
        >
          {saving ? (
            <>
              <Loader2 className="size-4 animate-spin" />
              Guardando datos…
            </>
          ) : (
            "Guardar datos de identidad"
          )}
        </Button>
      </div>
    </form>
  );
}
export function VerificationFlow() {
  const sessionIdentity = useSessionIdentity();
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
          Ingresa tus datos tal como aparecen en tu Cédula de Identidad chilena.
          La declaración legal y la revisión documental son pasos separados;
          guarda los datos para consultar aquí el estado de la solicitud.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-5">
        <VerificationIdentityForm
          key={`${sessionIdentity.status}:${sessionIdentity.accountId ?? "none"}`}
          isAuthenticated={sessionIdentity.status === "authenticated"}
        />
      </CardContent>
    </Card>
  );
}
