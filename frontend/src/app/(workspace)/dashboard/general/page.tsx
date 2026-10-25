"use client";

import Link from "next/link";
import { ArrowRight, User } from "lucide-react";
import { Button } from "@/components/workspace/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import { demoAccount } from "@/lib/workspace-fixtures";

export default function GeneralPage() {
  return (
    <section className="flex-1 p-4 lg:p-8 space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-lg lg:text-2xl font-medium text-gray-900">
            Información general
          </h1>
          <p className="text-sm text-gray-500 mt-1">
            Vista previa del workspace general.
          </p>
        </div>
        <Button asChild className="bg-primary text-primary-foreground">
          <Link href="/mi-cuenta" className="gap-2">
            <User className="size-4" />
            Ir a Mi Cuenta (Producción)
            <ArrowRight className="size-4" />
          </Link>
        </Button>
      </div>

      <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-900 flex items-start justify-between gap-4 dark:border-blue-800 dark:bg-blue-950/30 dark:text-blue-200">
        <div>
          <p className="font-medium">
            ¿Deseas editar tu perfil, foto o cuenta de cobro?
          </p>
          <p className="text-xs text-blue-700 dark:text-blue-300 mt-1">
            La administración completa de perfil, fotos sintéticas y cuentas de
            cobro se gestiona en la sección centralizada de Mi Cuenta.
          </p>
        </div>
        <Button asChild size="sm" variant="outline" className="shrink-0">
          <Link href="/mi-cuenta">Configurar en Mi Cuenta</Link>
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Información de la cuenta (Demo)</CardTitle>
          <CardDescription>Valores sintéticos del workspace</CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="space-y-4"
            onSubmit={(event) => event.preventDefault()}
          >
            <div>
              <Label htmlFor="name" className="mb-2">
                Nombre
              </Label>
              <Input
                id="name"
                name="name"
                placeholder="Nombre de muestra"
                defaultValue={demoAccount.name}
                disabled
              />
            </div>
            <div>
              <Label htmlFor="email" className="mb-2">
                Correo electrónico
              </Label>
              <Input
                id="email"
                name="email"
                type="email"
                placeholder="correo@example.invalid"
                defaultValue={demoAccount.email}
                disabled
              />
            </div>
            <p className="text-sm text-gray-600">
              Valores sintéticos. Para gestionar tu información real y
              autenticada dirígete a{" "}
              <Link
                href="/mi-cuenta"
                className="text-primary underline font-medium"
              >
                Mi Cuenta
              </Link>
              .
            </p>
            <Button
              type="button"
              className="bg-orange-500 hover:bg-orange-600 text-white"
              disabled
            >
              Guardar cambios
            </Button>
          </form>
        </CardContent>
      </Card>
    </section>
  );
}
