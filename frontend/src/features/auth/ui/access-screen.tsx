"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
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
import { authFacade } from "../auth-runtime";
import { useAuth } from "../ui/use-auth";
import { AuthFeedback, AuthShell } from "./auth-shell";
import { PasswordField } from "./password-field";
import { RateLimitNotice } from "./rate-limit-notice";
import { resolveReturnParam } from "../domain/return-target";
import { retryAfterSeconds } from "../domain/retry-after";
import { useRetryRemaining } from "./use-retry-remaining";

export function AccessScreen() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const returnTo = resolveReturnParam(searchParams.getAll("return_to"));
  const { snapshot, login, checkSession } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [retryUntil, setRetryUntil] = useState<number | null>(null);
  const retryRemaining = useRetryRemaining(retryUntil);
  const feedbackRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!message) return;
    feedbackRef.current?.focus();
  }, [message]);

  useEffect(() => {
    let active = true;
    void checkSession().catch((error: unknown) => {
      if (active) setMessage(authErrorMessage(error));
    });
    return () => {
      active = false;
    };
  }, [checkSession]);

  useEffect(() => {
    if (snapshot.status === "authenticated") {
      const isAdmin =
        snapshot.roles.includes("administrador") ||
        snapshot.roles.includes("administrador_global");
      const target =
        returnTo !== "/"
          ? returnTo
          : isAdmin
            ? "/administracion"
            : "/mi-cuenta";
      router.replace(target);
    }
  }, [snapshot.status, snapshot.roles, returnTo, router]);

  const submit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setMessage("");
      setRetryUntil(null);
      setPending(true);
      try {
        const confirmed = await login({ email, password });
        setPassword("");
        if (confirmed) {
          setEmail("");
          const currentSnapshot = authFacade.getSnapshot();
          const isAdmin =
            currentSnapshot.roles.includes("administrador") ||
            currentSnapshot.roles.includes("administrador_global");
          const target =
            returnTo !== "/"
              ? returnTo
              : isAdmin
                ? "/administracion"
                : "/mi-cuenta";
          router.replace(target);
        } else {
          setMessage("No pudimos confirmar el acceso. Intenta nuevamente.");
        }
      } catch (error) {
        setPassword("");
        setMessage(authErrorMessage(error));
        if (error instanceof ApiError && error.status === 429) {
          const seconds = retryAfterSeconds(error.retryAfter);
          setRetryUntil(seconds === null ? null : Date.now() + seconds * 1000);
        }
      } finally {
        setPending(false);
      }
    },
    [email, login, password, returnTo, router],
  );

  return (
    <AuthShell
      title="Acceso a tu cuenta"
      description="Inicia sesión con tu correo y contraseña."
      secondaryQuestion="¿Primera vez en EspaciGo?"
      secondaryLabel="Crear una cuenta"
      secondaryHref="/registro"
    >
      {snapshot.status === "checking" ? (
        <div
          data-testid="access-checking"
          className="flex min-h-[220px] items-center justify-center text-sm text-gray-500"
          aria-live="polite"
        >
          <span className="sr-only">Comprobando sesión…</span>
        </div>
      ) : snapshot.status === "authenticated" ? (
        <div
          data-testid="access-redirecting"
          className="flex min-h-[220px] items-center justify-center text-sm text-gray-500"
          aria-live="polite"
        >
          <span className="sr-only">Redirigiendo…</span>
        </div>
      ) : (
        <form className="space-y-6" onSubmit={submit}>
          <div>
            <Label
              htmlFor="email"
              className="block text-sm font-medium text-gray-700"
            >
              Correo electrónico
            </Label>
            <div className="mt-1">
              <Input
                id="email"
                name="email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                disabled={pending}
                required
                aria-describedby="email-help"
                className="relative block min-h-11 w-full rounded-full border border-gray-300 px-3 py-2 text-gray-900 placeholder:text-gray-500 focus:z-10 focus:border-orange-600 focus:outline-none focus:ring-orange-600 sm:text-sm"
              />
            </div>
            <p id="email-help" className="mt-1 text-xs text-gray-600">
              Usaremos el correo asociado a tu cuenta.
            </p>
          </div>
          <PasswordField
            id="password"
            label="Contraseña"
            autoComplete="current-password"
            value={password}
            onChange={setPassword}
            disabled={pending}
          />
          <div className="flex justify-end">
            <Link
              href="/recuperar-acceso"
              className="inline-flex min-h-11 items-center rounded-md text-sm font-medium text-orange-800 underline underline-offset-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700"
            >
              ¿Olvidaste tu contraseña?
            </Link>
          </div>
          <Button
            type="submit"
            disabled={pending || retryRemaining > 0}
            className="min-h-11 w-full rounded-full bg-orange-700 text-white hover:bg-orange-800 focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
          >
            {pending ? "Conectando…" : "Iniciar sesión"}
          </Button>
          <p className="text-center text-xs leading-relaxed text-gray-600">
            Tu sesión se conserva mientras esté vigente. Puedes cerrarla cuando
            quieras.
          </p>
        </form>
      )}
      <div className="mt-4 space-y-3">
        <AuthFeedback message={message} focusRef={feedbackRef} />
        <RateLimitNotice deadline={retryUntil} />
        {message.includes("Verifica tu correo") ? (
          <Link
            href="/verificar-correo"
            className="inline-flex min-h-11 items-center rounded-md text-sm font-medium text-orange-800 underline underline-offset-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700"
          >
            Solicitar instrucciones de verificación
          </Link>
        ) : null}
      </div>
    </AuthShell>
  );
}
