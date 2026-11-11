"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { clearPartialDraftRecoveryPending } from "./partial-draft-recovery-state";

const CREATE_SPACE_PATH = "/anfitrion/espacios/nuevo";

export function PartialDraftRecoveryRouteReset() {
  const pathname = usePathname();

  useEffect(() => {
    if (pathname !== CREATE_SPACE_PATH) {
      clearPartialDraftRecoveryPending();
    }
  }, [pathname]);

  useEffect(
    () => () => {
      clearPartialDraftRecoveryPending();
    },
    [],
  );

  return null;
}
