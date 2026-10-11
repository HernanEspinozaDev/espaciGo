export class SyntheticMapViewport {
    zoom = 1;
    panX = 0;
    panY = 0;
    reset() {
        this.zoom = 1;
        this.panX = 0;
        this.panY = 0;
    }
    zoomBy(factor) {
        if (!Number.isFinite(factor) || factor <= 0)
            return;
        this.zoom = Math.min(4, Math.max(1, this.zoom * factor));
    }
    panBy(dx, dy) {
        if (!Number.isFinite(dx) || !Number.isFinite(dy))
            return;
        const extent = Math.max(0, (this.zoom - 1) * 50);
        this.panX = Math.min(extent, Math.max(-extent, this.panX + dx));
        this.panY = Math.min(extent, Math.max(-extent, this.panY + dy));
    }
    project(position) {
        return { x: 50 + (position.x - 50) * this.zoom + this.panX, y: 50 + (position.y - 50) * this.zoom + this.panY };
    }
    isVisible(position) {
        const projected = this.project(position);
        return projected.x >= 0 && projected.x <= 100 && projected.y >= 0 && projected.y <= 100;
    }
}
