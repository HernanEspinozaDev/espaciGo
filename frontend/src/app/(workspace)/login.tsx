"use client";

import Link from "next/link";
import { Button } from "@/components/workspace/ui/button";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import { CircleIcon } from "lucide-react";

export function Login({ mode = "signin" }: { mode?: "signin" | "signup" }) {
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
          {mode === "signin" ? "Acceso a tu cuenta" : "Crear una cuenta"}
        </h1>
        <p className="mt-3 text-center text-sm text-gray-600">
          Vista del SaaS Starter adaptada. Autenticación y registro no están
          conectados.
        </p>
      </div>

      <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
        <form
          className="space-y-6"
          onSubmit={(event) => event.preventDefault()}
        >
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
                disabled
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
                placeholder="No disponible en esta fase"
                disabled
                className="appearance-none rounded-full relative block w-full px-3 py-2 border border-gray-300 placeholder-gray-500 text-gray-900 focus:outline-none focus:ring-orange-500 focus:border-orange-500 focus:z-10 sm:text-sm"
              />
            </div>
          </div>

          <Button
            type="button"
            disabled
            className="w-full flex justify-center items-center py-2 px-4 border border-transparent rounded-full shadow-sm text-sm font-medium text-white bg-orange-600 hover:bg-orange-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500"
          >
            {mode === "signin" ? "Iniciar sesión" : "Crear cuenta"}
          </Button>
          <p className="text-center text-xs leading-relaxed text-gray-600">
            Los campos están deshabilitados. No introduzcas credenciales reales;
            no hay sesión ni persistencia.
          </p>
        </form>

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
