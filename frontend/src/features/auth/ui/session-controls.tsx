"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/workspace/ui/button";
import { useAuth } from "./use-auth";
import { useSessionIdentity } from "./use-session-identity";

export interface SessionControlsProps {
  variant: "workspace" | "public";
}

export function SessionControls({ variant }: SessionControlsProps) {
  const { status, accountId, label } = useSessionIdentity();
  const { logout } = useAuth();
  const [loggingOut, setLoggingOut] = useState(false);

  const handleLogout = async () => {
    if (loggingOut) return;
    setLoggingOut(true);
    try {
      await logout();
    } catch {
      // useAuth.logout handles local clear and sets pending without false remote success
    } finally {
      setLoggingOut(false);
    }
  };

  if (status === "checking") {
    if (variant === "workspace") {
      return (
        <div
          data-testid="session-controls-checking"
          className="inline-flex h-9 w-20 rounded-full"
          aria-hidden="true"
        />
      );
    }
    return (
      <div
        data-testid="session-controls-checking"
        className="inline-flex min-h-11 w-32 rounded-full"
        aria-hidden="true"
      />
    );
  }

  if (status === "authenticated") {
    const displayIdentity =
      (label && label.length > 0 ? label : accountId) ?? "";

    if (variant === "workspace") {
      return (
        <div className="flex items-center gap-2 sm:gap-4 min-w-0">
          <span
            className="inline-block max-w-[110px] min-[390px]:max-w-[130px] sm:max-w-[200px] truncate text-sm font-medium text-gray-700"
            title={displayIdentity || undefined}
            aria-label={displayIdentity || undefined}
          >
            {displayIdentity}
          </span>
          <Button
            type="button"
            variant="ghost"
            disabled={loggingOut}
            onClick={() => void handleLogout()}
            className="rounded-full text-sm font-medium text-gray-700 hover:text-gray-900"
          >
            Cerrar sesión
          </Button>
        </div>
      );
    }

    return (
      <div className="flex items-center gap-2 sm:gap-3 min-w-0">
        <span
          className="inline-block max-w-[75px] min-[390px]:max-w-[95px] sm:max-w-[180px] truncate text-sm font-medium text-foreground"
          title={displayIdentity || undefined}
          aria-label={displayIdentity || undefined}
        >
          {displayIdentity}
        </span>
        <Link
          href="/mi-cuenta"
          className="inline-flex min-h-11 items-center rounded-full border border-border px-4 text-sm font-medium transition-colors hover:bg-secondary"
        >
          Mi cuenta
        </Link>
      </div>
    );
  }

  // status === "anonymous" (guest)
  if (variant === "workspace") {
    return (
      <Button asChild className="rounded-full">
        <Link href="/acceso">Acceso</Link>
      </Button>
    );
  }

  return (
    <Link
      href="/acceso"
      className="inline-flex min-h-11 items-center rounded-full border border-border px-4 text-sm font-medium transition-colors hover:bg-secondary"
    >
      Ingresar a EspaciGo
    </Link>
  );
}
