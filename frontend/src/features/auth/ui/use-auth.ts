"use client";

import { useCallback, useEffect, useSyncExternalStore } from "react";
import { authFacade } from "../auth-runtime";
import { retainAuthLifecycle } from "../application/auth-lifecycle";
import type { AuthSnapshot } from "../application/auth-facade";
import type {
  LoginRequest,
  AccessToken,
  SessionCapabilities,
} from "../application/auth.port";

type Capability = SessionCapabilities[number];

export interface AuthView {
  snapshot: AuthSnapshot;
  accessToken: AccessToken | null;
  getAccessToken(): AccessToken | null;
  login(input: LoginRequest): Promise<boolean>;
  checkSession(): Promise<boolean>;
  logout(): Promise<void>;
  hasCapability(capability: Capability): boolean;
  canRenderCapability(capability: Capability): boolean;
}

export function useAuth(): AuthView {
  useEffect(() => {
    return retainAuthLifecycle(authFacade);
  }, []);

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
  const getAccessToken = useCallback(() => authFacade.getAccessToken(), []);
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
    accessToken: authFacade.getAccessToken(),
    getAccessToken,
    login,
    checkSession,
    logout,
    hasCapability,
    canRenderCapability,
  };
}
