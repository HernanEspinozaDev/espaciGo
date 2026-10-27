export interface RegistrationResultData {
  email: string;
  status: 201 | 202;
}

interface StoredRegistrationResult extends RegistrationResultData {
  createdAt: number;
}

const TTL_MS = 5 * 60 * 1000; // 5 minutes

let ephemeralResult: StoredRegistrationResult | null = null;
let cachedSnapshot: RegistrationResultData | null = null;
let clearTimer: ReturnType<typeof setTimeout> | null = null;
let ttlTimer: ReturnType<typeof setTimeout> | null = null;
let currentVersion = 0;
let pendingClearVersion: number | null = null;
const listeners = new Set<() => void>();

function notifyListeners(): void {
  for (const listener of listeners) {
    try {
      listener();
    } catch {
      // ignore
    }
  }
}

export function subscribeRegistrationResult(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function setRegistrationResult(data: RegistrationResultData): void {
  if (typeof window === "undefined") return;
  cancelScheduledClear();
  if (ttlTimer !== null) {
    clearTimeout(ttlTimer);
    ttlTimer = null;
  }
  currentVersion++;
  ephemeralResult = {
    email: data.email,
    status: data.status,
    createdAt: Date.now(),
  };
  cachedSnapshot = Object.freeze({
    email: data.email,
    status: data.status,
  });
  ttlTimer = setTimeout(() => {
    clearRegistrationResult();
  }, TTL_MS);
  notifyListeners();
}

export function getRegistrationResult(): RegistrationResultData | null {
  if (typeof window === "undefined" || !ephemeralResult) return null;
  if (Date.now() - ephemeralResult.createdAt > TTL_MS) {
    clearRegistrationResult();
    return null;
  }
  return cachedSnapshot;
}

export function getRegistrationResultServerSnapshot(): null {
  return null;
}

export function clearRegistrationResult(): void {
  cancelScheduledClear();
  if (ttlTimer !== null) {
    clearTimeout(ttlTimer);
    ttlTimer = null;
  }
  const hadData = ephemeralResult !== null || cachedSnapshot !== null;
  ephemeralResult = null;
  cachedSnapshot = null;
  if (hadData) {
    notifyListeners();
  }
}

export function scheduleClearRegistrationResult(delayMs?: number): void {
  cancelScheduledClear();
  const targetVersion = currentVersion;
  pendingClearVersion = targetVersion;

  if (typeof delayMs === "number" && delayMs > 0) {
    clearTimer = setTimeout(() => {
      if (
        pendingClearVersion === targetVersion &&
        currentVersion === targetVersion
      ) {
        clearRegistrationResult();
      }
      clearTimer = null;
    }, delayMs);
    return;
  }

  queueMicrotask(() => {
    if (
      pendingClearVersion === targetVersion &&
      currentVersion === targetVersion
    ) {
      clearRegistrationResult();
    }
  });
}

export function cancelScheduledClear(): void {
  pendingClearVersion = null;
  if (clearTimer !== null) {
    clearTimeout(clearTimer);
    clearTimer = null;
  }
}
