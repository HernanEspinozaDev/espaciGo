"use client";

import { Fragment, type ReactNode } from "react";
import { useAuth } from "./use-auth";

export function PrivateSessionBoundary({ children }: { children: ReactNode }) {
  const { snapshot } = useAuth();
  const { status, accountId, roles, capabilities, restrictedMode, expiresAt } =
    snapshot;
  const hasIdentity = status === "authenticated" && accountId !== null;
  const sessionKey = hasIdentity
    ? JSON.stringify([
        status,
        accountId,
        roles,
        capabilities,
        restrictedMode,
        expiresAt,
      ])
    : "guest";

  return <Fragment key={sessionKey}>{children}</Fragment>;
}
