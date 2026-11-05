"use client";
import { useCallback, useEffect, useSyncExternalStore } from "react";
import { ReviewFacade } from "../application/review-facade";
import { ReviewOpenApiAdapter } from "../infrastructure/review-openapi-adapter";
import { useAuth } from "@/features/auth/ui/use-auth";
const facade = new ReviewFacade(new ReviewOpenApiAdapter());
export function useDocumentReview() {
  const { snapshot: auth } = useAuth();
  const snapshot = useSyncExternalStore(
    facade.subscribe,
    facade.getSnapshot,
    facade.getServerSnapshot,
  );
  const permissionsKey = JSON.stringify([
    auth.roles,
    auth.capabilities,
    auth.restrictedMode,
    auth.expiresAt,
  ]);
  useEffect(() => {
    facade.setSession(
      auth.status === "authenticated" ? auth.accountId : null,
      permissionsKey,
    );
    return () => facade.clearPrivateData();
  }, [auth.status, auth.accountId, permissionsKey]);
  const load = useCallback(() => facade.load(), []);
  const select = useCallback((id: string) => facade.select(id), []);
  const download = useCallback((id: string) => facade.download(id), []);
  const decide = useCallback(
    (input: Parameters<typeof facade.decide>[0]) => facade.decide(input),
    [],
  );
  const revoke = useCallback(
    (input: Parameters<typeof facade.revoke>[0]) => facade.revoke(input),
    [],
  );
  return { snapshot, auth, load, select, download, decide, revoke };
}
