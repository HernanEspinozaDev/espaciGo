import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "EspaciGo",
  description: "Encuentra tu espacio con EspaciGo.",
  icons: { icon: "/espacigo-mark.svg" },
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="es">
      <body>{children}</body>
    </html>
  );
}
