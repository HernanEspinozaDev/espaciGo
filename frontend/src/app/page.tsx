export default function Home() {
  return (
    <main className="shell" aria-labelledby="welcome-title">
      <div className="brand" aria-label="EspaciGo">
        <svg
          aria-hidden="true"
          viewBox="154 130 512 512"
          className="brand-mark"
        >
          <path
            d="M592 204C548 166 489 150 433 156C315 164 218 255 217 363C214 418 233 460 272 491L402 612Q409 622 416 612C456 566 519 535 561 488C595 450 605 404 603 356Q603 335 582 335H511Q500 335 491 328L439 286Q426 275 414 284L329 352Q317 362 317 377V410Q325 431 340 444C310 420 298 392 298 369C298 293 358 231 433 231C466 230 496 243 532 265Q540 270 548 262L592 218Q599 211 592 204Z"
            fill="currentColor"
          />
          <path
            d="M340 444V401Q340 389 350 381L407 340Q417 332 427 340L491 389Q501 398 514 400V428C514 441 508 450 498 458C456 492 393 499 350 474Q344 470 340 466Z"
            fill="white"
          />
        </svg>
        <span>EspaciGo</span>
      </div>
      <p className="eyebrow">Una nueva experiencia está en construcción</p>
      <h1 id="welcome-title">Encuentra tu espacio.</h1>
      <p className="description">
        Pronto podrás descubrir EspaciGo desde aquí.
      </p>
    </main>
  );
}
