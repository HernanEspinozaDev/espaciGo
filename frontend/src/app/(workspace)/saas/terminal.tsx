"use client";

import { useEffect, useState } from "react";
import { Check, Copy } from "lucide-react";

const terminalSteps = [
  "pnpm install --frozen-lockfile",
  "pnpm lint",
  "pnpm typecheck",
  "pnpm test",
  "pnpm build",
  "pnpm test:e2e",
];

export function Terminal() {
  const [terminalStep, setTerminalStep] = useState(0);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => {
      setTerminalStep((previous) =>
        Math.min(previous + 1, terminalSteps.length - 1),
      );
    }, 500);
    return () => clearTimeout(timer);
  }, [terminalStep]);

  const copyToClipboard = async () => {
    await navigator.clipboard.writeText(terminalSteps.join("\n"));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="w-full rounded-lg shadow-lg overflow-hidden bg-gray-900 text-white font-mono text-sm relative">
      <div className="p-4">
        <div className="flex justify-between items-center mb-4">
          <div className="flex space-x-2" aria-hidden="true">
            <div className="w-3 h-3 rounded-full bg-red-500" />
            <div className="w-3 h-3 rounded-full bg-yellow-500" />
            <div className="w-3 h-3 rounded-full bg-green-500" />
          </div>
          <button
            type="button"
            onClick={copyToClipboard}
            className="min-h-11 min-w-11 flex items-center justify-center text-gray-400 hover:text-white transition-colors motion-reduce:transition-none"
            aria-label="Copiar comandos locales"
          >
            {copied ? (
              <Check className="h-5 w-5" aria-hidden="true" />
            ) : (
              <Copy className="h-5 w-5" aria-hidden="true" />
            )}
          </button>
        </div>
        <p className="mb-3 text-xs text-gray-300">
          Comprobaciones locales disponibles en este proyecto:
        </p>
        <div className="space-y-2" aria-live="polite">
          {terminalSteps.map((step, index) => (
            <div
              key={step}
              className={`${index > terminalStep ? "opacity-0" : "opacity-100"} transition-opacity duration-300 motion-reduce:transition-none`}
            >
              <span className="text-green-400" aria-hidden="true">
                $
              </span>{" "}
              {step}
            </div>
          ))}
        </div>
        {copied && (
          <p className="sr-only" role="status">
            Comandos copiados
          </p>
        )}
      </div>
    </div>
  );
}
