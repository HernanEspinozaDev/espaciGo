"use client";

import { useEffect, useState, useMemo } from "react";
import Link from "next/link";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useBookings } from "./use-bookings";
import { formatBookingClp } from "./format";
import type { BookingState } from "../domain/types";
import { Button } from "@/components/workspace/ui/button";
import {
  Calendar,
  Clock,
  ArrowRight,
  Search,
  FileText,
  AlertCircle,
  CheckCircle,
  Clock3,
  XCircle,
  HelpCircle,
  RefreshCw,
} from "lucide-react";

export type ReservationFilterGroup =
  "todas" | "solicitada" | "confirmada" | "activa" | "completada" | "cancelada";

interface ReservationsListProps {
  mode?: "renter" | "host";
  title?: string;
  description?: string;
}

export function getBookingStateConfig(state: BookingState): {
  label: string;
  group: ReservationFilterGroup;
  badgeClass: string;
  icon: React.ComponentType<{ className?: string }>;
} {
  switch (state) {
    case "pendiente_de_pago":
      return {
        label: "Pendiente de Pago",
        group: "solicitada",
        badgeClass:
          "bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/20",
        icon: Clock3,
      };
    case "pagada":
      return {
        label: "Pago Realizado / Esperando Anfitrión",
        group: "solicitada",
        badgeClass:
          "bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20",
        icon: Clock,
      };
    case "aprobada_host":
      return {
        label: "Aprobada por Anfitrión",
        group: "confirmada",
        badgeClass:
          "bg-indigo-500/10 text-indigo-700 dark:text-indigo-400 border-indigo-500/20",
        icon: CheckCircle,
      };
    case "firma_parcial":
      return {
        label: "Firma Parcial de Contrato",
        group: "confirmada",
        badgeClass:
          "bg-indigo-500/10 text-indigo-700 dark:text-indigo-400 border-indigo-500/20",
        icon: FileText,
      };
    case "lista_para_checkin":
      return {
        label: "Lista para Check-in",
        group: "confirmada",
        badgeClass:
          "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/20",
        icon: CheckCircle,
      };
    case "en_curso":
      return {
        label: "Arriendo en Curso",
        group: "activa",
        badgeClass:
          "bg-emerald-600/15 text-emerald-800 dark:text-emerald-300 border-emerald-600/30 animate-pulse",
        icon: Clock,
      };
    case "finalizada":
      return {
        label: "Finalizada (Check-out)",
        group: "completada",
        badgeClass:
          "bg-teal-500/10 text-teal-700 dark:text-teal-400 border-teal-500/20",
        icon: CheckCircle,
      };
    case "cerrada":
      return {
        label: "Cerrada y Liquidada",
        group: "completada",
        badgeClass:
          "bg-zinc-500/10 text-zinc-700 dark:text-zinc-400 border-zinc-500/20",
        icon: CheckCircle,
      };
    case "en_disputa":
      return {
        label: "En Disputa",
        group: "cancelada",
        badgeClass:
          "bg-rose-500/10 text-rose-700 dark:text-rose-400 border-rose-500/20",
        icon: AlertCircle,
      };
    case "cancelada_por_firma":
    case "cancelada_por_pago":
    case "rechazada_arrendador":
    case "vencida_pago":
    case "vencida_host":
    case "cancelada_arrendatario":
      return {
        label: state.replace(/_/g, " "),
        group: "cancelada",
        badgeClass: "bg-destructive/10 text-destructive border-destructive/20",
        icon: XCircle,
      };
    default:
      return {
        label: state,
        group: "todas",
        badgeClass: "bg-muted text-muted-foreground border-border",
        icon: HelpCircle,
      };
  }
}

export function ReservationsList({
  mode = "renter",
  title,
  description,
}: ReservationsListProps) {
  const { snapshot: authSnapshot } = useAuth();
  const isAuthenticated = authSnapshot.status === "authenticated";
  const accountId = authSnapshot.accountId;
  const expiresAt = authSnapshot.expiresAt;

  const { reservations, loading, error, loadReservations } = useBookings();
  const [activeTab, setActiveTab] = useState<ReservationFilterGroup>("todas");
  const [searchQuery, setSearchQuery] = useState("");

  const accountKey = `${authSnapshot.status}:${accountId}:${expiresAt}`;
  const [prevAccountKey, setPrevAccountKey] = useState(accountKey);

  if (prevAccountKey !== accountKey) {
    setPrevAccountKey(accountKey);
    setActiveTab("todas");
    setSearchQuery("");
  }

  useEffect(() => {
    if (isAuthenticated) {
      loadReservations().catch(() => {});
    }
  }, [isAuthenticated, accountId, expiresAt, loadReservations]);

  const filteredReservations = useMemo(() => {
    return reservations.filter((reservation) => {
      const stateConfig = getBookingStateConfig(reservation.state);
      const matchesTab =
        activeTab === "todas" || stateConfig.group === activeTab;

      const matchesSearch =
        !searchQuery ||
        reservation.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
        reservation.space_id.toLowerCase().includes(searchQuery.toLowerCase());

      return matchesTab && matchesSearch;
    });
  }, [reservations, activeTab, searchQuery]);

  const detailBaseHref = mode === "host" ? "/anfitrion/reservas" : "/reservas";

  const filterTabs: Array<{ id: ReservationFilterGroup; label: string }> = [
    { id: "todas", label: "Todas" },
    { id: "solicitada", label: "Solicitadas" },
    { id: "confirmada", label: "Confirmadas" },
    { id: "activa", label: "En Curso" },
    { id: "completada", label: "Completadas" },
    { id: "cancelada", label: "Canceladas / Cierre" },
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
            {title ??
              (mode === "host" ? "Reservas como Anfitrión" : "Mis Reservas")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {description ??
              (mode === "host"
                ? "Gestiona aprobaciones, firmas de contratos digitales y operaciones de tus espacios."
                : "Monitorea tus arriendos, pagos simulados con Mercado Pago y ciclo de uso del espacio.")}
          </p>
        </div>
        <div className="flex items-center gap-3">
          {isAuthenticated && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => loadReservations()}
              disabled={loading}
              className="inline-flex items-center gap-2"
            >
              <RefreshCw
                className={`h-4 w-4 ${loading ? "animate-spin" : ""}`}
              />
              Actualizar
            </Button>
          )}
          {mode === "renter" ? (
            <Button asChild size="sm">
              <Link href="/reservas/nueva">Nueva Cotización</Link>
            </Button>
          ) : (
            <Button asChild size="sm">
              <Link href="/anfitrion/espacios">Mis Espacios</Link>
            </Button>
          )}
        </div>
      </div>

      {/* Global Error Banner */}
      {error && (
        <div
          role="alert"
          className="flex items-start gap-3 rounded-xl border border-destructive/20 bg-destructive/10 p-4 text-sm text-red-800 dark:text-red-300"
        >
          <AlertCircle className="mt-0.5 h-5 w-5 shrink-0" />
          <div>
            <p className="font-medium">No pudimos sincronizar las reservas</p>
            <p className="mt-0.5">{error}</p>
          </div>
        </div>
      )}

      {/* Filter and Search Controls */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <nav
          aria-label="Filtro por estado de reserva"
          className="flex flex-wrap gap-1.5"
        >
          {filterTabs.map((tab) => {
            const isSelected = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                aria-current={isSelected ? "true" : undefined}
                className={`rounded-full px-3.5 py-1.5 text-xs font-medium transition-colors ${
                  isSelected
                    ? "bg-primary text-primary-foreground shadow-sm"
                    : "bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground"
                }`}
              >
                {tab.label}
              </button>
            );
          })}
        </nav>

        <div className="relative w-full sm:w-64">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="search"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Buscar ID de reserva..."
            className="w-full rounded-lg border border-input bg-background py-1.5 pl-9 pr-3 text-sm shadow-sm placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>
      </div>

      {/* List Content */}
      {loading && reservations.length === 0 ? (
        <div className="space-y-4">
          {[1, 2, 3].map((n) => (
            <div
              key={n}
              className="h-32 animate-pulse rounded-2xl border border-border bg-card/50 p-6"
            />
          ))}
        </div>
      ) : filteredReservations.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-border p-12 text-center">
          <Calendar className="mx-auto h-12 w-12 text-muted-foreground/50" />
          <h2 className="mt-4 text-base font-medium text-foreground">
            No se encontraron reservas
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {searchQuery || activeTab !== "todas"
              ? "No hay reservas que coincidan con los filtros seleccionados."
              : mode === "host"
                ? "Aún no has recibido solicitudes de reserva para tus espacios publicados."
                : "Aún no tienes reservas activas ni solicitudes en proceso."}
          </p>
          {mode === "renter" && !searchQuery && activeTab === "todas" && (
            <div className="mt-6">
              <Button asChild>
                <Link href="/explorar">Explorar Espacios Disponibles</Link>
              </Button>
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-4" role="feed" aria-label="Listado de reservas">
          {filteredReservations.map((reservation) => {
            const config = getBookingStateConfig(reservation.state);
            const Icon = config.icon;
            return (
              <article
                key={reservation.id}
                className="group relative rounded-2xl border border-border bg-card p-5 transition-shadow hover:shadow-md sm:p-6"
              >
                <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                  <div className="space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium ${config.badgeClass}`}
                      >
                        <Icon className="h-3.5 w-3.5" />
                        {config.label}
                      </span>
                      {reservation.unread_count > 0 && (
                        <span className="rounded-full bg-primary px-2 py-0.5 text-[10px] font-semibold text-primary-foreground">
                          {reservation.unread_count} nuevo
                          {reservation.unread_count > 1 ? "s" : ""}
                        </span>
                      )}
                      <span className="text-xs text-muted-foreground">
                        ID: {reservation.id.slice(0, 8)}...
                      </span>
                    </div>

                    <div className="flex flex-wrap items-baseline gap-2">
                      <h2 className="text-lg font-semibold tracking-tight text-foreground">
                        Espacio: {reservation.space_id.slice(0, 8)}...
                      </h2>
                      <span className="text-sm text-muted-foreground">
                        · {reservation.units} {reservation.rate_unit}
                        {reservation.units > 1 ? "s" : ""}
                      </span>
                    </div>

                    <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
                      <span className="flex items-center gap-1">
                        <Calendar className="h-3.5 w-3.5" />
                        Inicio:{" "}
                        {new Date(reservation.start_at).toLocaleDateString(
                          "es-CL",
                          {
                            day: "2-digit",
                            month: "short",
                            year: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                          },
                        )}
                      </span>
                      <span className="flex items-center gap-1">
                        <Clock className="h-3.5 w-3.5" />
                        Término:{" "}
                        {new Date(reservation.end_at).toLocaleDateString(
                          "es-CL",
                          {
                            day: "2-digit",
                            month: "short",
                            year: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                          },
                        )}
                      </span>
                    </div>
                  </div>

                  <div className="flex flex-row items-center justify-between gap-4 sm:flex-col sm:items-end">
                    <div className="text-left sm:text-right">
                      <p className="text-xs text-muted-foreground">
                        Total Arriendo
                      </p>
                      <p className="text-lg font-bold text-foreground">
                        {formatBookingClp(reservation.subtotal_clp)} CLP
                      </p>
                    </div>

                    <Button
                      asChild
                      size="sm"
                      className="inline-flex items-center gap-1.5"
                    >
                      <Link
                        href={`${detailBaseHref}/${encodeURIComponent(reservation.id)}`}
                      >
                        Gestionar
                        <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
                      </Link>
                    </Button>
                  </div>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </div>
  );
}
