import Image from "next/image";
import Link from "next/link";
import type { ReactNode } from "react";
import { AuthLegalFooter } from "./auth-legal-footer";

export { AuthLegalFooter } from "./auth-legal-footer";

export function AuthShell({
  title,
  description,
  children,
  secondaryLabel,
  secondaryHref,
  secondaryQuestion,
  onNavigateAway,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  secondaryLabel: string;
  secondaryHref: string;
  secondaryQuestion: string;
  onNavigateAway?: () => void;
}) {
  return (
    <main
      id="main-content"
      tabIndex={-1}
      className="flex min-h-[100dvh] flex-col justify-center bg-gray-50 px-4 py-8 sm:px-6 sm:py-12 lg:px-8"
    >
      <div className="sm:mx-auto sm:w-full sm:max-w-md text-center">
        <Link
          href="/"
          onNavigate={onNavigateAway}
          className="inline-flex min-h-11 items-center justify-center gap-2.5 rounded-lg px-3 py-1.5 text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700"
          aria-label="EspaciGo, volver al marketplace"
        >
          <Image
            src="/espacigo-mark.svg"
            alt=""
            width={32}
            height={32}
            priority
            className="h-8 w-8 shrink-0"
          />
          <span className="text-xl font-bold tracking-tight text-gray-900">
            EspaciGo
          </span>
        </Link>
      </div>

      <div className="mt-6 sm:mx-auto sm:w-full sm:max-w-md">
        <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-xs sm:p-8">
          <div className="mb-6 text-left">
            <h1 className="text-2xl font-bold tracking-tight text-gray-900">
              {title}
            </h1>
            {description ? (
              <p className="mt-2 text-sm leading-relaxed text-gray-600">
                {description}
              </p>
            ) : null}
          </div>

          {children}

          <div className="mt-6">
            <div className="relative">
              <div
                className="absolute inset-0 flex items-center"
                aria-hidden="true"
              >
                <div className="w-full border-t border-gray-200" />
              </div>
              <div className="relative flex justify-center text-sm">
                <span className="bg-white px-2 text-gray-500">
                  {secondaryQuestion}
                </span>
              </div>
            </div>
            <Link
              href={secondaryHref}
              onNavigate={onNavigateAway}
              className="mt-6 inline-flex min-h-11 w-full items-center justify-center rounded-full border border-gray-300 bg-white px-4 py-2 text-center text-sm font-medium text-gray-700 shadow-xs hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
            >
              {secondaryLabel}
            </Link>
          </div>
        </div>

        <AuthLegalFooter onNavigate={onNavigateAway} />
      </div>
    </main>
  );
}

export function AuthFeedback({
  message,
  kind = "error",
  focusRef,
}: {
  message: string;
  kind?: "error" | "status";
  focusRef?: React.RefObject<HTMLDivElement | null>;
}) {
  if (!message) return null;
  return (
    <div
      ref={focusRef}
      role={kind === "error" ? "alert" : "status"}
      tabIndex={-1}
      className={
        kind === "error"
          ? "rounded-lg border border-red-300 bg-red-50 p-3 text-sm text-red-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-700"
          : "rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-700"
      }
    >
      {message}
    </div>
  );
}
