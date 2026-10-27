"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { ActionCredential } from "../domain/action-credential";
import type { ActionLinkStatus } from "../application/onboarding.port";

export type ActionTerminalOutcome = "used" | "expired" | "invalid";

export function useActionLinkStatus({
  credential,
  inspect,
  onTerminalStatus,
}: {
  credential: ActionCredential | null;
  inspect: (credential: ActionCredential) => Promise<ActionLinkStatus>;
  onTerminalStatus?: (status: ActionTerminalOutcome) => void;
}) {
  const [nonce, setNonce] = useState(0);
  const [probeResult, setProbeResult] = useState<{
    key: string;
    revision: number;
    status: ActionLinkStatus | "probe_error";
  } | null>(null);
  const epochRef = useRef(0);
  const onTerminalStatusRef = useRef(onTerminalStatus);
  const inspectRef = useRef(inspect);

  useEffect(() => {
    onTerminalStatusRef.current = onTerminalStatus;
    inspectRef.current = inspect;
  });

  const credentialKey = credential?.tokenId ?? null;
  const isChecking = Boolean(
    credentialKey &&
    (!probeResult ||
      probeResult.key !== credentialKey ||
      probeResult.revision !== nonce),
  );

  useEffect(() => {
    if (!credential || !credentialKey) return;

    const currentEpoch = ++epochRef.current;
    let active = true;

    inspectRef
      .current(credential)
      .then((status) => {
        if (!active || epochRef.current !== currentEpoch) return;
        if (status === "used" || status === "expired" || status === "invalid") {
          onTerminalStatusRef.current?.(status);
        } else {
          setProbeResult({ key: credentialKey, revision: nonce, status });
        }
      })
      .catch((error) => {
        if (!active || epochRef.current !== currentEpoch) return;
        if (error instanceof DOMException && error.name === "AbortError")
          return;
        setProbeResult({
          key: credentialKey,
          revision: nonce,
          status: "probe_error",
        });
      });

    return () => {
      active = false;
    };
  }, [credential, credentialKey, nonce]);

  const retry = useCallback(() => {
    setNonce((prev) => prev + 1);
  }, []);

  const reset = useCallback(() => {
    epochRef.current += 1;
    setNonce((prev) => prev + 1);
    setProbeResult(null);
  }, []);

  return {
    isChecking,
    isReady: Boolean(
      credentialKey &&
      !isChecking &&
      probeResult?.key === credentialKey &&
      probeResult.revision === nonce &&
      probeResult.status === "ready",
    ),
    isProbeError: Boolean(
      credentialKey &&
      !isChecking &&
      probeResult?.key === credentialKey &&
      probeResult.revision === nonce &&
      probeResult.status === "probe_error",
    ),
    retry,
    reset,
  };
}
