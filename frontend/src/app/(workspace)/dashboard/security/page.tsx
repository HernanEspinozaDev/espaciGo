"use client";

import { Button } from "@/components/workspace/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import { Lock, ShieldAlert, Trash2 } from "lucide-react";

export default function SecurityPage() {
  return (
    <section className="flex-1 p-4 lg:p-8">
      <h1 className="text-lg lg:text-2xl font-medium text-gray-900 mb-6">
        Seguridad
      </h1>
      <Card className="mb-8">
        <CardHeader>
          <CardTitle>Contraseña</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            className="space-y-4"
            onSubmit={(event) => event.preventDefault()}
          >
            <div>
              <Label htmlFor="current-password" className="mb-2">
                Contraseña actual
              </Label>
              <Input
                id="current-password"
                name="currentPassword"
                type="password"
                autoComplete="off"
                disabled
              />
            </div>
            <div>
              <Label htmlFor="new-password" className="mb-2">
                Nueva contraseña
              </Label>
              <Input
                id="new-password"
                name="newPassword"
                type="password"
                autoComplete="off"
                disabled
              />
            </div>
            <div>
              <Label htmlFor="confirm-password" className="mb-2">
                Confirmar nueva contraseña
              </Label>
              <Input
                id="confirm-password"
                name="confirmPassword"
                type="password"
                autoComplete="off"
                disabled
              />
            </div>
            <Button
              type="button"
              className="bg-orange-500 hover:bg-orange-600 text-white"
              disabled
            >
              <Lock className="mr-2 h-4 w-4" aria-hidden="true" />
              Actualizar contraseña
            </Button>
            <p className="text-sm text-gray-600">
              No hay sesión ni gestión de credenciales conectadas.
            </p>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Eliminar cuenta</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-gray-500 mb-4">
            La acción no está disponible en la vista local de demostración.
          </p>
          <form
            className="space-y-4"
            onSubmit={(event) => event.preventDefault()}
          >
            <div>
              <Label htmlFor="delete-password" className="mb-2">
                Confirmar contraseña
              </Label>
              <Input
                id="delete-password"
                name="password"
                type="password"
                autoComplete="off"
                disabled
              />
            </div>
            <Button
              type="button"
              variant="destructive"
              className="bg-red-600 hover:bg-red-700"
              disabled
            >
              <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
              Eliminar cuenta
            </Button>
          </form>
          <p className="mt-4 flex items-start gap-2 text-sm text-gray-600">
            <ShieldAlert
              className="mt-0.5 size-4 shrink-0"
              aria-hidden="true"
            />
            No se envía ni elimina ninguna cuenta desde esta vista.
          </p>
        </CardContent>
      </Card>
    </section>
  );
}
