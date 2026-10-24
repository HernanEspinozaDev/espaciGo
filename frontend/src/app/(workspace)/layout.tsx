import type { Metadata } from "next";
import type { ReactNode } from "react";
import { WorkspaceShell } from "@/components/workspace/workspace-shell";

export const metadata: Metadata = {
  title: "Panel de muestra",
  description: "Shell de administración demostrativo y sin sesión.",
  robots: { index: false, follow: false },
};

export default function WorkspaceLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return <WorkspaceShell>{children}</WorkspaceShell>;
}
