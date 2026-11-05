"use client";

import { useEffect } from "react";
import Link from "next/link";
import { Plus } from "lucide-react";
import { Button } from "@/components/workspace/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useHostListings } from "./use-host-listings";

export function AccountSpacesCard() {
  const { snapshot: auth } = useAuth();
  const { snapshot, loadListings } = useHostListings();

  useEffect(() => {
    if (auth.status === "authenticated") {
      void loadListings().catch(() => undefined);
    }
  }, [auth.status, loadListings]);

  const listings = snapshot.listings;
  const counts = {
    borrador: listings.filter((item) => item.state === "borrador").length,
    activa: listings.filter((item) => item.state === "activa").length,
    oculta: listings.filter((item) => item.state === "oculta").length,
  };

  return (
    <section aria-label="Mis espacios">
      <Card>
        <CardHeader>
          <CardTitle>Mis espacios</CardTitle>
          <p className="text-sm text-muted-foreground">
            Resumen de tus espacios
          </p>
        </CardHeader>
        <CardContent className="space-y-4">
          {auth.status === "authenticated" && snapshot.loading ? (
            <p role="status" className="text-sm text-muted-foreground">
              Cargando tus espacios…
            </p>
          ) : auth.status === "authenticated" && snapshot.error ? (
            <div role="alert" className="space-y-2 text-sm">
              <p>No pudimos cargar tus espacios.</p>
              <button
                type="button"
                className="underline"
                onClick={() => void loadListings().catch(() => undefined)}
              >
                Intentar de nuevo
              </button>
            </div>
          ) : auth.status !== "authenticated" ? (
            <p className="text-sm text-muted-foreground">
              Inicia sesión para ver tus espacios.
            </p>
          ) : listings.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              Aún no tienes espacios. Crea uno para comenzar.
            </p>
          ) : (
            <ul className="grid gap-2 text-sm sm:grid-cols-3">
              <li>
                {counts.borrador}{" "}
                {counts.borrador === 1 ? "borrador" : "borradores"}
              </li>
              <li>
                {counts.activa}{" "}
                {counts.activa === 1 ? "publicado" : "publicados"}
              </li>
              <li>
                {counts.oculta}{" "}
                {counts.oculta === 1 ? "archivado" : "archivados"}
              </li>
            </ul>
          )}
          <div className="flex flex-wrap gap-3">
            <Button asChild>
              <Link href="/anfitrion/espacios/nuevo">
                <Plus className="size-4" aria-hidden="true" />
                Crear espacio
              </Link>
            </Button>
            <Button asChild variant="outline">
              <Link href="/anfitrion/espacios">Ver mis espacios</Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
