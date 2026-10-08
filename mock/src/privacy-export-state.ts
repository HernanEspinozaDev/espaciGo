export interface PrivacyExportContext {
  accountID: string;
  sessionToken: string;
  generation: number;
}

export function capturePrivacyExportContext(
  accountID: string,
  sessionToken: string,
  generation: number,
): PrivacyExportContext | null {
  if (!accountID || !sessionToken) return null;
  return {accountID, sessionToken, generation};
}

export function privacyExportSessionMatches(
  context: PrivacyExportContext,
  accountID: string,
  sessionToken: string,
  generation: number,
): boolean {
  return Boolean(accountID && sessionToken)
    && context.accountID === accountID
    && context.sessionToken === sessionToken
    && context.generation === generation;
}

export async function deliverPrivacyExportIfCurrent<T>(
  context: PrivacyExportContext,
  load: () => Promise<T>,
  current: (context: PrivacyExportContext) => boolean,
  deliver: (data: T) => void,
): Promise<boolean> {
  let data: T;
  try {
    data = await load();
  } catch (error) {
    if (!current(context)) return false;
    throw error;
  }
  if (!current(context)) return false;
  deliver(data);
  return true;
}
