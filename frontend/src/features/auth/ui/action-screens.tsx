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
import { ApiError, authErrorMessage } from "@/lib/api/errors";
import { Button } from "@/components/workspace/ui/button";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import {
  clearVerificationCompleted,
  markVerificationCompleted,
} from "../application/verification-result";
import type { ActionLinkStatus } from "../application/onboarding.port";
import {
  onboardingErrorCopy,
  recoveryAcceptedCopy,
  reissueAcceptedCopy,
} from "../domain/onboarding-errors";
import { inspectPassword } from "../domain/password-policy";
import { retryAfterSeconds } from "../domain/retry-after";
import { useOnboarding } from "./use-onboarding";
import { useActionCredential } from "./use-action-credential";
import {
  useActionLinkStatus,
  type ActionTerminalOutcome,
} from "./use-action-link-status";
import { AuthFeedback, AuthShell } from "./auth-shell";
import { PasswordField, PasswordRequirements } from "./password-field";
import { RateLimitNotice } from "./rate-limit-notice";
import { useRetryRemaining } from "./use-retry-remaining";

export function VerificationScreen() {
  const router = useRouter();
  const onboarding = useOnboarding();
  const [email, setEmail] = useState("");
  const [pending, setPending] = useState(false);
  const [reissuePending, setReissuePending] = useState(false);
  const [message, setMessage] = useState("");
  const [statusMessage, setStatusMessage] = useState(false);
  const [retryUntil, setRetryUntil] = useState<number | null>(null);
  const [ambiguous, setAmbiguous] = useState(false);
  const [isInvalidToken, setIsInvalidToken] = useState(false);
  const [outcome, setOutcome] = useState<ActionTerminalOutcome | null>(null);
  const isMountedRef = useRef(true);
  const operationEpochRef = useRef(0);

  const probeResetRef = useRef<() => void>(() => {});

  const action = useActionCredential(() => {
    operationEpochRef.current += 1;
    clearVerificationCompleted();
    onboarding.cancel();
    setOutcome(null);
    probeResetRef.current();
    setPending(false);
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    setAmbiguous(false);
    setIsInvalidToken(false);
  });

  const handleTerminalStatus = useCallback(
    (status: ActionTerminalOutcome) => {
      action.clear();
      setOutcome(status);
    },
    [action],
  );

  const probe = useActionLinkStatus({
    credential: action.credential,
    inspect: (cred) => onboarding.inspectVerification(cred),
    onTerminalStatus: handleTerminalStatus,
  });

  useEffect(() => {
    probeResetRef.current = probe.reset;
  }, [probe.reset]);

  const retryRemaining = useRetryRemaining(retryUntil);
  const feedbackRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    isMountedRef.current = true;
    clearVerificationCompleted();
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (message) feedbackRef.current?.focus();
  }, [message]);

  const handleRequestNewLink = () => {
    action.clear();
    setOutcome(null);
    probe.reset();
    setMessage("");
    setStatusMessage(false);
    setIsInvalidToken(false);
  };

  async function verify(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const activeRawCredential = action.credential;
    if (!activeRawCredential || !probe.isReady) {
      setMessage(
        "El enlace no es válido o ya no está disponible. Abre el enlace que enviamos a tu correo o solicita uno nuevo.",
      );
      return;
    }

    const currentEpoch = ++operationEpochRef.current;
    clearVerificationCompleted();
    setPending(true);
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    setAmbiguous(false);
    setIsInvalidToken(false);
    try {
      await onboarding.verifyEmail(activeRawCredential);
      if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
        return;
      action.clear();
      markVerificationCompleted();
      router.replace("/correo-verificado");
    } catch (error) {
      if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
        return;
      if (error instanceof ApiError && error.status === 429) {
        const seconds = retryAfterSeconds(error.retryAfter);
        setRetryUntil(seconds === null ? null : Date.now() + seconds * 1000);
        setMessage(authErrorMessage(error));
      } else if (error instanceof ApiError && error.code === "invalid_token") {
        let recheckStatus: ActionLinkStatus | null = null;
        try {
          recheckStatus =
            await onboarding.inspectVerification(activeRawCredential);
        } catch {
          // probe error ignored
        }
        if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
          return;
        action.clear();
        if (recheckStatus === "used") {
          setOutcome("used");
        } else if (recheckStatus === "expired") {
          setOutcome("expired");
        } else if (recheckStatus === "invalid") {
          setOutcome("invalid");
        } else {
          setIsInvalidToken(true);
          setMessage(onboardingErrorCopy(error.code, "verification"));
        }
      } else if (error instanceof ApiError && error.status < 500) {
        action.clear();
        setMessage(onboardingErrorCopy(error.code, "verification"));
      } else {
        action.clear();
        setMessage(
          "No pudimos confirmar la respuesta. La verificación puede haberse completado. Intenta iniciar sesión; si tu correo sigue pendiente, solicita un nuevo enlace.",
        );
        setAmbiguous(true);
      }
    } finally {
      if (isMountedRef.current && operationEpochRef.current === currentEpoch) {
        setPending(false);
      }
    }
  }

  async function reissue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedEmail = email.trim();
    if (!trimmedEmail) {
      setMessage("Ingresa tu correo electrónico para continuar.");
      return;
    }
    const currentEpoch = ++operationEpochRef.current;
    setReissuePending(true);
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    setAmbiguous(false);
    setIsInvalidToken(false);
    try {
      await onboarding.reissueVerification({ email: trimmedEmail });
      if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
        return;
      setStatusMessage(true);
      setMessage(reissueAcceptedCopy());
      setEmail("");
    } catch (error) {
      if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
        return;
      if (error instanceof ApiError && error.status === 429) {
        const seconds = retryAfterSeconds(error.retryAfter);
        setRetryUntil(seconds === null ? null : Date.now() + seconds * 1000);
        setMessage(authErrorMessage(error));
      } else if (error instanceof ApiError && error.status < 500) {
        setMessage(onboardingErrorCopy(error.code));
      } else {
        setMessage(
          "No pudimos confirmar la solicitud del enlace. Por seguridad, no lo reintentaremos automáticamente; intenta de nuevo más tarde.",
        );
        setAmbiguous(true);
      }
    } finally {
      if (isMountedRef.current && operationEpochRef.current === currentEpoch) {
        setReissuePending(false);
      }
    }
  }

  if (outcome === "used") {
    return (
      <AuthShell
        title="Verificación ya realizada"
        description="El correo electrónico ya fue confirmado con este enlace."
        secondaryQuestion="¿Quieres seguir explorando?"
        secondaryLabel="Ver espacios"
        secondaryHref="/espacios"
      >
        <div className="space-y-4">
          <p className="text-sm leading-relaxed text-gray-700">
            Inicia sesión para ingresar a tu cuenta de EspaciGo.
          </p>
          <Link
            href="/acceso"
            className="inline-flex min-h-11 w-full items-center justify-center rounded-full bg-orange-700 px-4 py-2 text-center text-sm font-semibold text-white shadow-xs hover:bg-orange-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
          >
            Ir a acceso
          </Link>
          <Button
            type="button"
            onClick={handleRequestNewLink}
            className="min-h-11 w-full rounded-full border border-gray-300 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 shadow-xs hover:bg-gray-50"
          >
            Solicitar nuevo enlace
          </Button>
        </div>
      </AuthShell>
    );
  }

  if (outcome === "expired") {
    return (
      <AuthShell
        title="Enlace vencido"
        description="El enlace de verificación ha expirado. Solicita instrucciones nuevas para continuar."
        secondaryQuestion="¿Ya confirmaste?"
        secondaryLabel="Ir a acceso"
        secondaryHref="/acceso"
      >
        <div className="space-y-4">
          <p className="text-sm leading-relaxed text-gray-700">
            El enlace ha vencido por límite de tiempo. Para verificar tu cuenta,
            solicita un nuevo enlace a tu correo.
          </p>
          <Button
            type="button"
            onClick={handleRequestNewLink}
            className="min-h-11 w-full rounded-full bg-orange-700 px-4 py-3 text-sm font-semibold text-white shadow-xs hover:bg-orange-800"
          >
            Solicitar nuevo enlace
          </Button>
        </div>
      </AuthShell>
    );
  }

  if (outcome === "invalid") {
    return (
      <AuthShell
        title="Enlace no válido"
        description="El enlace no es válido o ya no está disponible. Si ya confirmaste tu correo, inicia sesión. Si aún no, solicita un nuevo enlace."
        secondaryQuestion="¿Ya confirmaste?"
        secondaryLabel="Ir a acceso"
        secondaryHref="/acceso"
      >
        <div className="space-y-4">
          <p className="text-sm leading-relaxed text-gray-700">
            El enlace no es válido o ya no está disponible. Si ya confirmaste tu
            correo, inicia sesión. Si aún no, solicita un nuevo enlace.
          </p>
          <Link
            href="/acceso"
            className="inline-flex min-h-11 w-full items-center justify-center rounded-full bg-orange-700 px-4 py-2 text-center text-sm font-semibold text-white shadow-xs hover:bg-orange-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
          >
            Comprobar acceso
          </Link>
          <Button
            type="button"
            onClick={handleRequestNewLink}
            className="min-h-11 w-full rounded-full border border-gray-300 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 shadow-xs hover:bg-gray-50"
          >
            Solicitar nuevo enlace
          </Button>
        </div>
      </AuthShell>
    );
  }

  return (
    <AuthShell
      title="Verificación de correo"
      description="Confirma tu dirección para activar tu cuenta de EspaciGo."
      secondaryQuestion="¿Ya confirmaste?"
      secondaryLabel="Ir a acceso"
      secondaryHref="/acceso"
    >
      <div className="space-y-6">
        <AuthFeedback
          focusRef={feedbackRef}
          message={message}
          kind={statusMessage ? "status" : "error"}
        />
        <RateLimitNotice deadline={retryUntil} />
        {ambiguous || isInvalidToken ? (
          <div>
            <Link
              href="/acceso"
              className="inline-flex min-h-11 w-full items-center justify-center rounded-full bg-orange-700 px-4 py-2 text-center text-sm font-semibold text-white shadow-xs hover:bg-orange-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
            >
              Comprobar acceso
            </Link>
          </div>
        ) : null}

        {probe.isChecking ? (
          <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-4 text-center text-sm text-gray-700">
            <p className="font-medium">Comprobando enlace...</p>
          </div>
        ) : probe.isProbeError ? (
          <div className="space-y-4">
            <div
              className="rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-900"
              role="alert"
            >
              No fue posible comprobar el estado del enlace. Intenta nuevamente.
            </div>
            <Button
              type="button"
              onClick={probe.retry}
              className="min-h-11 w-full rounded-full bg-orange-700 px-4 py-3 text-sm font-semibold text-white shadow-xs hover:bg-orange-800"
            >
              Reintentar comprobación
            </Button>
          </div>
        ) : probe.isReady && action.credential ? (
          <div className="space-y-4">
            <p className="text-sm leading-relaxed text-gray-700">
              Presiona el botón a continuación para completar la activación de
              tu cuenta.
            </p>
            <form onSubmit={verify} className="space-y-4" noValidate>
              <Button
                type="submit"
                disabled={pending || retryRemaining > 0}
                className="min-h-11 w-full rounded-full bg-orange-700 px-4 py-3 text-sm font-semibold text-white shadow-xs hover:bg-orange-800 disabled:opacity-50"
              >
                {pending ? "Confirmando..." : "Verificar mi correo"}
              </Button>
            </form>
          </div>
        ) : (
          <div className="space-y-6">
            <div className="space-y-2">
              <p className="text-sm leading-relaxed text-gray-700">
                Abre el enlace que enviamos a tu correo para activar tu cuenta.
              </p>
              <p className="text-xs text-gray-500">
                Si no recibiste el correo o el enlace venció, solicita uno nuevo
                a continuación.
              </p>
            </div>

            <section className="rounded-2xl border border-gray-200 bg-white p-4 space-y-3">
              <h2 className="text-sm font-semibold text-gray-900">
                Solicitar nuevo enlace
              </h2>
              <form onSubmit={reissue} className="space-y-3">
                <div className="space-y-1.5">
                  <Label
                    htmlFor="reissue-email"
                    className="text-xs font-semibold uppercase tracking-wider text-gray-500"
                  >
                    Correo electrónico
                  </Label>
                  <Input
                    id="reissue-email"
                    type="email"
                    value={email}
                    onChange={(event) => setEmail(event.target.value.trim())}
                    placeholder="ejemplo@dominio.com"
                    autoComplete="email"
                    required
                    disabled={reissuePending}
                  />
                </div>
                <Button
                  type="submit"
                  disabled={reissuePending || retryRemaining > 0}
                  className="min-h-11 w-full rounded-full border border-gray-300 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 shadow-xs hover:bg-gray-50 disabled:opacity-50"
                >
                  {reissuePending
                    ? "Solicitando..."
                    : "Solicitar instrucciones"}
                </Button>
              </form>
            </section>
          </div>
        )}

        <p className="text-center text-xs text-gray-500">
          ¿Deseas volver?{" "}
          <Link
            href="/acceso"
            className="font-medium text-orange-700 underline"
          >
            Ir a acceso
          </Link>
          .
        </p>
      </div>
    </AuthShell>
  );
}

export function RecoveryScreen() {
  const onboarding = useOnboarding();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [statusMessage, setStatusMessage] = useState(false);
  const [retryUntil, setRetryUntil] = useState<number | null>(null);
  const [ambiguous, setAmbiguous] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [outcome, setOutcome] = useState<ActionTerminalOutcome | null>(null);
  const isMountedRef = useRef(true);
  const operationEpochRef = useRef(0);

  const probeResetRef = useRef<() => void>(() => {});

  const action = useActionCredential(() => {
    operationEpochRef.current += 1;
    onboarding.cancel();
    setOutcome(null);
    probeResetRef.current();
    setIsSuccess(false);
    setPending(false);
    setPassword("");
    setConfirmation("");
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    setAmbiguous(false);
  });

  const handleTerminalStatus = useCallback(
    (status: ActionTerminalOutcome) => {
      action.clear();
      setPassword("");
      setConfirmation("");
      setOutcome(status);
    },
    [action],
  );

  const probe = useActionLinkStatus({
    credential: action.credential,
    inspect: (cred) => onboarding.inspectRecovery(cred),
    onTerminalStatus: handleTerminalStatus,
  });

  useEffect(() => {
    probeResetRef.current = probe.reset;
  }, [probe.reset]);

  const retryRemaining = useRetryRemaining(retryUntil);
  const feedbackRef = useRef<HTMLDivElement>(null);
  const activeCredential = action.credential;

  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (message) feedbackRef.current?.focus();
  }, [message]);

  const handleRequestNewLink = () => {
    action.clear();
    setOutcome(null);
    probe.reset();
    setPassword("");
    setConfirmation("");
    setMessage("");
    setStatusMessage(false);
  };

  async function requestRecovery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedEmail = email.trim();
    if (!trimmedEmail) {
      setMessage("Ingresa tu correo electrónico para continuar.");
      return;
    }
    const currentEpoch = ++operationEpochRef.current;
    setPending(true);
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    setAmbiguous(false);
    try {
      await onboarding.requestRecovery({ email: trimmedEmail });
      if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
        return;
      setStatusMessage(true);
      setMessage(recoveryAcceptedCopy());
      setEmail("");
    } catch (error) {
      if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
        return;
      if (error instanceof ApiError && error.status === 429) {
        const seconds = retryAfterSeconds(error.retryAfter);
        setRetryUntil(seconds === null ? null : Date.now() + seconds * 1000);
        setMessage(authErrorMessage(error));
      } else if (error instanceof ApiError && error.status < 500) {
        setMessage(onboardingErrorCopy(error.code));
      } else {
        setMessage(
          "No recibimos confirmación de la solicitud. Por seguridad, no la reintentaremos automáticamente; intenta de nuevo más tarde.",
        );
        setAmbiguous(true);
      }
    } finally {
      if (isMountedRef.current && operationEpochRef.current === currentEpoch) {
        setPending(false);
      }
    }
  }

  async function consumeRecovery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!activeCredential || !probe.isReady) {
      setMessage(
        "El enlace no es válido o ya no está disponible. Solicita instrucciones nuevas si corresponde.",
      );
      return;
    }
    const policy = inspectPassword(password);
    if (!policy.valid) {
      setMessage("La contraseña no cumple la política mínima de seguridad.");
      return;
    }
    if (password !== confirmation) {
      setMessage("La confirmación de la contraseña no coincide.");
      return;
    }

    const currentEpoch = ++operationEpochRef.current;
    setPending(true);
    setMessage("");
    setStatusMessage(false);
    setRetryUntil(null);
    setAmbiguous(false);

    try {
      await onboarding.consumeRecovery({
        token_id: activeCredential.tokenId,
        token: activeCredential.token,
        new_password: password,
        confirm_password: confirmation,
      });
      if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
        return;
      action.clear();
      setPassword("");
      setConfirmation("");
      setIsSuccess(true);
    } catch (error) {
      if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
        return;
      setPassword("");
      setConfirmation("");
      if (error instanceof ApiError && error.status === 429) {
        const seconds = retryAfterSeconds(error.retryAfter);
        setRetryUntil(seconds === null ? null : Date.now() + seconds * 1000);
        setMessage(authErrorMessage(error));
      } else if (error instanceof ApiError && error.code === "invalid_token") {
        let recheckStatus: ActionLinkStatus | null = null;
        try {
          recheckStatus = await onboarding.inspectRecovery(activeCredential);
        } catch {
          // probe error ignored
        }
        if (!isMountedRef.current || operationEpochRef.current !== currentEpoch)
          return;
        action.clear();
        if (recheckStatus === "used") {
          setOutcome("used");
        } else if (recheckStatus === "expired") {
          setOutcome("expired");
        } else if (recheckStatus === "invalid") {
          setOutcome("invalid");
        } else {
          setMessage(onboardingErrorCopy(error.code));
        }
      } else if (error instanceof ApiError && error.status < 500) {
        action.clear();
        setMessage(onboardingErrorCopy(error.code));
      } else {
        action.clear();
        setMessage(
          "No pudimos confirmar el cambio de contraseña. Por seguridad, no lo reintentaremos automáticamente; solicita instrucciones nuevas si lo requieres.",
        );
        setAmbiguous(true);
      }
    } finally {
      if (isMountedRef.current && operationEpochRef.current === currentEpoch) {
        setPending(false);
      }
    }
  }

  const recoveryPolicy = inspectPassword(password);
  const canSubmitRecovery =
    Boolean(activeCredential) &&
    probe.isReady &&
    !pending &&
    retryRemaining === 0 &&
    recoveryPolicy.valid &&
    password === confirmation &&
    confirmation.length > 0;

  if (isSuccess) {
    return (
      <AuthShell
        title="Contraseña actualizada"
        description="Tu contraseña ha sido restablecida exitosamente. Ya puedes iniciar sesión con tu nueva clave."
        secondaryQuestion="¿Listo para ingresar?"
        secondaryLabel="Ir a acceso"
        secondaryHref="/acceso"
      >
        <div className="space-y-6">
          <div className="rounded-2xl border border-green-200 bg-green-50/70 p-4 text-sm leading-relaxed text-green-900">
            <p className="font-semibold">¡Cambio exitoso!</p>
            <p className="mt-1 text-green-800">
              Tu contraseña ha sido actualizada correctamente. Inicia sesión
              para continuar en EspaciGo.
            </p>
          </div>
          <Link
            href="/acceso"
            className="inline-flex min-h-11 w-full items-center justify-center rounded-full bg-orange-700 px-4 py-2 text-center text-sm font-semibold text-white shadow-xs hover:bg-orange-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
          >
            Ir a acceso
          </Link>
        </div>
      </AuthShell>
    );
  }

  if (outcome === "used") {
    return (
      <AuthShell
        title="Enlace de recuperación ya utilizado"
        description="La contraseña ya fue cambiada con este enlace."
        secondaryQuestion="¿Quieres seguir explorando?"
        secondaryLabel="Ver espacios"
        secondaryHref="/espacios"
      >
        <div className="space-y-4">
          <p className="text-sm leading-relaxed text-gray-700">
            Si necesitas recuperar el acceso a tu cuenta, solicita un nuevo
            enlace a tu correo o inicia sesión.
          </p>
          <Link
            href="/acceso"
            className="inline-flex min-h-11 w-full items-center justify-center rounded-full bg-orange-700 px-4 py-2 text-center text-sm font-semibold text-white shadow-xs hover:bg-orange-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
          >
            Ir a acceso
          </Link>
          <Button
            type="button"
            onClick={handleRequestNewLink}
            className="min-h-11 w-full rounded-full border border-gray-300 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 shadow-xs hover:bg-gray-50"
          >
            Solicitar nuevo enlace
          </Button>
        </div>
      </AuthShell>
    );
  }

  if (outcome === "expired") {
    return (
      <AuthShell
        title="Enlace vencido"
        description="El enlace de recuperación ha expirado. Solicita instrucciones nuevas para continuar."
        secondaryQuestion="¿Recordaste tu contraseña?"
        secondaryLabel="Ir a acceso"
        secondaryHref="/acceso"
      >
        <div className="space-y-4">
          <p className="text-sm leading-relaxed text-gray-700">
            El enlace ha vencido por límite de tiempo. Para restablecer tu
            contraseña, solicita un nuevo enlace a tu correo.
          </p>
          <Button
            type="button"
            onClick={handleRequestNewLink}
            className="min-h-11 w-full rounded-full bg-orange-700 px-4 py-3 text-sm font-semibold text-white shadow-xs hover:bg-orange-800"
          >
            Solicitar nuevo enlace
          </Button>
        </div>
      </AuthShell>
    );
  }

  if (outcome === "invalid") {
    return (
      <AuthShell
        title="Enlace no válido"
        description="El enlace no es válido o ya no está disponible. Solicita instrucciones nuevas si corresponde."
        secondaryQuestion="¿Recordaste tu contraseña?"
        secondaryLabel="Ir a acceso"
        secondaryHref="/acceso"
      >
        <div className="space-y-4">
          <p className="text-sm leading-relaxed text-gray-700">
            El enlace no es válido o ya no está disponible. Solicita
            instrucciones nuevas para restablecer tu contraseña.
          </p>
          <Button
            type="button"
            onClick={handleRequestNewLink}
            className="min-h-11 w-full rounded-full bg-orange-700 px-4 py-3 text-sm font-semibold text-white shadow-xs hover:bg-orange-800"
          >
            Solicitar nuevo enlace
          </Button>
        </div>
      </AuthShell>
    );
  }

  return (
    <AuthShell
      title="Recuperar acceso"
      description="Restablece tu contraseña mediante el enlace seguro que enviamos a tu correo."
      secondaryQuestion="¿Recordaste tu contraseña?"
      secondaryLabel="Ir a acceso"
      secondaryHref="/acceso"
    >
      <div className="space-y-6">
        <AuthFeedback
          focusRef={feedbackRef}
          message={message}
          kind={statusMessage ? "status" : "error"}
        />
        <RateLimitNotice deadline={retryUntil} />
        {ambiguous ? (
          <p className="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs leading-relaxed text-amber-900">
            La operación no pudo confirmarse con certeza. No la reintentaremos
            automáticamente; ingresa de nuevo los datos cuando desees continuar.
          </p>
        ) : null}

        {probe.isChecking ? (
          <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-4 text-center text-sm text-gray-700">
            <p className="font-medium">Comprobando enlace...</p>
          </div>
        ) : probe.isProbeError ? (
          <div className="space-y-4">
            <div
              className="rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-900"
              role="alert"
            >
              No fue posible comprobar el estado del enlace. Intenta nuevamente.
            </div>
            <Button
              type="button"
              onClick={probe.retry}
              className="min-h-11 w-full rounded-full bg-orange-700 px-4 py-3 text-sm font-semibold text-white shadow-xs hover:bg-orange-800"
            >
              Reintentar comprobación
            </Button>
          </div>
        ) : probe.isReady && activeCredential ? (
          <form onSubmit={consumeRecovery} className="space-y-4" noValidate>
            <PasswordField
              id="recovery-new-password"
              label="Nueva contraseña"
              value={password}
              onChange={setPassword}
              autoComplete="new-password"
              disabled={pending}
            />
            <PasswordRequirements value={password} />
            <PasswordField
              id="recovery-confirm-password"
              label="Confirmar nueva contraseña"
              value={confirmation}
              onChange={setConfirmation}
              autoComplete="new-password"
              disabled={pending}
            />
            <Button
              type="submit"
              disabled={!canSubmitRecovery}
              className="min-h-11 w-full rounded-full bg-orange-700 px-4 py-3 text-sm font-semibold text-white shadow-xs hover:bg-orange-800 disabled:opacity-50"
            >
              {pending ? "Actualizando..." : "Actualizar contraseña"}
            </Button>
          </form>
        ) : (
          <form onSubmit={requestRecovery} className="space-y-4">
            <div className="space-y-1.5">
              <Label
                htmlFor="recovery-email"
                className="text-xs font-semibold uppercase tracking-wider text-gray-500"
              >
                Correo electrónico
              </Label>
              <Input
                id="recovery-email"
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value.trim())}
                placeholder="ejemplo@dominio.com"
                autoComplete="email"
                required
                disabled={pending}
              />
            </div>
            <Button
              type="submit"
              disabled={pending || retryRemaining > 0}
              className="min-h-11 w-full rounded-full bg-orange-700 px-4 py-3 text-sm font-semibold text-white shadow-xs hover:bg-orange-800 disabled:opacity-50"
            >
              {pending ? "Solicitando..." : "Solicitar instrucciones"}
            </Button>
          </form>
        )}

        <p className="text-center text-xs text-gray-500">
          ¿Recordaste tu contraseña?{" "}
          <Link
            href="/acceso"
            className="font-medium text-orange-700 underline"
          >
            Ir a acceso
          </Link>
          .
        </p>
      </div>
    </AuthShell>
  );
}
