import type { Metadata } from "next";
import { HostNavigation } from "./host-navigation";

export const metadata: Metadata = {
  title: "Panel Anfitrión | EspaciGo",
  description:
    "Gestión de propiedades, tarifas, calendarios y publicaciones de anfitrión.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AnfitrionLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <main className="container mx-auto px-4 py-8 max-w-5xl space-y-6">
      <HostNavigation />
      <section>{children}</section>
    </main>
  );
}
