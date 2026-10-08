export function publicationAction(state, roles) {
    if (!roles.includes("arrendador"))
        return null;
    if (state === "borrador" || state === "oculta")
        return { label: "Publicar", nextState: "activa" };
    if (state === "activa")
        return { label: "Ocultar", nextState: "oculta" };
    return null;
}
