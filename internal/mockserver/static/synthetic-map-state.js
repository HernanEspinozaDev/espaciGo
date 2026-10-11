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
        this.clampPan();
    }
    panBy(dx, dy) {
        if (!Number.isFinite(dx) || !Number.isFinite(dy))
            return;
        this.panX += dx;
        this.panY += dy;
        this.clampPan();
    }
    clampPan() {
        const extent = Math.max(0, (this.zoom - 1) * 50);
        const clamp = (value) => extent === 0 ? 0 : Math.min(extent, Math.max(-extent, value));
        this.panX = clamp(this.panX);
        this.panY = clamp(this.panY);
    }
    project(position) {
        return { x: 50 + (position.x - 50) * this.zoom + this.panX, y: 50 + (position.y - 50) * this.zoom + this.panY };
    }
    isVisible(position) {
        const projected = this.project(position);
        return projected.x >= 0 && projected.x <= 100 && projected.y >= 0 && projected.y <= 100;
    }
}
