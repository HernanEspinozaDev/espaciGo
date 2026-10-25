import type { Metadata } from "next";
import type { ReactNode } from "react";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "EspaciGo", template: "%s | EspaciGo" },
  description:
    "Explora arriendos residenciales y espacios flexibles publicados en EspaciGo.",
  applicationName: "EspaciGo",
  icons: { icon: "/espacigo-mark.svg", apple: "/espacigo-mark.svg" },
};

export default function RootLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="es">
      <body>
        <a
          className="sr-only fixed left-3 top-3 z-[100] rounded-md bg-background px-4 py-3 text-foreground shadow-lg focus:not-sr-only focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          href="#main-content"
        >
          Omitir al contenido principal
        </a>
        {children}
      </body>
    </html>
  );
}
