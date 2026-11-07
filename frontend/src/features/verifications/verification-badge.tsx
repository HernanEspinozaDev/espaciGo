export function VerificationBadge({ status }: { status: "unverified" | "pending" | "in_review" | "verified" | "rejected" }) {
  const style = { unverified: "border-border bg-muted text-foreground", pending: "border-amber-300 bg-amber-50 text-amber-900", in_review: "border-amber-300 bg-amber-50 text-amber-900", verified: "border-emerald-300 bg-emerald-50 text-emerald-800", rejected: "border-red-300 bg-red-50 text-red-800" }[status];
  const label = { unverified: "Identidad no verificada", pending: "Verificación pendiente", in_review: "En revisión", verified: "Identidad verificada", rejected: "Verificación rechazada" }[status];
  return <span className={`inline-flex rounded-full border px-3 py-1 text-sm font-medium ${style}`}>{label}</span>;
}
