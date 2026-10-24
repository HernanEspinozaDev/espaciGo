import type { Metadata } from "next";
import type { ReactNode } from "react";
import "./globals.css";

export const metadata: Metadata = {
  title: "EspaciGo",
  description: "Descubre espacios para tus próximos proyectos.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="es">
      <body>
        <a className="skip-link" href="#main-content">
          Omitir al contenido principal
        </a>
        {children}
      </body>
    </html>
  );
}
