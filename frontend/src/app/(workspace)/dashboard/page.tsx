import Link from "next/link";
import {
  Activity,
  Building2,
  CircleHelp,
  ExternalLink,
  Shield,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { workspaceActivity, workspaceSummary } from "@/lib/workspace-fixtures";

const eventIcons = {
  view: Building2,
  account: Activity,
  security: Shield,
} as const;

export default function DashboardPage() {
  return (
    <section className="flex-1 p-4 lg:p-8">
      <h1 className="text-lg lg:text-2xl font-medium mb-6 text-gray-900">
        Resumen del panel
      </h1>

      <Card className="mb-8">
        <CardHeader>
          <CardTitle>Estado del espacio de trabajo</CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="space-y-4">
            {workspaceSummary.map((item) => (
              <li
                key={item.title}
                className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2"
              >
                <div>
                  <p className="font-medium text-gray-900">
                    {item.title}: {item.value}
                  </p>
                  <p className="text-sm text-gray-600">{item.detail}</p>
                </div>
                <span className="text-xs text-gray-600">Solo muestra</span>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>

      <Card className="mb-8">
        <CardHeader>
          <CardTitle>Actividad reciente</CardTitle>
        </CardHeader>
        <CardContent>
          {workspaceActivity.length > 0 ? (
            <ul className="space-y-4">
              {workspaceActivity.map((event) => {
                const Icon = eventIcons[event.icon];
                return (
                  <li key={event.id} className="flex items-center space-x-4">
                    <div className="bg-orange-100 rounded-full p-2">
                      <Icon
                        className="w-5 h-5 text-orange-600"
                        aria-hidden="true"
                      />
                    </div>
                    <div className="flex-1">
                      <p className="text-sm font-medium text-gray-900">
                        {event.title}
                      </p>
                      <p className="text-xs text-gray-600">
                        {event.description} · {event.when}
                      </p>
                    </div>
                  </li>
                );
              })}
            </ul>
          ) : (
            <p className="text-gray-600">
              No hay actividad de muestra disponible.
            </p>
          )}
        </CardContent>
        <CardFooter className="border-t border-gray-200">
          <Link
            href="/dashboard/activity"
            className="inline-flex min-h-10 items-center gap-2 text-sm font-medium text-gray-700 hover:text-gray-900"
          >
            Ver historial <ExternalLink className="size-4" aria-hidden="true" />
          </Link>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Acciones de cuenta</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-gray-600">
            La sesión, la edición, las reservas y los cambios persistentes no
            están conectados en esta vista local.
          </p>
          <Button type="button" className="mt-4" disabled>
            Acción no disponible
          </Button>
        </CardContent>
        <CardFooter className="border-t border-gray-200">
          <p className="flex items-start gap-2 text-xs leading-relaxed text-gray-600">
            <CircleHelp className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            Navega por las vistas del starter sin introducir datos personales.
          </p>
        </CardFooter>
      </Card>
    </section>
  );
}
