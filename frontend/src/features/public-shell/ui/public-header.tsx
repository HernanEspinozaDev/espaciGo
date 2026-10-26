"use client";

import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import { Menu } from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

const navLinks = [
  { href: "/", label: "Inicio" },
  { href: "/explorar", label: "Explorar" },
  { href: "/categorias", label: "Categorías" },
  {
    href: "/arriendos",
    label: "Arriendos residenciales",
  },
  {
    href: "/como-funciona",
    label: "Cómo funciona",
  },
];

export function PublicHeader() {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <header className="sticky top-0 z-50 border-b border-border bg-background/90 backdrop-blur-md">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="relative flex h-16 items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            {/* Mobile menu trigger */}
            <Sheet open={isOpen} onOpenChange={setIsOpen}>
              <SheetTrigger asChild>
                <button
                  type="button"
                  className="-order-1 rounded-full p-2 transition-colors hover:bg-secondary lg:hidden"
                  aria-label="Abrir menú"
                >
                  <Menu className="h-6 w-6" />
                </button>
              </SheetTrigger>
              <SheetContent side="left" className="w-full max-w-xs p-6">
                <SheetHeader className="text-left">
                  <SheetTitle className="text-base font-semibold">
                    Navegación
                  </SheetTitle>
                </SheetHeader>
                <nav
                  className="mt-6 flex flex-col gap-4"
                  aria-label="Navegación móvil"
                >
                  {navLinks.map((link) => (
                    <Link
                      key={link.href}
                      href={link.href}
                      className="text-base font-medium text-muted-foreground hover:text-foreground transition-colors"
                      onClick={() => setIsOpen(false)}
                    >
                      {link.label}
                    </Link>
                  ))}
                  <div className="my-2 border-t border-border pt-4">
                    <Link
                      href="/acceso"
                      className="text-base font-medium text-primary hover:underline"
                      onClick={() => setIsOpen(false)}
                    >
                      Ingresar a EspaciGo
                    </Link>
                  </div>
                </nav>
              </SheetContent>
            </Sheet>

            {/* Desktop Navigation (Centered) */}
            <nav className="hidden lg:absolute lg:left-1/2 lg:top-1/2 lg:flex lg:-translate-x-1/2 lg:-translate-y-1/2 items-center gap-6">
              {navLinks.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  className="text-sm font-medium text-muted-foreground hover:text-foreground transition-colors whitespace-nowrap"
                >
                  {link.label}
                </Link>
              ))}
            </nav>

            {/* Logo (Left) */}
            <Link
              href="/"
              className="inline-flex shrink-0 items-center gap-2 text-xl font-semibold tracking-tight"
              aria-label="EspaciGo, inicio"
            >
              <Image
                src="/espacigo-mark.svg"
                alt=""
                width={30}
                height={30}
                priority
              />
              <span>EspaciGo</span>
            </Link>
          </div>

          {/* Right Action */}
          <div className="flex items-center gap-1 sm:gap-2">
            <Link
              href="/acceso"
              className="inline-flex min-h-11 items-center rounded-full border border-border px-4 text-sm font-medium transition-colors hover:bg-secondary"
            >
              Ingresar a EspaciGo
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}
