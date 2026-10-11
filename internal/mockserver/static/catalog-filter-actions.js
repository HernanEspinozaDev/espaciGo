export function clearCatalogFilters(actions) {
    actions.invalidatePending();
    actions.resetControls();
    actions.clearProfile();
    actions.clearCursors();
    actions.clearSelection();
    actions.resetMap();
}
