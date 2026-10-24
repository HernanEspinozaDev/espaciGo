import type { HTMLAttributes, ReactNode } from "react";

type StatusKind =
  | "loading"
  | "empty"
  | "error"
  | "success"
  | "denied"
  | "restricted"
  | "offline";

type StatusPanelProps = HTMLAttributes<HTMLElement> & {
  kind: StatusKind;
  title: string;
  children: ReactNode;
};

const symbols: Record<StatusKind, string> = {
  loading: "…",
  empty: "○",
  error: "!",
  success: "✓",
  denied: "×",
  restricted: "Ⅱ",
  offline: "⌁",
};

export function StatusPanel({
  kind,
  title,
  children,
  className = "",
  ...props
}: StatusPanelProps) {
  return (
    <section
      className={`status-panel status-panel--${kind} ${className}`.trim()}
      role={kind === "error" ? "alert" : "status"}
      aria-live={kind === "error" ? "assertive" : "polite"}
      {...props}
    >
      <span className="status-panel__symbol" aria-hidden="true">
        {symbols[kind]}
      </span>
      <div className="status-panel__copy">
        <h3>{title}</h3>
        <div>{children}</div>
      </div>
    </section>
  );
}
