export function galleryContextMatches(context, current) {
    return Boolean(context.token && context.accountID && context.spaceID &&
        context.token === current.token && context.accountID === current.accountID &&
        context.generation === current.generation && context.spaceID === current.spaceID &&
        context.revision === current.revision);
}
