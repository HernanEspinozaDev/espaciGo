import "client-only";

import type { AuthFacade } from "./auth-facade";

export type AuthBroadcastMessage =
  { type: "logout" } | { type: "session_changed" };

const AUTH_CHANNEL_NAME = "espacigo_auth";

export function broadcastAuthEvent(message: AuthBroadcastMessage): void {
  if (
    typeof window === "undefined" ||
    typeof BroadcastChannel === "undefined"
  ) {
    return;
  }
  try {
    const channel = new BroadcastChannel(AUTH_CHANNEL_NAME);
    channel.postMessage(message);
    channel.close();
  } catch {
    // BroadcastChannel unsupported or blocked
  }
}

function setupAuthLifecycle(facade: AuthFacade): () => void {
  if (typeof window === "undefined") return () => {};

  // Client startup check
  void facade.checkSession().catch(() => false);

  const handleFocus = () => {
    void facade.checkSession().catch(() => false);
  };

  const handleVisibility = () => {
    if (document.visibilityState === "visible") {
      void facade.checkSession().catch(() => false);
    }
  };

  const handlePageShow = () => {
    void facade.checkSession().catch(() => false);
  };

  window.addEventListener("focus", handleFocus);
  document.addEventListener("visibilitychange", handleVisibility);
  window.addEventListener("pageshow", handlePageShow);

  let channel: BroadcastChannel | null = null;
  if (typeof BroadcastChannel !== "undefined") {
    try {
      channel = new BroadcastChannel(AUTH_CHANNEL_NAME);
      channel.onmessage = (event: MessageEvent<AuthBroadcastMessage>) => {
        const data = event.data;
        if (!data || typeof data !== "object") return;
        if (data.type === "logout") {
          facade.handlePeerLogout();
        } else if (data.type === "session_changed") {
          void facade.checkSession().catch(() => false);
        }
      };
    } catch {
      channel = null;
    }
  }

  return () => {
    window.removeEventListener("focus", handleFocus);
    document.removeEventListener("visibilitychange", handleVisibility);
    window.removeEventListener("pageshow", handlePageShow);
    if (channel) {
      channel.close();
      channel = null;
    }
  };
}

let refCount = 0;
let activeCleanup: (() => void) | null = null;

export function retainAuthLifecycle(facade: AuthFacade): () => void {
  if (typeof window === "undefined") return () => {};

  refCount += 1;
  if (refCount === 1) {
    activeCleanup = setupAuthLifecycle(facade);
  }

  return () => {
    refCount -= 1;
    if (refCount <= 0) {
      refCount = 0;
      if (activeCleanup) {
        activeCleanup();
        activeCleanup = null;
      }
    }
  };
}
