"use client";

import { useEffect, useState } from "react";
import {
  BarChart3,
  Building2,
  Download,
  Loader2,
  RefreshCw,
  Scale,
  UserCheck,
  Users,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { Badge } from "@/components/ui/badge";
import { useAdmin } from "./use-admin";
import { AdminFeedbackBanners } from "./admin-feedback-banners";

export function AdminOverviewView() {
  const {
    snapshot,
    loadOverview,
    loadKPIs,
    loadGlobalMe,
    loadVerifications,
    loadDisputes,
    loadGlobalPublications,
    exportGlobalAuditEvents,
    canAccess,
  } = useAdmin();

  const [isExporting, setIsExporting] = useState(false);

  useEffect(() => {
    void loadGlobalMe();
    void loadOverview();
    void loadKPIs({
      from: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
      until: new Date().toISOString(),
      time_zone: "America/Santiago",
    });
    void loadVerifications();
    void loadDisputes();
    void loadGlobalPublications();
  }, [
    loadGlobalMe,
    loadOverview,
    loadKPIs,
    loadVerifications,
    loadDisputes,
    loadGlobalPublications,
  ]);

  const handleRefresh = async () => {
    await Promise.all([
      loadOverview(),
      loadKPIs({
        from: new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(),
        until: new Date().toISOString(),
        time_zone: "America/Santiago",
      }),
      loadVerifications(),
      loadDisputes(),
      loadGlobalPublications(),
    ]);
  };

  const handleExportAudit = async () => {
    setIsExporting(true);
    try {
      const from = new Date(
        Date.now() - 30 * 24 * 60 * 60 * 1000,
      ).toISOString();
      const until = new Date().toISOString();
      const blob = await exportGlobalAuditEvents({
        from,
        until,
        format: "json",
      });
      if (blob) {
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `auditoria_administrativa_${new Date().toISOString().slice(0, 10)}.json`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        setTimeout(() => URL.revokeObjectURL(url), 100);
      }
    } finally {
      setIsExporting(false);
    }
  };

  const { overview, verifications, disputes, globalPublications, loading } =
    snapshot;

  const pendingVerifications = verifications.filter(
    (v) => v.state === "en_revision",
  ).length;
  const openDisputes = disputes.filter(
    (d) => d.state === "abierta" || d.state === "en_revision",
  ).length;
  const activePublications = globalPublications.filter(
    (p) => p.state === "activa",
  ).length;

  return (
    <div className="space-y-6">
      <AdminFeedbackBanners />

      {/* Cabecera y acciones rápidas */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight text-foreground">
            Resumen y KPIs Operativos
          </h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            Métricas globales consolidadas, estado de la plataforma y
            trazabilidad.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleRefresh}
            disabled={loading}
            className="gap-1.5"
          >
            <RefreshCw
              className={`size-4 ${loading ? "animate-spin" : ""}`}
              aria-hidden="true"
            />
            <span>Actualizar</span>
          </Button>

          {canAccess("audit") && (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={handleExportAudit}
              disabled={isExporting}
              className="gap-1.5"
            >
              {isExporting ? (
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
              ) : (
                <Download className="size-4" aria-hidden="true" />
              )}
              <span>Exportar Auditoría (30d)</span>
            </Button>
          )}
        </div>
      </div>

      {/* Tarjetas de KPIs Principales */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Total Cuentas
            </CardTitle>
            <Users
              className="size-4 text-muted-foreground"
              aria-hidden="true"
            />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {overview?.total_accounts ?? "-"}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {overview?.administrative_blocked ?? 0} bloqueadas
              administrativamente
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Publicaciones Activas
            </CardTitle>
            <Building2
              className="size-4 text-muted-foreground"
              aria-hidden="true"
            />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{activePublications}</div>
            <p className="text-xs text-muted-foreground mt-1">
              {globalPublications.length} publicaciones registradas
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Verificaciones Pendientes
            </CardTitle>
            <UserCheck
              className="size-4 text-muted-foreground"
              aria-hidden="true"
            />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-amber-600 dark:text-amber-400">
              {pendingVerifications}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {verifications.length} casos totales en bandeja
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              Disputas Abiertas
            </CardTitle>
            <Scale
              className="size-4 text-muted-foreground"
              aria-hidden="true"
            />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-rose-600 dark:text-rose-400">
              {openDisputes}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              {disputes.length} disputas históricas registradas
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Desglose de estados y roles */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-base font-semibold flex items-center gap-2">
              <BarChart3 className="size-4 text-primary" aria-hidden="true" />
              <span>Distribución de Cuentas por Estado</span>
            </CardTitle>
            <CardDescription>
              Estado de vida de las cuentas en el sistema
            </CardDescription>
          </CardHeader>
          <CardContent>
            {overview?.by_state && Object.keys(overview.by_state).length > 0 ? (
              <ul className="space-y-2.5">
                {Object.entries(overview.by_state).map(([state, count]) => (
                  <li
                    key={state}
                    className="flex items-center justify-between py-1 border-b border-border/50 text-sm last:border-0"
                  >
                    <span className="capitalize text-muted-foreground">
                      {state.replace(/_/g, " ")}
                    </span>
                    <Badge variant="outline" className="font-mono">
                      {String(count)}
                    </Badge>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground italic py-4 text-center">
                No hay desglose por estado disponible.
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base font-semibold flex items-center gap-2">
              <Users className="size-4 text-primary" aria-hidden="true" />
              <span>Distribución de Cuentas por Rol</span>
            </CardTitle>
            <CardDescription>
              Roles asignados a usuarios en el directorio global
            </CardDescription>
          </CardHeader>
          <CardContent>
            {overview?.by_role && Object.keys(overview.by_role).length > 0 ? (
              <ul className="space-y-2.5">
                {Object.entries(overview.by_role).map(([role, count]) => (
                  <li
                    key={role}
                    className="flex items-center justify-between py-1 border-b border-border/50 text-sm last:border-0"
                  >
                    <span className="capitalize text-muted-foreground">
                      {role.replace(/_/g, " ")}
                    </span>
                    <Badge variant="secondary" className="font-mono">
                      {String(count)}
                    </Badge>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground italic py-4 text-center">
                No hay desglose por rol disponible.
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Metadatos de generación */}
      {overview?.generated_at && (
        <p className="text-xs text-muted-foreground text-right">
          Última actualización global:{" "}
          {new Date(overview.generated_at).toLocaleString("es-CL")}
        </p>
      )}
    </div>
  );
}
