"use client";

import { useLayoutEffect, useRef, useState } from "react";
import {
  createActionCredential,
  parseActionFragment,
  type ActionCredential,
} from "../domain/action-credential";

export function useActionCredential(
  onCapture?: (credential: ActionCredential | null) => void,
) {
  const [credential, setCredential] = useState<ActionCredential | null>(null);
  const credentialRef = useRef<ActionCredential | null>(null);
  const capturedRef = useRef<ActionCredential | null>(null);
  const mountedRef = useRef(false);
  const initialCaptureRef = useRef(false);
  const onCaptureRef = useRef(onCapture);

  useLayoutEffect(() => {
    onCaptureRef.current = onCapture;
  });

  useLayoutEffect(() => {
    mountedRef.current = true;

    const captureAndScrub = () => {
      const parsed = parseActionFragment(window.location.hash);
      window.history.replaceState(
        window.history.state,
        "",
        window.location.pathname,
      );
      const next = parsed.ok ? parsed.credential : null;
      capturedRef.current = next;
      credentialRef.current = next;
      setCredential(next);
      onCaptureRef.current?.(next);
    };

    if (!initialCaptureRef.current) {
      initialCaptureRef.current = true;
      captureAndScrub();
    } else {
      const captured = capturedRef.current;
      queueMicrotask(() => {
        if (mountedRef.current) setCredential(captured);
      });
    }

    window.addEventListener("hashchange", captureAndScrub);
    return () => {
      window.removeEventListener("hashchange", captureAndScrub);
      mountedRef.current = false;
      queueMicrotask(() => {
        if (!mountedRef.current) {
          credentialRef.current = null;
          capturedRef.current = null;
        }
      });
    };
  }, []);

  function setManual(tokenId: string, token: string): boolean {
    const parsed = createActionCredential(tokenId, token);
    if (!parsed.ok) return false;
    capturedRef.current = parsed.credential;
    credentialRef.current = parsed.credential;
    setCredential(parsed.credential);
    return true;
  }

  function clear(): void {
    capturedRef.current = null;
    credentialRef.current = null;
    setCredential(null);
  }

  return { credential, setManual, clear };
}
