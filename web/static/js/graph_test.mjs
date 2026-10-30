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
	fitCanvasView,
	extractEdgeEndpoints,
	isCanvasVisible,
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

test("fitCanvasView resets to default viewport when no nodes are present", () => {
	const canvas = { clientWidth: 800, clientHeight: 600 };
	const viewport = { x: 50, y: 50, zoom: 1.5 };
	const res = fitCanvasView(canvas, viewport, () => []);
	assert.deepEqual(res, { x: 0, y: 0, zoom: 1.0 });
	assert.deepEqual(viewport, { x: 0, y: 0, zoom: 1.0 });
});

test("fitCanvasView clamps zoom to 1.0 and centers a single node", () => {
	const canvas = { clientWidth: 800, clientHeight: 600 };
	const viewport = { x: 0, y: 0, zoom: 1.0 };
	const nodes = [{ x: 100, y: 100, width: 200, height: 100 }];
	const res = fitCanvasView(canvas, viewport, () => nodes);
	assert.equal(res.zoom, 1.0);
	// Center of node is (200, 150). Center of 800x600 viewport is (400, 300).
	// Viewport x = 400 - 200 * 1.0 = 200. Viewport y = 300 - 150 * 1.0 = 150.
	assert.equal(res.x, 200);
	assert.equal(res.y, 150);
});

test("fitCanvasView fits multiple nodes with 40px margin", () => {
	const canvas = { clientWidth: 800, clientHeight: 600 };
	const viewport = { x: 0, y: 0, zoom: 1.0 };
	// Content bounds: x: [0, 1000] (width 1000), y: [0, 500] (height 500)
	// Available: width: 800 - 80 = 720. height: 600 - 80 = 520.
	// Zoom: min(720 / 1000 = 0.72, 520 / 500 = 1.04) => 0.72.
	const nodes = [
		{ x: 0, y: 0, width: 200, height: 100 },
		{ x: 800, y: 400, width: 200, height: 100 },
	];
	const res = fitCanvasView(canvas, viewport, () => nodes);
	assert.ok(Math.abs(res.zoom - 0.72) < 1e-4);
	// Center is (500, 250).
	// Viewport x = 400 - 500 * 0.72 = 400 - 360 = 40 (matches 40px margin!).
	assert.ok(Math.abs(res.x - 40) < 1e-4);
});

test("fitCanvasView clamps zoom to minimum 0.25 when nodes are spread widely", () => {
	const canvas = { clientWidth: 800, clientHeight: 600 };
	const viewport = { x: 0, y: 0, zoom: 1.0 };
	const nodes = [
		{ x: 0, y: 0, width: 200, height: 100 },
		{ x: 10000, y: 10000, width: 200, height: 100 },
	];
	const res = fitCanvasView(canvas, viewport, () => nodes);
	assert.equal(res.zoom, 0.25);
});

test("fitCanvasView reads node position and size from style properties with fallback", () => {
	const canvas = { clientWidth: 1000, clientHeight: 800 };
	const viewport = { x: 0, y: 0, zoom: 1.0 };
	const domNodes = [
		{
			style: { left: "100px", top: "100px" },
			offsetWidth: 0,
			offsetHeight: 0,
		},
	];
	// Fallback width 264, height 140. Node box: [100, 364], [100, 240].
	// Center: (232, 170). Canvas: 1000x800 -> center (500, 400).
	// Zoom clamps to 1.0.
	// Viewport x = 500 - 232 * 1.0 = 268. Viewport y = 400 - 170 * 1.0 = 230.
	const res = fitCanvasView(canvas, viewport, () => domNodes);
	assert.equal(res.zoom, 1.0);
	assert.equal(res.x, 268);
	assert.equal(res.y, 230);
});

test("extractEdgeEndpoints handles data-predecessor/successor attributes and invalid inputs", () => {
	assert.equal(extractEdgeEndpoints(null), null);
	assert.equal(extractEdgeEndpoints({}), null);

	const edge = {
		getAttribute(name) {
			if (name === "data-predecessor") return "pred-1";
			if (name === "data-successor") return "succ-2";
			return null;
		},
		dataset: {
			predecessor: "pred-1",
			successor: "succ-2",
		},
	};
	assert.deepEqual(extractEdgeEndpoints(edge), { predecessor: "pred-1", successor: "succ-2" });
});

test("loose task client drop converts to canvas coordinates correctly", () => {
	const viewport = { x: 50, y: 30, zoom: 0.8 };
	const canvasRect = { left: 100, top: 50 };
	const clientDrop = { clientX: 250, clientY: 150 };

	const screenPoint = {
		x: clientDrop.clientX - canvasRect.left, // 150
		y: clientDrop.clientY - canvasRect.top,  // 100
	};
	const canvasCoord = screenToCanvas(screenPoint, viewport);
	// (150 - 50) / 0.8 = 100 / 0.8 = 125
	// (100 - 30) / 0.8 = 70 / 0.8 = 87.5
	assert.equal(canvasCoord.x, 125);
	assert.equal(canvasCoord.y, 87.5);
});

test("isCanvasVisible returns false for falsy or zero-dimension canvas", () => {
	assert.equal(isCanvasVisible(null), false);
	assert.equal(isCanvasVisible(undefined), false);
	assert.equal(isCanvasVisible({ offsetWidth: 0, offsetHeight: 0, clientWidth: 0, clientHeight: 0 }), false);
});

test("isCanvasVisible returns false when hidden in narrow screen loose tab", () => {
	const canvas = {
		offsetWidth: 400,
		offsetHeight: 600,
		closest(selector) {
			if (selector === ".board-panes") {
				return { dataset: { tab: "loose" } };
			}
			return null;
		},
	};

	// Save global window
	const originalWindow = globalThis.window;
	try {
		globalThis.window = {
			matchMedia: (q) => ({ matches: q.includes("max-width: 767px") }),
			getComputedStyle: () => ({ visibility: "visible", display: "block" }),
		};
		assert.equal(isCanvasVisible(canvas), false);

		// When on desktop (> 767px), matchMedia returns false -> canvas is visible
		globalThis.window.matchMedia = () => ({ matches: false });
		assert.equal(isCanvasVisible(canvas), true);

		// When CSS visibility is hidden, it is false regardless
		globalThis.window.getComputedStyle = () => ({ visibility: "hidden" });
		assert.equal(isCanvasVisible(canvas), false);
	} finally {
		if (originalWindow === undefined) {
			delete globalThis.window;
		} else {
			globalThis.window = originalWindow;
		}
	}
});


