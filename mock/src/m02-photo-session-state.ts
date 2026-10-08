export interface M02PhotoSession {
  token: string;
  account: string;
  generation: number;
}

export function captureM02PhotoSession(token: string, account: string, generation: number): M02PhotoSession | null {
  return token && account ? {token, account, generation} : null;
}

export function m02PhotoSessionMatches(
  captured: M02PhotoSession,
  token: string,
  account: string,
  generation: number,
): boolean {
  return Boolean(token && account)
    && captured.token === token
    && captured.account === account
    && captured.generation === generation;
}

/** Delivers an asynchronous photo result only while the exact requesting session is current. */
export async function deliverM02PhotoIfCurrent<T>(
  captured: M02PhotoSession,
  load: () => Promise<T>,
  current: (captured: M02PhotoSession) => boolean,
  deliver: (result: T) => void,
): Promise<boolean> {
  let result: T;
  try {
    result = await load();
  } catch (error) {
    if (!current(captured)) return false;
    throw error;
  }
  return applyM02PhotoIfCurrent(captured, current, () => deliver(result));
}

export function applyM02PhotoIfCurrent(
  captured: M02PhotoSession,
  current: (captured: M02PhotoSession) => boolean,
  apply: () => void,
): boolean {
  if (!current(captured)) return false;
  apply();
  return true;
}
