import test from "node:test";
import assert from "node:assert/strict";

import {
	screenToCanvas,
	canvasToScreen,
	panViewport,
	zoomViewportAt,
	cubicCurve,
	clampZoom,
	DEFAULT_VIEWPORT,
} from "./graph.js";

test("screenToCanvas and canvasToScreen are inverse", () => {
	const viewport = { x: 120, y: -40, zoom: 0.5 };
	const canvas = { x: 300, y: 80 };
	const screen = canvasToScreen(canvas, viewport);
	assert.deepEqual(screen, { x: 270, y: 0 });
	const back = screenToCanvas(screen, viewport);
	assert.ok(Math.abs(back.x - canvas.x) < 1e-9);
	assert.ok(Math.abs(back.y - canvas.y) < 1e-9);
});

test("panViewport shifts origin without altering zoom", () => {
	const viewport = panViewport(DEFAULT_VIEWPORT, 20, -15);
	assert.equal(viewport.x, 20);
	assert.equal(viewport.y, -15);
	assert.equal(viewport.zoom, 1.0);
});

test("zoomViewportAt keeps anchor point stationary", () => {
	const viewport = { x: 30, y: 10, zoom: 1.0 };
	const anchor = { x: 200, y: 150 };
	const before = screenToCanvas(anchor, viewport);
	const zoomed = zoomViewportAt(viewport, anchor, 1.5);
	const after = screenToCanvas(anchor, zoomed);
	assert.equal(zoomed.zoom, 1.5);
	assert.ok(Math.abs(before.x - after.x) < 1e-9);
	assert.ok(Math.abs(before.y - after.y) < 1e-9);
});

test("clampZoom restricts factor to [0.25, 2.0]", () => {
	assert.equal(clampZoom(10), 2.0);
	assert.equal(clampZoom(0.01), 0.25);
	assert.equal(clampZoom(1.2), 1.2);
});

test("cubicCurve formats SVG path string", () => {
	const path = cubicCurve({ x: 0, y: 0 }, { x: 400, y: 100 });
	assert.equal(path, "M 0.00 0.00 C 200.00 0.00, 200.00 100.00, 400.00 100.00");
});
