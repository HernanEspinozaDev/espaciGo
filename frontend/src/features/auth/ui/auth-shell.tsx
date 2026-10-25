import Link from "next/link";
import { CircleIcon } from "lucide-react";
import type { ReactNode } from "react";

export function AuthShell({
  title,
  description,
  children,
  secondaryLabel,
  secondaryHref,
  secondaryQuestion,
}: {
  title: string;
  description?: string;
  children: ReactNode;
  secondaryLabel: string;
  secondaryHref: string;
  secondaryQuestion: string;
}) {
  return (
    <main
      id="main-content"
      tabIndex={-1}
      className="flex min-h-[100dvh] flex-col justify-center bg-gray-50 px-4 py-12 sm:px-6 lg:px-8"
    >
      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        <Link
          href="/"
          className="flex min-h-11 items-center justify-center gap-2 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700"
          aria-label="EspaciGo, volver al marketplace"
        >
          <CircleIcon
            className="h-12 w-12 text-orange-500"
            aria-hidden="true"
          />
          <span className="text-xl font-semibold text-gray-900">EspaciGo</span>
        </Link>
        <h1 className="mt-6 text-center text-3xl font-extrabold text-gray-900">
          {title}
        </h1>
        {description ? (
          <p className="mt-3 text-center text-sm leading-relaxed text-gray-600">
            {description}
          </p>
        ) : null}
      </div>

      <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
        {children}
        <div className="mt-6">
          <div className="relative">
            <div
              className="absolute inset-0 flex items-center"
              aria-hidden="true"
            >
              <div className="w-full border-t border-gray-300" />
            </div>
            <div className="relative flex justify-center text-sm">
              <span className="bg-gray-50 px-2 text-gray-500">
                {secondaryQuestion}
              </span>
            </div>
          </div>
          <Link
            href={secondaryHref}
            className="mt-6 inline-flex min-h-11 w-full items-center justify-center rounded-full border border-gray-300 bg-white px-4 py-2 text-center text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 focus-visible:ring-offset-2"
          >
            {secondaryLabel}
          </Link>
        </div>
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
