export type PublishedContentBody = { title?: string; base_price_clp?: number };

export type PublishedContentChange =
  | { kind: "unchanged" }
  | { kind: "unsupported-price" }
  | { kind: "update"; body: PublishedContentBody };

// Build a partial PATCH-like PUT body from the values shown when the form was
// rendered. CLP stays an integer; the mock refuses values beyond JS's exact
// integer range instead of silently rounding an int64 from the API.
export function publishedContentChange(
  loadedTitle: string,
  loadedPrice: number,
  nextTitle: string,
  nextPriceText: string,
): PublishedContentChange {
  const titleChanged = nextTitle !== loadedTitle;
  // If the API value is outside Number's exact integer range, the UI keeps
  // the control blank; blank then means "leave this stored value untouched".
  const priceChanged = Number.isSafeInteger(loadedPrice)
    ? nextPriceText !== String(loadedPrice)
    : nextPriceText !== "";
  if (!titleChanged && !priceChanged) return { kind: "unchanged" };

  const body: PublishedContentBody = {};
  if (titleChanged) body.title = nextTitle;
  if (priceChanged) {
    if (!/^\d+$/.test(nextPriceText)) return { kind: "unsupported-price" };
    const exactPrice = BigInt(nextPriceText);
    if (exactPrice < 5001n || exactPrice > BigInt(Number.MAX_SAFE_INTEGER)) {
      return { kind: "unsupported-price" };
    }
    body.base_price_clp = Number(exactPrice);
  }
  return { kind: "update", body };
}
