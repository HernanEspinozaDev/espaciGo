import Link from "next/link";

export function Footer() {
  const year = new Date().getFullYear();

  return (
    <footer className="border-t border-border bg-background">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col gap-8 py-12 sm:flex-row sm:gap-16 sm:py-16">
          <div className="sm:max-w-xs">
            <Link href="/" className="text-xl font-bold text-foreground">
              EspaciGo
            </Link>
            <p className="mt-4 text-sm leading-relaxed text-muted-foreground">
              Arriendos residenciales y espacios flexibles para vivir, trabajar,
              guardar, reunirse o celebrar.
            </p>
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">Explora</h2>
            <ul className="mt-4 space-y-3">
              <li>
                <Link
                  href="/explorar"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Catálogo público
                </Link>
              </li>
              <li>
                <Link
                  href="/categorias"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Categorías
                </Link>
              </li>
              <li>
                <Link
                  href="/arriendos"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Arriendos residenciales
                </Link>
              </li>
            </ul>
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">
              Conoce EspaciGo
            </h2>
            <ul className="mt-4 space-y-3">
              <li>
                <Link
                  href="/como-funciona"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Cómo funciona
                </Link>
              </li>
              <li>
                <Link
                  href="/anfitriones"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Para anfitriones
                </Link>
              </li>
              <li>
                <Link
                  href="/login"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Ingresar a EspaciGo
                </Link>
              </li>
            </ul>
          </div>
          <div>
            <h2 className="text-sm font-semibold text-foreground">
              Información legal
            </h2>
            <ul className="mt-4 space-y-3">
              <li>
                <Link
                  href="/terminos-y-condiciones"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Términos y condiciones
                </Link>
              </li>
              <li>
                <Link
                  href="/politica-de-privacidad"
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Política de privacidad
                </Link>
              </li>
            </ul>
          </div>
        </div>
        <div className="flex flex-col gap-4 border-t border-border py-6 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-muted-foreground">© {year} EspaciGo</p>
          <p className="max-w-xl text-xs leading-relaxed text-muted-foreground">
            La tarifa y modalidad dependen de cada publicación. Una estimación
            no confirma disponibilidad ni constituye cotización o reserva.
          </p>
        </div>
      </div>
    </footer>
  );
}
