"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { Button } from "@/components/workspace/ui/button";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import { authErrorMessage } from "@/lib/api/errors";
import { useAuth } from "@/features/auth/ui/use-auth";
import { CircleIcon } from "lucide-react";

export function Login() {
  const { snapshot, login, checkSession, logout } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    void checkSession().catch((error: unknown) => {
      setMessage(authErrorMessage(error));
    });
  }, [checkSession]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage("");
    setPending(true);
    try {
      const accepted = await login({ email, password });
      if (accepted) {
        setEmail("");
      } else {
        setMessage("La operación fue reemplazada por otra sesión.");
      }
    } catch (error) {
      setMessage(authErrorMessage(error));
    } finally {
      setPassword("");
      setPending(false);
    }
  }

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
    <main className="min-h-[100dvh] flex flex-col justify-center py-12 px-4 sm:px-6 lg:px-8 bg-gray-50">
      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        <div className="flex justify-center">
          <CircleIcon
            className="h-12 w-12 text-orange-500"
            aria-hidden="true"
          />
        </div>
        <h1 className="mt-6 text-center text-3xl font-extrabold text-gray-900">
          Acceso a tu cuenta
        </h1>
        <p className="mt-3 text-center text-sm text-gray-600">
          Inicia sesión con una cuenta habilitada por el servicio.
        </p>
      </div>

      <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
        {snapshot.status === "authenticated" ? (
          <section
            aria-label="Estado de sesión"
            className="space-y-4 rounded-2xl border border-gray-200 bg-white p-6 text-center shadow-sm"
          >
            <p className="font-medium text-gray-900">Sesión activa</p>
            {snapshot.restrictedMode ? (
              <p className="text-sm text-amber-800">
                Modo restringido: solo se habilitan operaciones autorizadas por
                el servicio.
              </p>
            ) : null}
            <Button
              type="button"
              disabled={pending}
              onClick={() => void signOut()}
              className="w-full flex justify-center items-center py-2 px-4 border border-transparent rounded-full shadow-sm text-sm font-medium text-white bg-orange-700 hover:bg-orange-800 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-700"
            >
              Cerrar sesión
            </Button>
          </section>
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
                  autoComplete="off"
                  placeholder="correo@example.invalid"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  disabled={pending}
                  required
                  className="appearance-none rounded-full relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 focus:outline-none focus:ring-orange-500 focus:border-orange-500 focus:z-10 sm:text-sm"
                />
              </div>
            </div>

            <div>
              <Label
                htmlFor="password"
                className="block text-sm font-medium text-gray-700"
              >
                Contraseña
              </Label>
              <div className="mt-1">
                <Input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="off"
                  placeholder="Ingresa tu contraseña"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  disabled={pending}
                  required
                  className="appearance-none rounded-full relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 focus:outline-none focus:ring-orange-500 focus:border-orange-500 focus:z-10 sm:text-sm"
                />
              </div>
            </div>

            <Button
              type="submit"
              disabled={pending}
              className="w-full flex justify-center items-center py-2 px-4 border border-transparent rounded-full shadow-sm text-sm font-medium text-white bg-orange-700 hover:bg-orange-800 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-700"
            >
              {pending ? "Conectando…" : "Iniciar sesión"}
            </Button>
            <p className="text-center text-xs leading-relaxed text-gray-600">
              La sesión se conserva solo en la memoria de esta pestaña.
            </p>
          </form>
        )}

        {message ? (
          <p className="mt-4 text-center text-sm text-gray-700" role="status">
            {message}
          </p>
        ) : null}

        <div className="mt-6">
          <div className="relative">
            <div className="absolute inset-0 flex items-center">
              <div className="w-full border-t border-gray-300" />
            </div>
            <div className="relative flex justify-center text-sm">
              <span className="px-2 bg-gray-50 text-gray-500">
                Navegación de demostración
              </span>
            </div>
          </div>
          <div className="mt-6">
            <Link
              href="/saas"
              className="w-full flex justify-center py-2 px-4 border border-gray-300 rounded-full shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500"
            >
              Volver a la presentación
            </Link>
          </div>
        </div>
      </div>
    </main>
  );
}
