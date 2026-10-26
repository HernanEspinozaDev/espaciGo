export function About() {
  return (
    <section id="about" className="bg-secondary/30">
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-24 lg:px-8">
        <div className="mx-auto max-w-2xl text-center">
          <h2 className="text-3xl font-medium tracking-tight text-foreground sm:text-4xl">
            Un espacio para cada proyecto
          </h2>
          <p className="mt-6 text-lg leading-relaxed text-muted-foreground">
            EspaciGo busca acercar a las personas a lugares que se ajusten a sus
            ideas: desde una sala para reunirse hasta un taller para crear.
          </p>
          <p className="mt-4 text-lg leading-relaxed text-muted-foreground">
            Este catálogo es una reconstrucción local con contenido sintético.
            No representa publicaciones, anfitriones ni disponibilidad reales.
          </p>
        </div>
      </div>
    </section>
  );
}
