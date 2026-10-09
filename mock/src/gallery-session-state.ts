export interface GalleryContext {
  token: string;
  accountID: string;
  generation: number;
  spaceID: string;
  revision: number;
}

export function galleryContextMatches(
  context: GalleryContext,
  current: {token:string;accountID:string;generation:number;spaceID:string;revision:number},
): boolean {
  return Boolean(context.token && context.accountID && context.spaceID &&
    context.token===current.token && context.accountID===current.accountID &&
    context.generation===current.generation && context.spaceID===current.spaceID &&
    context.revision===current.revision);
}
