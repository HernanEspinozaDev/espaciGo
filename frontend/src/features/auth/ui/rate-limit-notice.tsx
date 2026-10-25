"use client";

import { useEffect, useState } from "react";
import { remainingRetrySeconds } from "../domain/retry-after";

export function RateLimitNotice({ deadline }: { deadline: number | null }) {
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
  if (deadline === null) return null;
  const remaining = remainingRetrySeconds(deadline, now);
  if (remaining === 0) return null;
  return (
    <p className="text-sm text-amber-900" aria-live="off">
      Puedes volver a intentarlo en aproximadamente {remaining} segundos.
    </p>
  );
}
