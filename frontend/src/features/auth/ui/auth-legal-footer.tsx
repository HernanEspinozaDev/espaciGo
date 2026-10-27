import Link from "next/link";

export function AuthLegalFooter({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <footer
      aria-label="Información legal"
      className="mt-6 text-center text-xs text-gray-600"
    >
      <nav
        aria-label="Enlaces legales"
        className="flex flex-wrap items-center justify-center gap-x-3 gap-y-1"
      >
        <Link
          href="/terminos-y-condiciones"
          onNavigate={onNavigate}
          className="inline-flex min-h-11 items-center rounded-sm underline underline-offset-4 hover:text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700"
        >
          Términos y condiciones
        </Link>
        <span aria-hidden="true" className="text-gray-400">
          ·
        </span>
        <Link
          href="/politica-de-privacidad"
          onNavigate={onNavigate}
          className="inline-flex min-h-11 items-center rounded-sm underline underline-offset-4 hover:text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700"
        >
          Política de privacidad
        </Link>
      </nav>
    </footer>
  );
}
