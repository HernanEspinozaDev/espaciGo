"use client";

import { useEffect, useState } from "react";
import {
  AlertCircle,
  Building2,
  CheckCircle2,
  Eye,
  EyeOff,
  Loader2,
  RefreshCw,
  Search,
  Trash2,
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
import { isValidAdminReason } from "../domain/admin-rules";
import type { GlobalAdminPublication } from "../domain/types";
import { AdminFeedbackBanners } from "./admin-feedback-banners";

function renderPubStateBadge(state: GlobalAdminPublication["state"]) {
  switch (state) {
    case "activa":
      return (
        <Badge
          variant="default"
          className="gap-1 bg-emerald-700 hover:bg-emerald-700 text-white font-medium"
        >
          <CheckCircle2 className="size-3.5" aria-hidden="true" />
          Activa
        </Badge>
      );
    case "oculta":
      return (
        <Badge
          variant="secondary"
          className="gap-1 bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200"
        >
          <EyeOff className="size-3.5" aria-hidden="true" />
          Oculta
        </Badge>
      );
    case "borrador":
      return (
        <Badge variant="outline" className="gap-1 text-muted-foreground">
          Borrador
        </Badge>
      );
    case "eliminada":
      return (
        <Badge
          variant="destructive"
          className="gap-1 bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200 font-medium"
        >
          Retirada
        </Badge>
      );
    default:
      return <Badge variant="outline">{state}</Badge>;
  }
}

export function AdminSpacesView() {
  const {
    snapshot,
    loadGlobalPublications,
    loadPublishedSpaces,
    setGlobalPublicationState,
    retireGlobalPublication,
  } = useAdmin();

  const [searchFilter, setSearchFilter] = useState("");
  const [stateFilter, setStateFilter] = useState<string>("todas");

  // State change dialog
  const [targetPub, setTargetPub] = useState<GlobalAdminPublication | null>(
    null,
  );
  const [newState, setNewState] = useState<"activa" | "oculta">("activa");
  const [stateReason, setStateReason] = useState<
    "correccion_perfil" | "revision_administrativa" | "gestion_operativa"
  >("revision_administrativa");

  // Retire dialog
  const [retireTargetPub, setRetireTargetPub] =
    useState<GlobalAdminPublication | null>(null);
  const [retireReason, setRetireReason] = useState("");
  const [retireError, setRetireError] = useState<string | null>(null);

  useEffect(() => {
    void loadGlobalPublications();
    void loadPublishedSpaces();
  }, [loadGlobalPublications, loadPublishedSpaces]);

  const { globalPublications, loading, actionLoading } = snapshot;

  const filteredPublications = globalPublications.filter((pub) => {
    const matchesState = stateFilter === "todas" || pub.state === stateFilter;
    const matchesSearch =
      searchFilter.trim() === "" ||
      pub.title.toLowerCase().includes(searchFilter.toLowerCase()) ||
      pub.category_name.toLowerCase().includes(searchFilter.toLowerCase()) ||
      pub.owner_email.toLowerCase().includes(searchFilter.toLowerCase());
    return matchesState && matchesSearch;
  });

  const handleOpenStateChange = (
    pub: GlobalAdminPublication,
    state: "activa" | "oculta",
  ) => {
    setTargetPub(pub);
    setNewState(state);
    setStateReason("revision_administrativa");
  };

  const handleConfirmStateChange = async () => {
    if (!targetPub) return;
    const success = await setGlobalPublicationState(targetPub.publication_id, {
      state: newState,
      expected_updated_at: targetPub.updated_at,
      reason: stateReason,
    });
    if (success) {
      setTargetPub(null);
    }
  };

  const handleOpenRetire = (pub: GlobalAdminPublication) => {
    setRetireTargetPub(pub);
    setRetireReason("");
    setRetireError(null);
  };

  const handleConfirmRetire = async () => {
    if (!retireTargetPub) return;
    if (!isValidAdminReason(retireReason)) {
      setRetireError(
        "El motivo de retiro es obligatorio y debe tener entre 5 y 500 caracteres válidos.",
      );
      return;
    }

    const success = await retireGlobalPublication(
      retireTargetPub.publication_id,
      {
        owner_id: retireTargetPub.owner_id,
        expected_updated_at: retireTargetPub.updated_at,
        reason: retireReason.trim(),
      },
    );
    if (success) {
      setRetireTargetPub(null);
      setRetireReason("");
    }
  };

  return (
    <div className="space-y-6">
      <AdminFeedbackBanners />

      {/* Encabezado */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight text-foreground">
            Moderación de Espacios y Publicaciones
          </h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            Control de visibilidad pública, auditoría de contenido y retiro
            administrativo con control de concurrencia optimista.
          </p>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => {
            void loadGlobalPublications();
            void loadPublishedSpaces();
          }}
          disabled={loading}
          className="gap-1.5"
        >
          <RefreshCw
            className={`size-4 ${loading ? "animate-spin" : ""}`}
            aria-hidden="true"
          />
          <span>Actualizar</span>
        </Button>
      </div>

      {/* Barra de Búsqueda y Filtros */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
        <div className="relative flex-1">
          <Search
            className="absolute left-3 top-2.5 size-4 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            type="search"
            placeholder="Buscar por título, categoría o email de anfitrión..."
            value={searchFilter}
            onChange={(e) => setSearchFilter(e.target.value)}
            className="pl-9"
          />
        </div>

        <div className="flex items-center gap-2">
          <Label htmlFor="space-state-filter" className="sr-only">
            Filtrar por estado
          </Label>
          <select
            id="space-state-filter"
            value={stateFilter}
            onChange={(e) => setStateFilter(e.target.value)}
            className="rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <option value="todas">Todos los estados</option>
            <option value="activa">Activas</option>
            <option value="oculta">Ocultas</option>
            <option value="borrador">Borradores</option>
            <option value="eliminada">Retiradas</option>
          </select>
        </div>
      </div>

      {/* Lista de Publicaciones */}
      {filteredPublications.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground text-sm space-y-2">
            <Building2
              className="size-10 mx-auto text-muted-foreground/60"
              aria-hidden="true"
            />
            <p className="font-medium">
              No se encontraron espacios con los filtros actuales.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {filteredPublications.map((pub) => (
            <Card
              key={pub.publication_id}
              className="overflow-hidden shadow-sm hover:shadow transition-shadow"
            >
              <CardHeader className="pb-3 bg-muted/20 border-b border-border/40">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <Badge
                        variant="secondary"
                        className="text-xs font-medium"
                      >
                        {pub.category_name}
                      </Badge>
                      <span className="text-xs text-muted-foreground font-mono">
                        ID: {pub.publication_id.slice(0, 8)}...
                      </span>
                    </div>
                    <CardTitle className="text-base font-semibold">
                      {pub.title}
                    </CardTitle>
                  </div>
                  <div>{renderPubStateBadge(pub.state)}</div>
                </div>
              </CardHeader>
              <CardContent className="pt-4 space-y-4">
                <p className="text-sm text-muted-foreground line-clamp-2">
                  {pub.description}
                </p>

                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                  <div>
                    <span className="text-muted-foreground block">
                      Anfitrión
                    </span>
                    <span
                      className="font-medium truncate block"
                      title={pub.owner_email}
                    >
                      {pub.owner_name ?? pub.owner_email}
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block">
                      Tarifa Base
                    </span>
                    <span className="font-medium">
                      ${Number(pub.base_price_clp).toLocaleString("es-CL")} CLP
                      / {pub.rate_unit}
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block">
                      Superficie y Aforo
                    </span>
                    <span className="font-medium">
                      {pub.area_m2} m² • {pub.capacity} pers.
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block">
                      Última Actualización
                    </span>
                    <span className="font-medium">
                      {new Date(pub.updated_at).toLocaleString("es-CL")}
                    </span>
                  </div>
                </div>

                {/* Control de acciones */}
                <div className="flex flex-wrap items-center justify-end gap-2 pt-2 border-t border-border/40">
                  {pub.state === "oculta" && (
                    <Button
                      type="button"
                      variant="default"
                      size="sm"
                      onClick={() => handleOpenStateChange(pub, "activa")}
                      className="bg-emerald-700 hover:bg-emerald-800 text-white gap-1.5 text-xs"
                    >
                      <Eye className="size-3.5" aria-hidden="true" />
                      <span>Activar Espacio</span>
                    </Button>
                  )}

                  {pub.state === "activa" && (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => handleOpenStateChange(pub, "oculta")}
                      className="text-amber-700 hover:bg-amber-50 dark:text-amber-300 dark:hover:bg-amber-950/40 gap-1.5 text-xs border-amber-300"
                    >
                      <EyeOff className="size-3.5" aria-hidden="true" />
                      <span>Ocultar / Suspender</span>
                    </Button>
                  )}

                  {pub.state !== "eliminada" && (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => handleOpenRetire(pub)}
                      className="text-destructive hover:bg-destructive/10 border-destructive/30 gap-1.5 text-xs"
                    >
                      <Trash2 className="size-3.5" aria-hidden="true" />
                      <span>Retirar Espacio</span>
                    </Button>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Modal de Cambio de Estado con Concurrencia Optimista */}
      <Dialog
        open={targetPub !== null}
        onOpenChange={(open) => !open && setTargetPub(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {newState === "activa" ? (
                <>
                  <CheckCircle2
                    className="size-5 text-emerald-600"
                    aria-hidden="true"
                  />
                  <span>Activar Publicación</span>
                </>
              ) : (
                <>
                  <EyeOff
                    className="size-5 text-amber-600"
                    aria-hidden="true"
                  />
                  <span>Ocultar / Suspender Publicación</span>
                </>
              )}
            </DialogTitle>
            <DialogDescription>
              Cambio de estado administrativo con control de concurrencia
              optimista (expected_updated_at). Si otra sesión modificó el
              recurso, se emitirá un aviso HTTP 409.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="state-reason">
                Motivo administrativo del cambio
              </Label>
              <select
                id="state-reason"
                value={stateReason}
                onChange={(e) =>
                  setStateReason(
                    e.target.value as
                      | "correccion_perfil"
                      | "revision_administrativa"
                      | "gestion_operativa",
                  )
                }
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="revision_administrativa">
                  Revisión administrativa de calidad
                </option>
                <option value="gestion_operativa">
                  Gestión operativa del inventario
                </option>
                <option value="correccion_perfil">
                  Corrección de datos de publicación
                </option>
              </select>
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setTargetPub(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant={newState === "activa" ? "default" : "secondary"}
              onClick={handleConfirmStateChange}
              disabled={actionLoading}
              className={
                newState === "activa"
                  ? "bg-emerald-700 hover:bg-emerald-800 text-white"
                  : ""
              }
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Confirmar {newState === "activa" ? "Activación" : "Suspensión"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Modal de Retiro de Publicación */}
      <Dialog
        open={retireTargetPub !== null}
        onOpenChange={(open) => !open && setRetireTargetPub(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Trash2 className="size-5 text-destructive" aria-hidden="true" />
              <span>Retirar Espacio Definitivamente</span>
            </DialogTitle>
            <DialogDescription>
              Esta acción retira permanentemente el espacio del catálogo
              público. Requiere un motivo obligatorio (mínimo 5 caracteres).
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="retire-reason">
                Motivo obligatorio del retiro
              </Label>
              <Input
                id="retire-reason"
                value={retireReason}
                onChange={(e) => {
                  setRetireReason(e.target.value);
                  if (retireError) setRetireError(null);
                }}
                placeholder="Ej. Incumplimiento de términos o solicitud de retiro por anfitrión..."
                aria-required="true"
                aria-invalid={retireError !== null}
                className={retireError ? "border-destructive" : ""}
              />
              {retireError && (
                <p className="text-xs text-destructive flex items-center gap-1 mt-1">
                  <AlertCircle className="size-3.5" aria-hidden="true" />
                  {retireError}
                </p>
              )}
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setRetireTargetPub(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={handleConfirmRetire}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Confirmar Retiro
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
