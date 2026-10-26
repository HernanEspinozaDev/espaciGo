"use client";

import { useEffect, useState, useTransition } from "react";
import Link from "next/link";
import {
  Plus,
  Edit3,
  Archive,
  Trash2,
  Send,
  Building2,
  AlertCircle,
  CheckCircle2,
  Loader2,
  MapPin,
  Users,
  Maximize2,
} from "lucide-react";
import { Button } from "@/components/workspace/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useHostListings } from "./use-host-listings";
import type { ListingState, ListingSummary } from "../domain/types";

type FilterStatus = "all" | "borrador" | "activa" | "oculta";

const PAGE_SIZE = 6;

export function HostListingsList() {
  const { snapshot: authSnapshot } = useAuth();
  const {
    snapshot,
    loadListings,
    publishListing,
    archiveListing,
    deleteListing,
  } = useHostListings();

  const [activeFilter, setActiveFilter] = useState<FilterStatus>("all");
  const [currentPage, setCurrentPage] = useState<number>(1);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [isPending, startTransition] = useTransition();
  const [operatingId, setOperatingId] = useState<string | null>(null);

  useEffect(() => {
    if (authSnapshot.status === "authenticated") {
      loadListings().catch(() => {
        // Error is set in snapshot.error
      });
    }
  }, [authSnapshot.status, loadListings]);

  const allListings = snapshot.listings;
  const filteredListings = allListings.filter((item) => {
    if (activeFilter === "all") return true;
    return item.state === activeFilter;
  });

  const totalPages = Math.max(
    1,
    Math.ceil(filteredListings.length / PAGE_SIZE),
  );
  const paginatedListings = filteredListings.slice(
    (currentPage - 1) * PAGE_SIZE,
    currentPage * PAGE_SIZE,
  );

  const handleFilterChange = (filter: FilterStatus) => {
    setActiveFilter(filter);
    setCurrentPage(1);
    setActionError(null);
    setActionSuccess(null);
  };

  const handlePublish = async (listingId: string) => {
    if (authSnapshot.status !== "authenticated") return;
    setActionError(null);
    setActionSuccess(null);
    setOperatingId(listingId);
    startTransition(async () => {
      try {
        await publishListing(listingId);
        setActionSuccess("Espacio publicado exitosamente con estado activo.");
      } catch (err) {
        const message =
          err instanceof Error
            ? err.message
            : "No fue posible publicar el espacio";
        setActionError(message);
      } finally {
        setOperatingId(null);
      }
    });
  };

  const handleArchive = async (listingId: string) => {
    if (authSnapshot.status !== "authenticated") return;
    setActionError(null);
    setActionSuccess(null);
    setOperatingId(listingId);
    startTransition(async () => {
      try {
        await archiveListing(listingId);
        setActionSuccess(
          "Espacio archivado. Ya no será visible en el catálogo público.",
        );
      } catch (err) {
        const message =
          err instanceof Error
            ? err.message
            : "No fue posible archivar el espacio";
        setActionError(message);
      } finally {
        setOperatingId(null);
      }
    });
  };

  const handleDeleteDraft = async (listingId: string, title: string) => {
    if (authSnapshot.status !== "authenticated") return;
    const confirmed = window.confirm(
      `¿Estás seguro de eliminar el borrador "${title}"? Esta acción no se puede deshacer.`,
    );
    if (!confirmed) return;

    setActionError(null);
    setActionSuccess(null);
    setOperatingId(listingId);
    startTransition(async () => {
      try {
        await deleteListing(listingId);
        setActionSuccess("Borrador eliminado correctamente.");
      } catch (err) {
        const message =
          err instanceof Error
            ? err.message
            : "No fue posible eliminar el borrador";
        setActionError(message);
      } finally {
        setOperatingId(null);
      }
    });
  };

  const renderStateBadge = (state: ListingState) => {
    switch (state) {
      case "activa":
        return (
          <Badge
            variant="default"
            className="bg-emerald-600 text-white hover:bg-emerald-700"
          >
            Publicado
          </Badge>
        );
      case "oculta":
        return (
          <Badge
            variant="outline"
            className="border-amber-500/40 text-amber-700 bg-amber-50"
          >
            Archivado
          </Badge>
        );
      case "borrador":
      default:
        return (
          <Badge variant="secondary" className="bg-slate-100 text-slate-700">
            Borrador
          </Badge>
        );
    }
  };

  return (
    <div className="space-y-6">
      {/* Cabecera y botón de acción principal */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-2 border-b border-border">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold tracking-tight text-foreground">
            Mis Espacios
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Gestiona tus propiedades, actualiza tarifas, horarios y fotografías.
          </p>
        </div>
        <Button asChild size="lg" className="font-semibold shadow-sm shrink-0">
          <Link
            href="/anfitrion/espacios/nuevo"
            className="inline-flex items-center gap-2"
          >
            <Plus className="size-4" aria-hidden="true" />
            Publicar un espacio
          </Link>
        </Button>
      </div>

      {/* Avisos de éxito o error */}
      {actionSuccess && (
        <div
          role="status"
          className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800 flex items-start gap-3"
        >
          <CheckCircle2
            className="size-5 shrink-0 text-emerald-600 mt-0.5"
            aria-hidden="true"
          />
          <div className="flex-1">{actionSuccess}</div>
          <button
            type="button"
            onClick={() => setActionSuccess(null)}
            className="text-emerald-700 hover:text-emerald-900 text-xs font-semibold"
            aria-label="Cerrar aviso de éxito"
          >
            ✕
          </button>
        </div>
      )}

      {(actionError || snapshot.error) && (
        <div
          role="alert"
          className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive flex items-start gap-3"
        >
          <AlertCircle className="size-5 shrink-0 mt-0.5" aria-hidden="true" />
          <div className="flex-1">{actionError || snapshot.error}</div>
          <button
            type="button"
            onClick={() => setActionError(null)}
            className="text-destructive hover:underline text-xs font-semibold"
            aria-label="Cerrar aviso de error"
          >
            ✕
          </button>
        </div>
      )}

      {/* Barra de Filtros */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div
          role="tablist"
          aria-label="Filtros por estado de publicación"
          className="inline-flex rounded-lg bg-muted p-1 text-muted-foreground"
        >
          <button
            type="button"
            role="tab"
            aria-selected={activeFilter === "all"}
            onClick={() => handleFilterChange("all")}
            className={`px-3 py-1.5 text-xs sm:text-sm font-medium rounded-md transition-all ${
              activeFilter === "all"
                ? "bg-background text-foreground shadow-xs font-semibold"
                : "hover:text-foreground"
            }`}
          >
            Todos ({allListings.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeFilter === "borrador"}
            onClick={() => handleFilterChange("borrador")}
            className={`px-3 py-1.5 text-xs sm:text-sm font-medium rounded-md transition-all ${
              activeFilter === "borrador"
                ? "bg-background text-foreground shadow-xs font-semibold"
                : "hover:text-foreground"
            }`}
          >
            Borradores (
            {allListings.filter((i) => i.state === "borrador").length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeFilter === "activa"}
            onClick={() => handleFilterChange("activa")}
            className={`px-3 py-1.5 text-xs sm:text-sm font-medium rounded-md transition-all ${
              activeFilter === "activa"
                ? "bg-background text-foreground shadow-xs font-semibold"
                : "hover:text-foreground"
            }`}
          >
            Publicados ({allListings.filter((i) => i.state === "activa").length}
            )
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeFilter === "oculta"}
            onClick={() => handleFilterChange("oculta")}
            className={`px-3 py-1.5 text-xs sm:text-sm font-medium rounded-md transition-all ${
              activeFilter === "oculta"
                ? "bg-background text-foreground shadow-xs font-semibold"
                : "hover:text-foreground"
            }`}
          >
            Archivados ({allListings.filter((i) => i.state === "oculta").length}
            )
          </button>
        </div>

        {snapshot.loading && (
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            <span>Actualizando listado...</span>
          </div>
        )}
      </div>

      {/* Listado de tarjetas de espacios */}
      {paginatedListings.length === 0 ? (
        <Card className="border-dashed py-12 text-center">
          <CardContent className="flex flex-col items-center justify-center space-y-4">
            <div className="p-4 rounded-full bg-muted text-muted-foreground">
              <Building2 className="size-8" aria-hidden="true" />
            </div>
            <div className="space-y-1 max-w-md">
              <h2 className="text-lg font-semibold text-foreground">
                {activeFilter === "all"
                  ? "No tienes espacios registrados todavía"
                  : "No hay espacios con el filtro seleccionado"}
              </h2>
              <p className="text-sm text-muted-foreground">
                Publica tus oficinas, bodegas, salas de eventos o
                estacionamientos para comenzar a recibir reservas.
              </p>
            </div>
            <Button asChild className="mt-2">
              <Link href="/anfitrion/espacios/nuevo">
                <Plus className="size-4 mr-1.5" aria-hidden="true" />
                Crear nuevo espacio
              </Link>
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {paginatedListings.map((listing: ListingSummary) => {
            const isOperating = operatingId === listing.id && isPending;
            const formattedPrice = new Intl.NumberFormat("es-CL").format(
              listing.base_price_clp,
            );

            return (
              <Card
                key={listing.id}
                className="flex flex-col justify-between overflow-hidden transition-shadow hover:shadow-md"
              >
                <CardHeader className="space-y-2 pb-3">
                  <div className="flex items-start justify-between gap-2">
                    <Badge variant="outline" className="text-xs font-normal">
                      {listing.category_name || listing.category_code}
                    </Badge>
                    {renderStateBadge(listing.state)}
                  </div>
                  <CardTitle className="text-lg line-clamp-2 leading-snug">
                    <Link
                      href={`/anfitrion/espacios/${listing.id}/editar`}
                      className="hover:text-primary transition-colors"
                    >
                      {listing.title}
                    </Link>
                  </CardTitle>
                  <CardDescription className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <MapPin className="size-3.5 shrink-0" aria-hidden="true" />
                    <span className="truncate">
                      {listing.comuna_code || "Chile"}
                      {listing.region_code
                        ? ` (Región ${listing.region_code})`
                        : ""}
                    </span>
                  </CardDescription>
                </CardHeader>

                <CardContent className="space-y-3 py-2 text-sm">
                  <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground bg-muted/30 p-2.5 rounded-lg">
                    <div className="flex items-center gap-1.5">
                      <Maximize2
                        className="size-3.5 text-foreground/70"
                        aria-hidden="true"
                      />
                      <span>{listing.area_m2} m²</span>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <Users
                        className="size-3.5 text-foreground/70"
                        aria-hidden="true"
                      />
                      <span>Hasta {listing.capacity} pers.</span>
                    </div>
                  </div>

                  <div className="pt-1">
                    <div className="text-xs text-muted-foreground">
                      Tarifa base
                    </div>
                    <div className="text-lg font-bold text-foreground">
                      ${formattedPrice} CLP
                      <span className="text-xs font-normal text-muted-foreground">
                        /{listing.rate_unit}
                      </span>
                    </div>
                  </div>
                </CardContent>

                <CardFooter className="pt-3 border-t border-border flex flex-wrap gap-2 justify-between items-center">
                  <Button
                    asChild
                    variant="outline"
                    size="sm"
                    className="gap-1.5 text-xs flex-1 sm:flex-initial"
                  >
                    <Link href={`/anfitrion/espacios/${listing.id}/editar`}>
                      <Edit3 className="size-3.5" aria-hidden="true" />
                      Editar
                    </Link>
                  </Button>

                  <div className="flex items-center gap-1.5">
                    {listing.state === "borrador" && (
                      <>
                        <Button
                          variant="default"
                          size="sm"
                          disabled={isOperating}
                          onClick={() => handlePublish(listing.id)}
                          className="gap-1 text-xs"
                          aria-label={`Publicar ${listing.title}`}
                        >
                          {isOperating ? (
                            <Loader2
                              className="size-3.5 animate-spin"
                              aria-hidden="true"
                            />
                          ) : (
                            <Send className="size-3.5" aria-hidden="true" />
                          )}
                          Publicar
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          disabled={isOperating}
                          onClick={() =>
                            handleDeleteDraft(listing.id, listing.title)
                          }
                          className="text-destructive hover:bg-destructive/10 text-xs px-2"
                          aria-label={`Eliminar borrador ${listing.title}`}
                        >
                          <Trash2 className="size-3.5" aria-hidden="true" />
                        </Button>
                      </>
                    )}

                    {listing.state === "activa" && (
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={isOperating}
                        onClick={() => handleArchive(listing.id)}
                        className="gap-1 text-xs text-amber-700 hover:bg-amber-50"
                        aria-label={`Archivar ${listing.title}`}
                      >
                        {isOperating ? (
                          <Loader2
                            className="size-3.5 animate-spin"
                            aria-hidden="true"
                          />
                        ) : (
                          <Archive className="size-3.5" aria-hidden="true" />
                        )}
                        Archivar
                      </Button>
                    )}

                    {listing.state === "oculta" && (
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={isOperating}
                        onClick={() => handlePublish(listing.id)}
                        className="gap-1 text-xs text-emerald-700 hover:bg-emerald-50"
                        aria-label={`Reactivar ${listing.title}`}
                      >
                        {isOperating ? (
                          <Loader2
                            className="size-3.5 animate-spin"
                            aria-hidden="true"
                          />
                        ) : (
                          <Send className="size-3.5" aria-hidden="true" />
                        )}
                        Reactivar
                      </Button>
                    )}
                  </div>
                </CardFooter>
              </Card>
            );
          })}
        </div>
      )}

      {/* Paginación simple y accesible */}
      {totalPages > 1 && (
        <nav
          aria-label="Paginación de espacios"
          className="flex items-center justify-center gap-2 pt-6"
        >
          <Button
            variant="outline"
            size="sm"
            disabled={currentPage <= 1}
            onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
            aria-label="Ir a página anterior"
          >
            Anterior
          </Button>
          <span className="text-xs text-muted-foreground px-2">
            Página {currentPage} de {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={currentPage >= totalPages}
            onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
            aria-label="Ir a página siguiente"
          >
            Siguiente
          </Button>
        </nav>
      )}
    </div>
  );
}
