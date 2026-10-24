import { AlertCircle, Building2, Settings, Shield } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { workspaceActivity } from "@/lib/workspace-fixtures";

const iconMap: Record<(typeof workspaceActivity)[number]["icon"], LucideIcon> =
  {
    view: Building2,
    account: Settings,
    security: Shield,
  };

export default function ActivityPage() {
  return (
    <section className="flex-1 p-4 lg:p-8">
      <h1 className="mb-6 text-lg font-medium text-gray-900 lg:text-2xl">
        Historial de actividad
      </h1>
      <Card>
        <CardHeader>
          <CardTitle>Actividad reciente</CardTitle>
        </CardHeader>
        <CardContent>
          {workspaceActivity.length > 0 ? (
            <ul className="space-y-4">
              {workspaceActivity.map((event) => {
                const Icon = iconMap[event.icon];
                return (
                  <li key={event.id} className="flex items-center space-x-4">
                    <div className="rounded-full bg-orange-100 p-2">
                      <Icon
                        className="h-5 w-5 text-orange-600"
                        aria-hidden="true"
                      />
                    </div>
                    <div className="flex-1">
                      <p className="text-sm font-medium text-gray-900">
                        {event.title}
                      </p>
                      <p className="text-xs text-gray-500">
                        {event.description} · {event.when}
                      </p>
                    </div>
                  </li>
                );
              })}
            </ul>
          ) : (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <AlertCircle
                className="mb-4 h-12 w-12 text-orange-500"
                aria-hidden="true"
              />
              <h2 className="mb-2 text-lg font-semibold text-gray-900">
                Sin actividad de muestra
              </h2>
              <p className="max-w-sm text-sm text-gray-500">
                Esta vista no está conectada a una sesión ni a un historial
                persistente.
              </p>
            </div>
          )}
        </CardContent>
      </Card>
      <p className="mt-4 text-xs text-gray-500">
        Eventos sintéticos de presentación; no registran acciones ni datos
        personales.
      </p>
    </section>
  );
}
