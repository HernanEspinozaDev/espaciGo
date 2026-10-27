"use client";

let verifiedInMemory = false;
let clearTimer: ReturnType<typeof setTimeout> | null = null;
let clearVersion = 0;
const listeners = new Set<() => void>();

function notify(): void {
  listeners.forEach((listener) => {
    try {
      listener();
    } catch {
      // ignore
    }
  });
}

export function markVerificationCompleted(): void {
  cancelScheduledClearVerificationCompleted();
  verifiedInMemory = true;
  notify();
}

export function clearVerificationCompleted(): void {
  cancelScheduledClearVerificationCompleted();
  verifiedInMemory = false;
  notify();
}

export function scheduleClearVerificationCompleted(delayMs = 0): void {
  cancelScheduledClearVerificationCompleted();
  const version = clearVersion;
  const clear = () => {
    if (version !== clearVersion) return;
    clearVerificationCompleted();
  };
  if (delayMs > 0) {
    clearTimer = setTimeout(clear, delayMs);
  } else {
    queueMicrotask(clear);
  }
}

export function cancelScheduledClearVerificationCompleted(): void {
  clearVersion += 1;
  if (clearTimer !== null) {
    clearTimeout(clearTimer);
    clearTimer = null;
  }
}

export function isVerificationCompletedInMemory(): boolean {
  return verifiedInMemory;
}

export function subscribeVerificationResult(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function getVerificationResultSnapshot(): boolean {
  return verifiedInMemory;
}

export function getServerVerificationResultSnapshot(): boolean {
  return false;
}
