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
import { demoAccount } from "@/lib/workspace-fixtures";

export default function GeneralPage() {
  return (
    <section className="flex-1 p-4 lg:p-8">
      <h1 className="text-lg lg:text-2xl font-medium text-gray-900 mb-6">
        Información general
      </h1>

      <Card>
        <CardHeader>
          <CardTitle>Información de la cuenta</CardTitle>
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
              Valores sintéticos. La sesión y la actualización de cuenta no
              están conectadas.
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
