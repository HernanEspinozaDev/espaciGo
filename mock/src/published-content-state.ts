export type PublishedContentBody = { title?: string; description?: string; capacity?: number; usage_rules?: string; base_price_clp?: number };
export type PublishedContentSnapshot = { title: string; description: string; capacity: number; usage_rules: string; base_price_clp: number };
export type PublishedContentForm = { title: string; description: string; capacity: string; usage_rules: string; base_price_clp: string };

export type PublishedContentChange =
  | { kind: "unchanged" }
  | { kind: "unsupported-price" }
  | { kind: "invalid-capacity" }
  | { kind: "update"; body: PublishedContentBody };

// Build a partial PATCH-like PUT body from the values shown when the form was
// rendered. CLP stays an integer; the mock refuses values beyond JS's exact
// integer range instead of silently rounding an int64 from the API.
export function publishedContentChange(
  loaded: PublishedContentSnapshot,
  next: PublishedContentForm,
): PublishedContentChange {
  const titleChanged = next.title !== loaded.title;
  const descriptionChanged = next.description !== loaded.description;
  const rulesChanged = next.usage_rules !== loaded.usage_rules;
  const capacityChanged = next.capacity !== String(loaded.capacity);
  // If the API value is outside Number's exact integer range, the UI keeps
  // the control blank; blank then means "leave this stored value untouched".
  const priceChanged = Number.isSafeInteger(loaded.base_price_clp)
    ? next.base_price_clp !== String(loaded.base_price_clp)
    : next.base_price_clp !== "";
  if (!titleChanged && !descriptionChanged && !rulesChanged && !capacityChanged && !priceChanged) return { kind: "unchanged" };

  const body: PublishedContentBody = {};
  if (titleChanged) body.title = next.title;
  if (descriptionChanged) body.description = next.description;
  if (rulesChanged) body.usage_rules = next.usage_rules;
  if (capacityChanged) {
    if (!/^\d+$/.test(next.capacity)) return { kind: "invalid-capacity" };
    const exactCapacity = BigInt(next.capacity);
    if (exactCapacity < 1n || exactCapacity > 2147483647n) return { kind: "invalid-capacity" };
    body.capacity = Number(exactCapacity);
  }
  if (priceChanged) {
    if (!/^\d+$/.test(next.base_price_clp)) return { kind: "unsupported-price" };
    const exactPrice = BigInt(next.base_price_clp);
    if (exactPrice < 5001n || exactPrice > BigInt(Number.MAX_SAFE_INTEGER)) {
      return { kind: "unsupported-price" };
    }
    body.base_price_clp = Number(exactPrice);
  }
  return { kind: "update", body };
}
