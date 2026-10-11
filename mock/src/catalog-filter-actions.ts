export type ClearCatalogActions = {
  invalidatePending: () => void;
  resetControls: () => void;
  clearProfile: () => void;
  clearCursors: () => void;
  clearSelection: () => void;
  resetMap: () => void;
};

export function clearCatalogFilters(actions: ClearCatalogActions): void {
  actions.invalidatePending();
  actions.resetControls();
  actions.clearProfile();
  actions.clearCursors();
  actions.clearSelection();
  actions.resetMap();
}
