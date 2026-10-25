"use client";

import { AlertCircle, AlertTriangle, KeyRound, X } from "lucide-react";
import { Button } from "@/components/workspace/ui/button";
import { useAdmin } from "./use-admin";

export function AdminFeedbackBanners() {
  const { snapshot, clearFeedback } = useAdmin();

  if (
    !snapshot.recentAuthRequired &&
    !snapshot.concurrencyConflict &&
    !snapshot.error
  ) {
    return null;
  }

  return (
    <aside
      aria-label="Alertas y notificaciones administrativas"
      className="space-y-3"
    >
      {/* Gate 428: Reautenticación reciente requerida */}
      {snapshot.recentAuthRequired && (
        <div
          role="alert"
          className="flex items-start justify-between gap-3 rounded-lg border border-amber-300 bg-amber-50 p-4 text-amber-900 shadow-sm dark:border-amber-800 dark:bg-amber-950/50 dark:text-amber-200"
        >
          <div className="flex items-start gap-3">
            <KeyRound
              className="mt-0.5 size-5 shrink-0 text-amber-600 dark:text-amber-400"
              aria-hidden="true"
            />
            <div>
              <p className="font-semibold text-sm">
                Reautenticación requerida (HTTP 428)
              </p>
              <p className="mt-0.5 text-xs text-amber-800 dark:text-amber-300">
                Esta operación administrativa sensible requiere que confirmes
                tus credenciales de inicio de sesión reciente antes de
                continuar.
              </p>
            </div>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={clearFeedback}
            className="h-7 w-7 p-0 text-amber-700 hover:bg-amber-100 hover:text-amber-900 dark:text-amber-300 dark:hover:bg-amber-900"
            aria-label="Cerrar alerta de reautenticación"
          >
            <X className="size-4" aria-hidden="true" />
          </Button>
        </div>
      )}

      {/* Gate 409: Conflicto de concurrencia optimista */}
      {snapshot.concurrencyConflict && (
        <div
          role="alert"
          className="flex items-start justify-between gap-3 rounded-lg border border-orange-300 bg-orange-50 p-4 text-orange-900 shadow-sm dark:border-orange-800 dark:bg-orange-950/50 dark:text-orange-200"
        >
          <div className="flex items-start gap-3">
            <AlertTriangle
              className="mt-0.5 size-5 shrink-0 text-orange-600 dark:text-orange-400"
              aria-hidden="true"
            />
            <div>
              <p className="font-semibold text-sm">
                Conflicto de concurrencia detectado (HTTP 409)
              </p>
              <p className="mt-0.5 text-xs text-orange-800 dark:text-orange-300">
                El recurso fue actualizado por otra sesión administrativa
                simultánea (versión no coincidente). Recarga el elemento para
                obtener el estado más reciente.
              </p>
            </div>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={clearFeedback}
            className="h-7 w-7 p-0 text-orange-700 hover:bg-orange-100 hover:text-orange-900 dark:text-orange-300 dark:hover:bg-orange-900"
            aria-label="Cerrar alerta de conflicto de concurrencia"
          >
            <X className="size-4" aria-hidden="true" />
          </Button>
        </div>
      )}

      {/* Error general o 403 con purga de memoria */}
      {snapshot.error &&
        !snapshot.recentAuthRequired &&
        !snapshot.concurrencyConflict && (
          <div
            role="alert"
            className="flex items-start justify-between gap-3 rounded-lg border border-red-300 bg-red-50 p-4 text-red-900 shadow-sm dark:border-red-800 dark:bg-red-950/50 dark:text-red-200"
          >
            <div className="flex items-start gap-3">
              <AlertCircle
                className="mt-0.5 size-5 shrink-0 text-red-600 dark:text-red-400"
                aria-hidden="true"
              />
              <div>
                <p className="font-semibold text-sm">
                  Mensaje del sistema administrativo
                </p>
                <p className="mt-0.5 text-xs text-red-800 dark:text-red-300">
                  {snapshot.error}
                </p>
              </div>
            </div>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={clearFeedback}
              className="h-7 w-7 p-0 text-red-700 hover:bg-red-100 hover:text-red-900 dark:text-red-300 dark:hover:bg-red-900"
              aria-label="Cerrar alerta de error"
            >
              <X className="size-4" aria-hidden="true" />
            </Button>
          </div>
        )}
    </aside>
  );
}
