"use client";

import { Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";

export function PasswordField({
  id,
  label,
  value,
  onChange,
  autoComplete,
  disabled = false,
  describedBy,
  invalid = false,
  required = true,
}: {
  id: string;
  label: string;
  value: string;
  onChange(value: string): void;
  autoComplete: "current-password" | "new-password";
  disabled?: boolean;
  describedBy?: string;
  invalid?: boolean;
  required?: boolean;
}) {
  const [visible, setVisible] = useState(false);
  return (
    <div>
      <Label htmlFor={id} className="block text-sm font-medium text-gray-700">
        {label}
      </Label>
      <div className="relative mt-1">
        <Input
          id={id}
          name={id}
          type={visible ? "text" : "password"}
          autoComplete={autoComplete}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          disabled={disabled}
          required={required}
          aria-invalid={invalid || undefined}
          aria-describedby={describedBy}
          className="relative block min-h-11 w-full rounded-full border border-gray-300 px-3 py-2 pr-14 text-gray-900 placeholder:text-gray-500 focus:z-10 focus:border-orange-600 focus:outline-none focus:ring-orange-600 sm:text-sm"
        />
        <button
          type="button"
          aria-label={
            visible
              ? `Ocultar ${label.toLocaleLowerCase("es")}`
              : `Mostrar ${label.toLocaleLowerCase("es")}`
          }
          aria-pressed={visible}
          disabled={disabled}
          onClick={() => setVisible((current) => !current)}
          className="absolute right-1 top-1 inline-flex size-10 items-center justify-center rounded-full text-gray-700 hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-orange-700 disabled:opacity-50"
        >
          {visible ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
        </button>
      </div>
    </div>
  );
}

export function PasswordRequirements({ value }: { value: string }) {
  const chars = Array.from(value).length >= 8;
  const uppercase = /\p{Lu}/u.test(value);
  const digit = /\p{Nd}/u.test(value);
  const punctuation = /[\p{P}\p{S}]/u.test(value);
  const bytes = new TextEncoder().encode(value).byteLength <= 72;
  const requirements = [
    ["Al menos 8 caracteres", chars],
    ["Una letra mayúscula", uppercase],
    ["Un número", digit],
    ["Un signo o puntuación", punctuation],
    ["Dentro del largo máximo permitido", bytes],
  ] as const;
  return (
    <ul
      id="password-requirements"
      className="mt-2 list-inside list-disc space-y-1 text-sm text-gray-700"
    >
      {requirements.map(([label, met]) => (
        <li key={label}>
          <span>{label}</span>
          <span className="sr-only">: {met ? "cumplido" : "pendiente"}</span>
        </li>
      ))}
    </ul>
  );
}
