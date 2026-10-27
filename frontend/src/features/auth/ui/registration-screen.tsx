"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { ApiError } from "@/lib/api/errors";
import { Button } from "@/components/workspace/ui/button";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import type { ConsentSet } from "../domain/consent-documents";
import { validateConsentDocuments } from "../domain/consent-documents";
import { onboardingErrorCopy } from "../domain/onboarding-errors";
import { inspectPassword } from "../domain/password-policy";
import { setRegistrationResult } from "../application/registration-result";
import { useOnboarding } from "./use-onboarding";
import { AuthFeedback, AuthShell } from "./auth-shell";
import { PasswordField, PasswordRequirements } from "./password-field";

function isStaleConsent(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    (error.code === "consent_invalid" ||
      error.fields.some((field) => field.code === "consent_invalid"))
  );
}

export function RegistrationScreen() {
  const router = useRouter();
  const onboarding = useOnboarding();
  const [documents, setDocuments] = useState<ConsentSet | null>(null);
  const [loading, setLoading] = useState(true);
  const [termsError, setTermsError] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [termsAccepted, setTermsAccepted] = useState(false);
  const [privacyAccepted, setPrivacyAccepted] = useState(false);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [feedbackKind, setFeedbackKind] = useState<"error" | "status">("error");
  const [consentIssue, setConsentIssue] = useState(false);
  const [ambiguous, setAmbiguous] = useState(false);
  const noticeRef = useRef<HTMLDivElement>(null);

  const loadDocuments = useCallback(async () => {
    setLoading(true);
    setTermsError("");
    setDocuments(null);
    setTermsAccepted(false);
    setPrivacyAccepted(false);
    setConsentIssue(false);
    try {
      const result = await onboarding.terms();
      const validated = validateConsentDocuments(result);
      if (!validated) throw new Error("terms_incomplete");
      setDocuments(validated);
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      setTermsError(
        error instanceof ApiError && error.code === "terms_unavailable"
          ? "El servicio no dispone de documentos vigentes completos. El registro permanece bloqueado."
          : "No pudimos verificar documentos completos y vigentes. El registro permanece bloqueado.",
      );
    } finally {
      setLoading(false);
    }
  }, [onboarding]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadDocuments(), 0);
    return () => window.clearTimeout(timer);
  }, [loadDocuments]);

  useEffect(() => {
    if (message || termsError) noticeRef.current?.focus();
  }, [message, termsError]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!documents || !termsAccepted || !privacyAccepted || ambiguous) return;
    setPending(true);
    setMessage("");
    setFeedbackKind("error");
    setConsentIssue(false);
    try {
      const status = await onboarding.register({
        email,
        password,
        terms_version_ids: [documents.terms.id, documents.privacy.id],
      });
      setPassword("");
      setTermsAccepted(false);
      setPrivacyAccepted(false);
      setFeedbackKind("status");
      setConsentIssue(false);
      setRegistrationResult({ email, status });
      router.replace("/registro-exitoso");
    } catch (error) {
      setPassword("");
      if (isStaleConsent(error)) {
        setTermsAccepted(false);
        setPrivacyAccepted(false);
        setMessage(onboardingErrorCopy("consent_invalid"));
        await loadDocuments();
        setConsentIssue(true);
      } else {
        setConsentIssue(false);
        setMessage(
          error instanceof ApiError
            ? onboardingErrorCopy(
                error.code === "email_registered"
                  ? "email_registered"
                  : error.code,
              )
            : "No recibimos una confirmación del registro. No lo enviaremos de nuevo automáticamente; puedes acceder o recuperar el acceso antes de volver a intentarlo.",
        );
        if (!(error instanceof ApiError) || error.status >= 500) {
          setAmbiguous(true);
        }
      }
    } finally {
      setPending(false);
    }
  }

  const passwordValid = inspectPassword(password).valid;
  const consentReady = Boolean(documents && termsAccepted && privacyAccepted);

  return (
    <AuthShell
      title="Crea tu cuenta"
      description="Únete a EspaciGo para reservar o publicar espacios."
      secondaryQuestion="¿Ya tienes una cuenta?"
      secondaryLabel="Ir a acceso"
      secondaryHref="/acceso"
    >
      <form className="space-y-6" onSubmit={submit}>
        {loading ? (
          <p role="status" className="text-sm text-gray-700">
            Cargando condiciones…
          </p>
        ) : null}
        {termsError ? (
          <div
            ref={noticeRef}
            tabIndex={-1}
            role="alert"
            className="space-y-3 rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950 focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-700"
          >
            <p>{termsError}</p>
            <Button
              type="button"
              variant="outline"
              onClick={() => void loadDocuments()}
              className="min-h-11 rounded-full"
            >
              Reintentar carga
            </Button>
          </div>
        ) : null}

        <div>
          <Label
            htmlFor="register-email"
            className="block text-sm font-medium text-gray-700"
          >
            Correo electrónico
          </Label>
          <Input
            id="register-email"
            name="email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            required
            disabled={pending}
            className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 text-gray-900 focus-visible:ring-2 focus-visible:ring-orange-700 sm:text-sm"
          />
        </div>

        <div className="space-y-2">
          <PasswordField
            id="register-password"
            label="Contraseña"
            autoComplete="new-password"
            value={password}
            onChange={setPassword}
            disabled={pending}
            invalid={Boolean(password) && !passwordValid}
            describedBy="password-requirements"
          />
          <PasswordRequirements value={password} />
        </div>

        {documents ? (
          <div className="space-y-3 pt-1">
            <label className="flex min-h-11 cursor-pointer items-start gap-3 rounded-md py-2 text-sm text-gray-800">
              <input
                type="checkbox"
                id="terms-and-conditions-consent"
                checked={termsAccepted}
                onChange={(event) => setTermsAccepted(event.target.checked)}
                disabled={pending}
                aria-invalid={!termsAccepted && consentIssue ? true : undefined}
                aria-describedby={
                  !termsAccepted && consentIssue ? "consent-error" : undefined
                }
                className="mt-0.5 size-5 shrink-0 accent-orange-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
              />
              <span>
                Acepto los{" "}
                <a
                  href="/terminos-y-condiciones"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="rounded-xs text-orange-800 underline underline-offset-4 hover:text-orange-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700"
                >
                  Términos y condiciones
                </a>
                .
              </span>
            </label>

            <label className="flex min-h-11 cursor-pointer items-start gap-3 rounded-md py-2 text-sm text-gray-800">
              <input
                type="checkbox"
                id="privacy-policy-consent"
                checked={privacyAccepted}
                onChange={(event) => setPrivacyAccepted(event.target.checked)}
                disabled={pending}
                aria-invalid={
                  !privacyAccepted && consentIssue ? true : undefined
                }
                aria-describedby={
                  !privacyAccepted && consentIssue ? "consent-error" : undefined
                }
                className="mt-0.5 size-5 shrink-0 accent-orange-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
              />
              <span>
                He leído y acepto la{" "}
                <a
                  href="/politica-de-privacidad"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="rounded-xs text-orange-800 underline underline-offset-4 hover:text-orange-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700"
                >
                  Política de privacidad
                </a>
                .
              </span>
            </label>

            {consentIssue && !termsError ? (
              <p id="consent-error" className="text-sm text-red-800">
                Revisa y acepta por separado ambos documentos.
              </p>
            ) : null}
          </div>
        ) : null}

        <AuthFeedback
          message={message}
          kind={feedbackKind}
          focusRef={noticeRef}
        />

        {message.includes("ya tiene una cuenta") ? (
          <div className="flex flex-wrap gap-3 text-sm">
            <Link
              href="/acceso"
              className="inline-flex min-h-11 items-center text-orange-800 underline underline-offset-4"
            >
              Acceder
            </Link>
            <Link
              href="/recuperar-acceso"
              className="inline-flex min-h-11 items-center text-orange-800 underline underline-offset-4"
            >
              Recuperar acceso
            </Link>
          </div>
        ) : null}

        <Button
          type="submit"
          disabled={
            pending ||
            loading ||
            !documents ||
            !consentReady ||
            !passwordValid ||
            ambiguous
          }
          className="min-h-11 w-full rounded-full bg-orange-700 text-white hover:bg-orange-800 focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
        >
          {pending ? "Creando cuenta…" : "Crear cuenta"}
        </Button>
      </form>
    </AuthShell>
  );
}
