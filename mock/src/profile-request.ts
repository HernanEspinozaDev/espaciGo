/** Guards asynchronous profile loads against category changes and older replies. */
export class ProfileRequestGate {
  private sequence = 0;

  begin(): number {
    this.sequence += 1;
    return this.sequence;
  }

  accepts(request: number, requestedCategory: string, selectedCategory: string): boolean {
    return request === this.sequence && requestedCategory !== "" && requestedCategory === selectedCategory;
  }
}

export function profileMatchesSelection(profileCategory: string | undefined, selectedCategory: string): boolean {
  return selectedCategory !== "" && profileCategory === selectedCategory;
}
