"use client";

import { useEffect, useState } from "react";
import { remainingRetrySeconds } from "../domain/retry-after";

export function useRetryRemaining(deadline: number | null): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (deadline === null) return;
    const immediate = window.setTimeout(() => setNow(Date.now()), 0);
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => {
      window.clearTimeout(immediate);
      window.clearInterval(timer);
    };
  }, [deadline]);
  return deadline === null ? 0 : remainingRetrySeconds(deadline, now);
}
