/** Guards asynchronous profile loads against category changes and older replies. */
export class ProfileRequestGate {
    sequence = 0;
    begin() {
        this.sequence += 1;
        return this.sequence;
    }
    accepts(request, requestedCategory, selectedCategory) {
        return request === this.sequence && requestedCategory !== "" && requestedCategory === selectedCategory;
    }
}
export function profileMatchesSelection(profileCategory, selectedCategory) {
    return selectedCategory !== "" && profileCategory === selectedCategory;
}
