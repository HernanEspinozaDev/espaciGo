declare const clpAmountBrand: unique symbol;

export type ClpAmount = string & { readonly [clpAmountBrand]: true };

export function clpAmount(value: string | number | bigint): ClpAmount {
  if (typeof value === "number" && !Number.isSafeInteger(value)) {
    throw new RangeError("CLP amount cannot pass through an unsafe number");
  }
  const text = String(value);
  if (!/^-?\d+$/.test(text))
    throw new TypeError("CLP amount must be an integer");
  return text as ClpAmount;
}

export function formatClp(value: ClpAmount): string {
  const negative = value.startsWith("-");
  const digits = negative ? value.slice(1) : value;
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `${negative ? "−" : ""}$${grouped}`;
}

export function addClp(left: ClpAmount, right: ClpAmount): ClpAmount {
  return (BigInt(left) + BigInt(right)).toString() as ClpAmount;
}
