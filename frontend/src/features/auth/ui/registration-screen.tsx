"use client";

import Link from "next/link";
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
import type { ConsentDocument, ConsentSet } from "../domain/consent-documents";
import { validateConsentDocuments } from "../domain/consent-documents";
import { onboardingErrorCopy } from "../domain/onboarding-errors";
import { inspectPassword } from "../domain/password-policy";
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

function DocumentPanel({
  document,
  checked,
  onChange,
  invalid,
}: {
  document: ConsentDocument;
  checked: boolean;
  onChange(checked: boolean): void;
  invalid: boolean;
}) {
  const label =
    document.type === "terminos"
      ? "Términos y condiciones"
      : "Política de privacidad";
  return (
    <section className="space-y-3 rounded-xl border border-gray-200 bg-white p-4">
      <details open>
        <summary className="min-h-11 cursor-pointer rounded-md py-2 font-semibold text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700">
          {document.document.title}
        </summary>
        <div className="space-y-4 pb-3 pt-2 text-sm leading-relaxed text-gray-700">
          {document.document.sections.map((section, index) => (
            <section key={`${document.id}-${index}`}>
              <h2 className="font-semibold text-gray-900">{section.heading}</h2>
              <p className="mt-1 whitespace-pre-wrap">{section.body}</p>
            </section>
          ))}
        </div>
      </details>
      <label className="flex min-h-11 cursor-pointer items-start gap-3 rounded-md py-2 text-sm text-gray-800">
        <input
          type="checkbox"
          checked={checked}
          onChange={(event) => onChange(event.target.checked)}
          aria-invalid={invalid || undefined}
          aria-describedby={invalid ? "consent-error" : undefined}
          className="mt-1 size-5 shrink-0 accent-orange-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
        />
        <span>He leído y acepto {label}.</span>
      </label>
    </section>
  );
}

export function RegistrationScreen() {
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
      setMessage(
        status === 201
          ? "La cuenta quedó registrada. Revisa las instrucciones de verificación del correo."
          : "La cuenta quedó registrada, pero no se confirmó la entrega de instrucciones. Solicita una reemisión de verificación.",
      );
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
        if (!(error instanceof ApiError) || error.status >= 500)
          setAmbiguous(true);
      }
    } finally {
      setPending(false);
    }
  }

  const passwordValid = inspectPassword(password).valid;
  const consentReady = Boolean(documents && termsAccepted && privacyAccepted);

  return (
    <AuthShell
      title="Crear cuenta"
      description="Regístrate con los documentos vigentes que revisarás a continuación."
      secondaryQuestion="¿Ya tienes una cuenta?"
      secondaryLabel="Ir a acceso"
      secondaryHref="/acceso"
    >
      <form className="space-y-6" onSubmit={submit}>
        {loading ? (
          <p role="status" className="text-sm text-gray-700">
            Cargando documentos…
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
        {documents ? (
          <div className="space-y-3">
            <p className="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900">
              Documentos sintéticos de ensayo; no son textos legales
              productivos.
            </p>
            <DocumentPanel
              document={documents.terms}
              checked={termsAccepted}
              onChange={setTermsAccepted}
              invalid={!termsAccepted && consentIssue}
            />
            <DocumentPanel
              document={documents.privacy}
              checked={privacyAccepted}
              onChange={setPrivacyAccepted}
              invalid={!privacyAccepted && consentIssue}
            />
            {consentIssue && !termsError ? (
              <p id="consent-error" className="text-sm text-red-800">
                Revisa y acepta por separado ambos documentos.
              </p>
            ) : null}
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
            aria-describedby="register-email-help"
            className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 text-gray-900 focus-visible:ring-2 focus-visible:ring-orange-700 sm:text-sm"
          />
          <p id="register-email-help" className="mt-1 text-xs text-gray-600">
            No solicitaremos un rol ni una preferencia de actividad en este
            registro.
          </p>
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
        <AuthFeedback
          message={message}
          kind={feedbackKind}
          focusRef={noticeRef}
        />
        {message.includes("ya tiene una cuenta") ? (
          <div className="flex flex-wrap gap-3 text-sm">
            <Link
              href="/acceso"
              className="min-h-11 inline-flex items-center text-orange-800 underline underline-offset-4"
            >
              Acceder
            </Link>
            <Link
              href="/recuperar-acceso"
              className="min-h-11 inline-flex items-center text-orange-800 underline underline-offset-4"
            >
              Recuperar acceso
            </Link>
          </div>
        ) : null}
        <p className="text-sm text-gray-700">
          También puedes consultar{" "}
          <Link
            href="/terminos-y-condiciones"
            className="text-orange-800 underline underline-offset-4"
          >
            términos
          </Link>{" "}
          y{" "}
          <Link
            href="/politica-de-privacidad"
            className="text-orange-800 underline underline-offset-4"
          >
            privacidad
          </Link>
          .
        </p>
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
        <p className="text-center text-xs text-gray-600">
          La aceptación se registra solo al confirmar este envío.
        </p>
      </form>
    </AuthShell>
  );
}
