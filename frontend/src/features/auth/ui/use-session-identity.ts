"use client";

import { useSyncExternalStore } from "react";
import { useAuth } from "./use-auth";
import { sessionIdentityStore } from "../session-identity-runtime";
import type {
  AuthStatus,
  SessionIdentitySnapshot,
} from "../application/session-identity";

export type { AuthStatus, SessionIdentitySnapshot };

export function useSessionIdentity(): SessionIdentitySnapshot {
  useAuth();
  return useSyncExternalStore(
    sessionIdentityStore.subscribe,
    sessionIdentityStore.getSnapshot,
    sessionIdentityStore.getServerSnapshot,
  );
}
