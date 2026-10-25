"use client";

import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { ApiError, authErrorMessage } from "@/lib/api/errors";
import { Button } from "@/components/workspace/ui/button";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import { createActionCredential } from "../domain/action-credential";
import {
  onboardingErrorCopy,
  recoveryAcceptedCopy,
  reissueAcceptedCopy,
} from "../domain/onboarding-errors";
import { inspectPassword } from "../domain/password-policy";
import { retryAfterSeconds } from "../domain/retry-after";
import { useOnboarding } from "./use-onboarding";
import { useActionCredential } from "./use-action-credential";
import { AuthFeedback, AuthShell } from "./auth-shell";
import { PasswordField, PasswordRequirements } from "./password-field";
import { RateLimitNotice } from "./rate-limit-notice";
import { useRetryRemaining } from "./use-retry-remaining";

export function VerificationScreen() {
  const onboarding = useOnboarding();
  const [tokenId, setTokenId] = useState("");
  const [manualToken, setManualToken] = useState("");
  const [email, setEmail] = useState("");
  const [pending, setPending] = useState(false);
  const [reissuePending, setReissuePending] = useState(false);
  const [message, setMessage] = useState("");
  const [statusMessage, setStatusMessage] = useState(false);
  const [retryUntil, setRetryUntil] = useState<number | null>(null);
  const [ambiguous, setAmbiguous] = useState(false);
  const action = useActionCredential(() => {
    setTokenId("");
    setManualToken("");
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    setAmbiguous(false);
  });
  const retryRemaining = useRetryRemaining(retryUntil);
  const feedbackRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (message) feedbackRef.current?.focus();
  }, [message]);

  async function verify(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    const candidate = action.credential
      ? { ok: true as const, credential: action.credential }
      : createActionCredential(tokenId, manualToken);
    if (!candidate.ok) {
      setMessage(
        "El enlace no es válido o ya no está disponible. Solicita instrucciones nuevas si corresponde.",
      );
      setPending(false);
      return;
    }
    try {
      await onboarding.verifyEmail(candidate.credential);
      action.clear();
      setTokenId("");
      setManualToken("");
      setStatusMessage(true);
      setMessage("Correo verificado. Ya puedes iniciar sesión.");
    } catch (error) {
      if (error instanceof ApiError && error.status === 429) {
        const seconds = retryAfterSeconds(error.retryAfter);
        setRetryUntil(seconds === null ? null : Date.now() + seconds * 1000);
        setMessage(authErrorMessage(error));
      } else if (error instanceof ApiError && error.status < 500) {
        action.clear();
        setTokenId("");
        setManualToken("");
        setMessage(onboardingErrorCopy(error.code));
      } else {
        action.clear();
        setTokenId("");
        setManualToken("");
        setMessage(
          "No recibimos confirmación del enlace. Por seguridad, no lo enviaremos de nuevo automáticamente; solicita instrucciones nuevas.",
        );
        setAmbiguous(true);
      }
    } finally {
      setPending(false);
    }
  }

  async function reissue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setReissuePending(true);
    setMessage("");
    setStatusMessage(false);
    try {
      await onboarding.reissueVerification({ email });
      setStatusMessage(true);
      setMessage(reissueAcceptedCopy());
      setEmail("");
    } catch (error) {
      setMessage(
        error instanceof ApiError && error.status === 429
          ? "No pudimos completar la solicitud ahora. Si corresponde, intentaremos enviar instrucciones."
          : authErrorMessage(error),
      );
    } finally {
      setReissuePending(false);
    }
  }

  return (
    <AuthShell
      title="Verificar correo"
      description="Confirma tu correo para habilitar el acceso a la cuenta."
      secondaryQuestion="¿Ya verificaste tu correo?"
      secondaryLabel="Ir a acceso"
      secondaryHref="/acceso"
    >
      <div className="space-y-6">
        <form className="space-y-6" onSubmit={verify}>
          <p className="text-sm leading-relaxed text-gray-700">
            El enlace se procesa solo cuando confirmas aquí; los escáneres y
            vistas previas no consumen el token automáticamente.
          </p>
          {action.credential ? (
            <p
              role="status"
              className="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900"
            >
              Hay instrucciones disponibles en esta página. El contenido del
              token permanece oculto.
            </p>
          ) : (
            <div className="space-y-4">
              <div>
                <Label
                  htmlFor="verification-token-id"
                  className="block text-sm font-medium text-gray-700"
                >
                  ID del enlace
                </Label>
                <Input
                  id="verification-token-id"
                  type="password"
                  autoComplete="off"
                  spellCheck={false}
                  value={tokenId}
                  onChange={(event) => setTokenId(event.target.value)}
                  disabled={pending}
                  className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 font-mono text-sm"
                />
              </div>
              <div>
                <Label
                  htmlFor="verification-token"
                  className="block text-sm font-medium text-gray-700"
                >
                  Token de verificación
                </Label>
                <Input
                  id="verification-token"
                  type="password"
                  autoComplete="off"
                  spellCheck={false}
                  value={manualToken}
                  onChange={(event) => setManualToken(event.target.value)}
                  disabled={pending}
                  className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 font-mono text-sm"
                />
              </div>
            </div>
          )}
          <Button
            type="submit"
            disabled={pending || ambiguous || retryRemaining > 0}
            className="min-h-11 w-full rounded-full bg-orange-700 text-white hover:bg-orange-800 focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
          >
            {pending ? "Verificando…" : "Confirmar correo"}
          </Button>
        </form>
        <RateLimitNotice deadline={retryUntil} />
        <div className="relative" aria-hidden="true">
          <div className="border-t border-gray-300" />
        </div>
        <form className="space-y-4" onSubmit={reissue}>
          <h2 className="text-lg font-semibold text-gray-900">
            Solicitar nuevas instrucciones
          </h2>
          <p className="text-sm text-gray-700">
            La respuesta no confirma si existe una cuenta ni si se entregará un
            mensaje.
          </p>
          <div>
            <Label
              htmlFor="verification-email"
              className="block text-sm font-medium text-gray-700"
            >
              Correo electrónico
            </Label>
            <Input
              id="verification-email"
              type="email"
              autoComplete="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              required
              disabled={reissuePending}
              className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 text-sm"
            />
          </div>
          <Button
            type="submit"
            variant="outline"
            disabled={reissuePending}
            className="min-h-11 w-full rounded-full"
          >
            {reissuePending ? "Enviando solicitud…" : "Solicitar instrucciones"}
          </Button>
        </form>
        <AuthFeedback
          message={message}
          kind={statusMessage ? "status" : "error"}
          focusRef={feedbackRef}
        />
        <p className="text-sm text-gray-700">
          Si necesitas restablecer tu contraseña, visita{" "}
          <Link
            href="/recuperar-acceso"
            className="text-orange-800 underline underline-offset-4"
          >
            recuperar acceso
          </Link>
          .
        </p>
      </div>
    </AuthShell>
  );
}

export function RecoveryScreen() {
  const onboarding = useOnboarding();
  const [mode, setMode] = useState<"request" | "reset">("request");
  const [email, setEmail] = useState("");
  const [tokenId, setTokenId] = useState("");
  const [manualToken, setManualToken] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [statusMessage, setStatusMessage] = useState(false);
  const [retryUntil, setRetryUntil] = useState<number | null>(null);
  const [ambiguous, setAmbiguous] = useState(false);
  const action = useActionCredential(() => {
    setTokenId("");
    setManualToken("");
    setPassword("");
    setConfirmation("");
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    setAmbiguous(false);
  });
  const retryRemaining = useRetryRemaining(retryUntil);
  const feedbackRef = useRef<HTMLDivElement>(null);
  const activeMode = action.credential ? "reset" : mode;

  useEffect(() => {
    if (message) feedbackRef.current?.focus();
  }, [message]);

  function clearRecoverySecrets() {
    action.clear();
    setTokenId("");
    setManualToken("");
    setPassword("");
    setConfirmation("");
  }

  async function requestRecovery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    try {
      await onboarding.requestRecovery({ email });
      setStatusMessage(true);
      setMessage(recoveryAcceptedCopy());
      setEmail("");
    } catch (error) {
      if (error instanceof ApiError && error.status === 429) {
        const seconds = retryAfterSeconds(error.retryAfter);
        setRetryUntil(seconds === null ? null : Date.now() + seconds * 1000);
        setMessage(authErrorMessage(error));
      } else {
        setMessage(authErrorMessage(error));
      }
    } finally {
      setPending(false);
    }
  }

  async function consumeRecovery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    const candidate = action.credential
      ? { ok: true as const, credential: action.credential }
      : createActionCredential(tokenId, manualToken);
    if (!candidate.ok) {
      setMessage(
        "El enlace no es válido o ya no está disponible. Solicita instrucciones nuevas si corresponde.",
      );
      setPending(false);
      return;
    }
    try {
      await onboarding.consumeRecovery({
        token_id: candidate.credential.tokenId,
        token: candidate.credential.token,
        new_password: password,
        confirm_password: confirmation,
      });
      clearRecoverySecrets();
      setStatusMessage(true);
      setMode("request");
      setMessage(
        "La contraseña se actualizó y se revocaron las sesiones activas. Inicia sesión nuevamente.",
      );
    } catch (error) {
      setPassword("");
      setConfirmation("");
      if (error instanceof ApiError && error.status === 429) {
        const seconds = retryAfterSeconds(error.retryAfter);
        setRetryUntil(seconds === null ? null : Date.now() + seconds * 1000);
        setMessage(authErrorMessage(error));
      } else if (error instanceof ApiError && error.status < 500) {
        setMessage(onboardingErrorCopy(error.code));
        if (error.code === "invalid_token") {
          clearRecoverySecrets();
        }
      } else {
        clearRecoverySecrets();
        setMessage(
          "No recibimos confirmación del cambio. Por seguridad, no reenviaremos el token automáticamente; solicita nuevas instrucciones antes de continuar.",
        );
        setAmbiguous(true);
      }
    } finally {
      setPending(false);
    }
  }

  const validPassword = inspectPassword(password).valid;
  const credentialsReady = Boolean(
    action.credential || (tokenId && manualToken),
  );

  return (
    <AuthShell
      title="Recuperar acceso"
      description="Solicita instrucciones o establece una contraseña nueva desde un enlace válido."
      secondaryQuestion="¿Recordaste tu contraseña?"
      secondaryLabel="Ir a acceso"
      secondaryHref="/acceso"
    >
      <div className="space-y-6">
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant={activeMode === "reset" ? "default" : "outline"}
            onClick={() => setMode("reset")}
            className="min-h-11 rounded-full"
          >
            Tengo un enlace
          </Button>
          <Button
            type="button"
            variant={activeMode === "request" ? "default" : "outline"}
            onClick={() => {
              clearRecoverySecrets();
              setMode("request");
              setAmbiguous(false);
            }}
            className="min-h-11 rounded-full"
          >
            Solicitar instrucciones
          </Button>
        </div>
        {activeMode === "request" ? (
          <form className="space-y-6" onSubmit={requestRecovery}>
            <div>
              <Label
                htmlFor="recovery-email"
                className="block text-sm font-medium text-gray-700"
              >
                Correo electrónico
              </Label>
              <Input
                id="recovery-email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
                disabled={pending}
                className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 text-sm"
              />
            </div>
            <Button
              type="submit"
              disabled={pending || retryRemaining > 0}
              className="min-h-11 w-full rounded-full bg-orange-700 text-white hover:bg-orange-800 focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
            >
              {pending ? "Enviando solicitud…" : "Solicitar instrucciones"}
            </Button>
            <RateLimitNotice deadline={retryUntil} />
          </form>
        ) : (
          <form className="space-y-6" onSubmit={consumeRecovery}>
            <p className="text-sm text-gray-700">
              La contraseña se cambiará solo después de confirmar. No
              mostraremos ni guardaremos el token fuera de esta página.
            </p>
            {!action.credential ? (
              <div className="space-y-4">
                <div>
                  <Label
                    htmlFor="recovery-token-id"
                    className="block text-sm font-medium text-gray-700"
                  >
                    ID del enlace
                  </Label>
                  <Input
                    id="recovery-token-id"
                    type="password"
                    autoComplete="off"
                    spellCheck={false}
                    value={tokenId}
                    onChange={(event) => setTokenId(event.target.value)}
                    disabled={pending}
                    className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 font-mono text-sm"
                  />
                </div>
                <div>
                  <Label
                    htmlFor="recovery-token"
                    className="block text-sm font-medium text-gray-700"
                  >
                    Token de recuperación
                  </Label>
                  <Input
                    id="recovery-token"
                    type="password"
                    autoComplete="off"
                    spellCheck={false}
                    value={manualToken}
                    onChange={(event) => setManualToken(event.target.value)}
                    disabled={pending}
                    className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 font-mono text-sm"
                  />
                </div>
              </div>
            ) : (
              <p
                role="status"
                className="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900"
              >
                Hay instrucciones disponibles en esta página. El contenido del
                token permanece oculto.
              </p>
            )}
            <div className="space-y-2">
              <PasswordField
                id="recovery-password"
                label="Nueva contraseña"
                autoComplete="new-password"
                value={password}
                onChange={setPassword}
                disabled={pending}
                invalid={Boolean(password) && !validPassword}
                describedBy="password-requirements"
              />
              <PasswordRequirements value={password} />
            </div>
            <div>
              <Label
                htmlFor="recovery-confirmation"
                className="block text-sm font-medium text-gray-700"
              >
                Confirmar nueva contraseña
              </Label>
              <Input
                id="recovery-confirmation"
                type="password"
                autoComplete="new-password"
                value={confirmation}
                onChange={(event) => setConfirmation(event.target.value)}
                required
                disabled={pending}
                aria-invalid={
                  (Boolean(confirmation) && confirmation !== password) ||
                  undefined
                }
                className="mt-1 min-h-11 rounded-full border border-gray-300 px-3 text-sm"
              />
            </div>
            <Button
              type="submit"
              disabled={
                pending ||
                ambiguous ||
                !credentialsReady ||
                !validPassword ||
                confirmation !== password ||
                retryRemaining > 0
              }
              className="min-h-11 w-full rounded-full bg-orange-700 text-white hover:bg-orange-800 focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
            >
              {pending ? "Actualizando contraseña…" : "Actualizar contraseña"}
            </Button>
            <RateLimitNotice deadline={retryUntil} />
          </form>
        )}
        <AuthFeedback
          message={message}
          kind={statusMessage ? "status" : "error"}
          focusRef={feedbackRef}
        />
        {message && !statusMessage ? (
          <p className="text-sm text-gray-700">
            Si no puedes usar el enlace,{" "}
            <Link
              href="/recuperar-acceso"
              className="text-orange-800 underline underline-offset-4"
              onClick={() => {
                clearRecoverySecrets();
                setMode("request");
                setAmbiguous(false);
              }}
            >
              solicita instrucciones nuevas
            </Link>
            .
          </p>
        ) : null}
      </div>
    </AuthShell>
  );
}
