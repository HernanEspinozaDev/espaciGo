"use client";

import { useEffect, useState } from "react";
import {
  Code,
  Download,
  FileText,
  Filter,
  Loader2,
  RefreshCw,
  Search,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/workspace/ui/label";
import { Input } from "@/components/workspace/ui/input";
import { useAdmin } from "./use-admin";
import type { AuditEvent, AuditEventsQuery } from "../domain/types";
import { AdminFeedbackBanners } from "./admin-feedback-banners";

export function AdminAuditView() {
  const { snapshot, loadGlobalAuditEvents, exportGlobalAuditEvents } =
    useAdmin();

  const [searchAction, setSearchAction] = useState("");
  const [resultFilter, setResultFilter] = useState<string>("todos");
  const [resourceFilter, setResourceFilter] = useState("");
  const [isExporting, setIsExporting] = useState<string | null>(null);

  // Selected event for details view
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);

  // Dates for query (default last 30 days)
  const [fromDate, setFromDate] = useState(() => {
    const d = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000);
    return d.toISOString().slice(0, 10);
  });
  const [untilDate, setUntilDate] = useState(() => {
    return new Date().toISOString().slice(0, 10);
  });

  const fetchEvents = async () => {
    const query: AuditEventsQuery = {
      from: new Date(fromDate).toISOString(),
      until: new Date(untilDate + "T23:59:59.999Z").toISOString(),
      result:
        resultFilter === "exito" || resultFilter === "rechazo"
          ? resultFilter
          : undefined,
      action: searchAction.trim() || undefined,
      resource_type: resourceFilter.trim() || undefined,
    };
    await loadGlobalAuditEvents(query);
  };

  useEffect(() => {
    void fetchEvents();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resultFilter]);

  const { globalAuditEvents, loading } = snapshot;

  const handleExport = async (format: "json" | "csv") => {
    setIsExporting(format);
    try {
      const query: AuditEventsQuery = {
        from: new Date(fromDate).toISOString(),
        until: new Date(untilDate + "T23:59:59.999Z").toISOString(),
        format,
        result:
          resultFilter === "exito" || resultFilter === "rechazo"
            ? resultFilter
            : undefined,
        action: searchAction.trim() || undefined,
        resource_type: resourceFilter.trim() || undefined,
      };
      const blob = await exportGlobalAuditEvents(query);
      if (blob) {
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `auditoria_${format}_${new Date().toISOString().slice(0, 10)}.${format}`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        setTimeout(() => URL.revokeObjectURL(url), 100);
      }
    } finally {
      setIsExporting(null);
    }
  };

  return (
    <div className="space-y-6">
      <AdminFeedbackBanners />

      {/* Encabezado */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight text-foreground">
            Trazabilidad y Auditoría Administrativa
          </h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            Registro cronológico inmutable de operaciones sensibles, actores y
            resoluciones del sistema.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void fetchEvents()}
            disabled={loading}
            className="gap-1.5"
          >
            <RefreshCw
              className={`size-4 ${loading ? "animate-spin" : ""}`}
              aria-hidden="true"
            />
            <span>Actualizar</span>
          </Button>

          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => void handleExport("json")}
            disabled={isExporting !== null}
            className="gap-1.5"
          >
            {isExporting === "json" ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : (
              <Download className="size-4" aria-hidden="true" />
            )}
            <span>JSON</span>
          </Button>

          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => void handleExport("csv")}
            disabled={isExporting !== null}
            className="gap-1.5"
          >
            {isExporting === "csv" ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : (
              <Download className="size-4" aria-hidden="true" />
            )}
            <span>CSV</span>
          </Button>
        </div>
      </div>

      {/* Filtros de Auditoría */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <Filter className="size-4 text-primary" aria-hidden="true" />
            <span>Filtros de Búsqueda</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void fetchEvents();
            }}
            className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3 text-sm"
          >
            <div className="space-y-1.5">
              <Label htmlFor="audit-from">Desde</Label>
              <Input
                id="audit-from"
                type="date"
                value={fromDate}
                onChange={(e) => setFromDate(e.target.value)}
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="audit-until">Hasta</Label>
              <Input
                id="audit-until"
                type="date"
                value={untilDate}
                onChange={(e) => setUntilDate(e.target.value)}
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="audit-result">Resultado</Label>
              <select
                id="audit-result"
                value={resultFilter}
                onChange={(e) => setResultFilter(e.target.value)}
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="todos">Todos los resultados</option>
                <option value="exito">Éxito</option>
                <option value="rechazo">Rechazo</option>
              </select>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="audit-resource">Tipo de Recurso</Label>
              <Input
                id="audit-resource"
                placeholder="Ej. account"
                value={resourceFilter}
                onChange={(e) => setResourceFilter(e.target.value)}
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="audit-action">Acción o Evento</Label>
              <Input
                id="audit-action"
                placeholder="Ej. accounts.block"
                value={searchAction}
                onChange={(e) => setSearchAction(e.target.value)}
              />
            </div>

            <div className="flex items-end">
              <Button type="submit" className="w-full gap-1.5">
                <Search className="size-4" aria-hidden="true" />
                <span>Filtrar</span>
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Lista / Timeline de Eventos */}
      {globalAuditEvents.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground text-sm space-y-2">
            <FileText
              className="size-10 mx-auto text-muted-foreground/60"
              aria-hidden="true"
            />
            <p className="font-medium">
              No hay eventos de auditoría que coincidan con la consulta.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {globalAuditEvents.map((event) => (
            <Card
              key={event.event_id}
              className="shadow-sm hover:border-primary/40 transition-colors"
            >
              <CardContent className="p-4 flex flex-col md:flex-row md:items-center md:justify-between gap-4">
                <div className="space-y-1.5 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge
                      variant={
                        event.result === "exito" ? "default" : "destructive"
                      }
                      className={
                        event.result === "exito"
                          ? "bg-emerald-700 text-white font-mono text-xs"
                          : "font-mono text-xs"
                      }
                    >
                      {event.result.toUpperCase()}
                    </Badge>
                    <span className="font-semibold font-mono text-sm text-foreground">
                      {event.action}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {new Date(event.timestamp).toLocaleString("es-CL")}
                    </span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 text-xs text-muted-foreground pt-1">
                    <div>
                      <span className="font-medium">Actor: </span>
                      <span className="font-mono">
                        {event.actor_id.slice(0, 8)}...
                      </span>
                    </div>
                    <div>
                      <span className="font-medium">Recurso: </span>
                      <span>
                        {event.resource_type ?? "N/A"}{" "}
                        {event.resource_id
                          ? `(#${event.resource_id.slice(0, 8)})`
                          : ""}
                      </span>
                    </div>
                    <div>
                      <span className="font-medium">Motivo / Código: </span>
                      <span>{event.reason_code ?? "Ninguno"}</span>
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-2 shrink-0">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setSelectedEvent(event)}
                    className="text-xs gap-1.5"
                  >
                    <Code className="size-3.5" aria-hidden="true" />
                    <span>Detalles Payload</span>
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Modal Detalles Payload */}
      <Dialog
        open={selectedEvent !== null}
        onOpenChange={(open) => !open && setSelectedEvent(null)}
      >
        <DialogContent className="max-w-xl max-h-[85vh] flex flex-col">
          <DialogHeader>
            <DialogTitle className="font-mono text-sm flex items-center gap-2">
              <Code className="size-4 text-primary" aria-hidden="true" />
              <span>Detalles del Evento: {selectedEvent?.action}</span>
            </DialogTitle>
            <DialogDescription>
              ID de Evento: {selectedEvent?.event_id} • Timestamp:{" "}
              {selectedEvent
                ? new Date(selectedEvent.timestamp).toISOString()
                : ""}
            </DialogDescription>
          </DialogHeader>

          <div className="flex-1 overflow-auto rounded-md bg-muted p-3 font-mono text-xs text-foreground/90 space-y-2">
            <pre className="whitespace-pre-wrap break-all">
              {JSON.stringify(
                {
                  event_id: selectedEvent?.event_id,
                  actor_id: selectedEvent?.actor_id,
                  target_account_id: selectedEvent?.target_account_id,
                  action: selectedEvent?.action,
                  result: selectedEvent?.result,
                  resource_type: selectedEvent?.resource_type,
                  resource_id: selectedEvent?.resource_id,
                  reason_code: selectedEvent?.reason_code,
                  correlation_id: selectedEvent?.correlation_id,
                  timestamp: selectedEvent?.timestamp,
                  details: selectedEvent?.details,
                },
                null,
                2,
              )}
            </pre>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setSelectedEvent(null)}
            >
              Cerrar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
