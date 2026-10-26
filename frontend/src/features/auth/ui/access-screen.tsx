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

function formatRoleBadge(role: string): string {
  switch (role) {
    case "administrador_global":
      return "Administrador Global";
    case "administrador":
      return "Administrador";
    case "arrendador":
      return "Anfitrión / Arrendador";
    case "arrendatario":
      return "Arrendatario";
    default:
      return role;
  }
}

export function AccessScreen() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const returnTo = resolveReturnParam(searchParams.getAll("return_to"));
  const { snapshot, login, checkSession, logout } = useAuth();
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

  async function signOut() {
    setMessage("");
    setPending(true);
    try {
      await logout();
      setMessage("Sesión cerrada.");
    } catch (error) {
      setMessage(authErrorMessage(error));
    } finally {
      setPending(false);
    }
  }

  return (
    <AuthShell
      title="Acceso a tu cuenta"
      description="Inicia sesión con tu correo y contraseña."
      secondaryQuestion="¿Primera vez en EspaciGo?"
      secondaryLabel="Crear una cuenta"
      secondaryHref="/registro"
    >
      {snapshot.status === "authenticated" ? (
        <section
          aria-label="Estado de sesión activa"
          className="space-y-5 rounded-2xl border border-orange-200 bg-orange-50/50 p-6 text-center shadow-sm"
        >
          <div className="space-y-1">
            <p className="text-lg font-semibold text-gray-950">Sesión activa</p>
            <p className="text-xs text-muted-foreground">
              Tienes una sesión iniciada en este navegador.
            </p>
          </div>

          {snapshot.roles.length > 0 ? (
            <div className="flex flex-wrap items-center justify-center gap-1.5 pt-1">
              {snapshot.roles.map((role) => (
                <span
                  key={role}
                  className="inline-flex items-center rounded-full bg-orange-100 px-3 py-1 text-xs font-semibold uppercase tracking-wider text-orange-900 border border-orange-200"
                >
                  {formatRoleBadge(role)}
                </span>
              ))}
            </div>
          ) : null}

          {snapshot.restrictedMode ? (
            <p className="text-xs text-amber-800">
              La cuenta tiene acceso restringido a operaciones permitidas por el
              servicio.
            </p>
          ) : null}

          <div className="space-y-2.5 pt-2">
            {snapshot.roles.includes("administrador") ||
            snapshot.roles.includes("administrador_global") ? (
              <Button
                asChild
                className="min-h-11 w-full rounded-full bg-orange-700 text-white hover:bg-orange-800 font-medium"
              >
                <Link href="/administracion">
                  Ingresar al Panel de Administración
                </Link>
              </Button>
            ) : null}

            {returnTo !== "/" && returnTo !== "/administracion" ? (
              <Button
                asChild
                className="min-h-11 w-full rounded-full bg-orange-700 text-white hover:bg-orange-800 font-medium"
              >
                <Link href={returnTo}>Continuar a {returnTo}</Link>
              </Button>
            ) : null}

            <div className="flex flex-col sm:flex-row gap-2">
              <Button
                asChild
                variant="outline"
                className="min-h-11 flex-1 rounded-full border-gray-300"
              >
                <Link href="/mi-cuenta">Mi Cuenta</Link>
              </Button>
              {snapshot.roles.includes("arrendador") ? (
                <Button
                  asChild
                  variant="outline"
                  className="min-h-11 flex-1 rounded-full border-gray-300"
                >
                  <Link href="/anfitrion/espacios">Mis Espacios</Link>
                </Button>
              ) : null}
            </div>

            <Button
              type="button"
              variant="ghost"
              disabled={pending}
              onClick={() => void signOut()}
              className="min-h-11 w-full rounded-full text-gray-600 hover:text-gray-900 hover:bg-gray-100"
            >
              Cerrar sesión
            </Button>
          </div>
        </section>
      ) : null}

      <div
        className={
          snapshot.status === "authenticated"
            ? "mt-8 border-t border-gray-200 pt-6"
            : ""
        }
      >
        {snapshot.status === "authenticated" ? (
          <p className="mb-4 text-center text-sm font-medium text-gray-700">
            O inicia sesión con una cuenta diferente:
          </p>
        ) : null}
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
            La sesión se conserva solo en la memoria de esta pestaña y se
            confirma con el servicio.
          </p>
        </form>
      </div>
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
