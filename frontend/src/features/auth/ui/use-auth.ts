"use client";

import { useCallback, useSyncExternalStore } from "react";
import { authFacade } from "../auth-runtime";
import type { AuthSnapshot } from "../application/auth-facade";
import type {
  LoginRequest,
  SessionCapabilities,
} from "../application/auth.port";

type Capability = SessionCapabilities[number];

export interface AuthView {
  snapshot: AuthSnapshot;
  login(input: LoginRequest): Promise<boolean>;
  checkSession(): Promise<boolean>;
  logout(): Promise<void>;
  hasCapability(capability: Capability): boolean;
  canRenderCapability(capability: Capability): boolean;
}

export function useAuth(): AuthView {
  const snapshot = useSyncExternalStore(
    authFacade.subscribe,
    authFacade.getSnapshot,
    authFacade.getServerSnapshot,
  );
  const login = useCallback(
    (input: LoginRequest) => authFacade.login(input),
    [],
  );
  const checkSession = useCallback(() => authFacade.checkSession(), []);
  const logout = useCallback(() => authFacade.logout(), []);
  const hasCapability = useCallback(
    (capability: Capability) => authFacade.hasCapability(capability),
    [],
  );
  const canRenderCapability = useCallback(
    (capability: Capability) => authFacade.canRenderCapability(capability),
    [],
  );

  return {
    snapshot,
    login,
    checkSession,
    logout,
    hasCapability,
    canRenderCapability,
  };
}
