// Client-side interactions for the task dependency graph canvas:
// pan, zoom, node repositioning, edge creation via drag & drop, and loose tasks placement.

export const MIN_ZOOM = 0.25;
export const MAX_ZOOM = 2.0;
export const ZOOM_STEP = 1.2;
export const GRID_SIZE = 24.0;
export const MIN_CONTROL_OFFSET = 48.0;

export const DEFAULT_VIEWPORT = {
	x: 0,
	y: 0,
	zoom: 1.0,
};

export function clampZoom(zoom) {
	return Math.min(Math.max(zoom, MIN_ZOOM), MAX_ZOOM);
}

export function screenToCanvas(screen, viewport) {
	return {
		x: (screen.x - viewport.x) / viewport.zoom,
		y: (screen.y - viewport.y) / viewport.zoom,
	};
}

export function canvasToScreen(canvas, viewport) {
	return {
		x: canvas.x * viewport.zoom + viewport.x,
		y: canvas.y * viewport.zoom + viewport.y,
	};
}

export function panViewport(viewport, dx, dy) {
	return {
		x: viewport.x + dx,
		y: viewport.y + dy,
		zoom: viewport.zoom,
	};
}

export function zoomViewportAt(viewport, anchor, factor) {
	const zoom = clampZoom(viewport.zoom * factor);
	const canvas = screenToCanvas(anchor, viewport);
	return {
		x: anchor.x - canvas.x * zoom,
		y: anchor.y - canvas.y * zoom,
		zoom,
	};
}

function controlOffset(from, to) {
	const dx = to.x - from.x;
	if (dx >= 0.0) {
		return Math.max(dx * 0.5, MIN_CONTROL_OFFSET);
	}
	return MIN_CONTROL_OFFSET + Math.abs(dx) * 0.5;
}

export function cubicCurve(from, to) {
	const offset = controlOffset(from, to);
	const c1 = { x: from.x + offset, y: from.y };
	const c2 = { x: to.x - offset, y: to.y };
	return `M ${from.x.toFixed(2)} ${from.y.toFixed(2)} C ${c1.x.toFixed(2)} ${c1.y.toFixed(2)}, ${c2.x.toFixed(2)} ${c2.y.toFixed(2)}, ${to.x.toFixed(2)} ${to.y.toFixed(2)}`;
}

export function updateCanvasTransform(canvasEl, viewport) {
	const edges = canvasEl.querySelector(".graph-edges");
	const nodes = canvasEl.querySelector(".graph-nodes");
	const transform = `translate(${viewport.x.toFixed(2)}px, ${viewport.y.toFixed(2)}px) scale(${viewport.zoom.toFixed(4)})`;
	if (edges) edges.style.transform = transform;
	if (nodes) nodes.style.transform = transform;
	const gridSize = GRID_SIZE * viewport.zoom;
	canvasEl.style.backgroundSize = `${gridSize.toFixed(2)}px ${gridSize.toFixed(2)}px`;
	canvasEl.style.backgroundPosition = `${viewport.x.toFixed(2)}px ${viewport.y.toFixed(2)}px`;
}

/**
 * Initializes interactive behaviors for all task graph canvases under root.
 */
export function initGraphCanvases(root = document) {
	const canvases = root.querySelectorAll("[data-graph-canvas]");
	for (const canvas of canvases) {
		setupCanvas(canvas);
	}
}

function setupCanvas(canvas) {
	if (canvas.dataset.graphInitialized) return;
	canvas.dataset.graphInitialized = "true";

	let viewport = { ...DEFAULT_VIEWPORT };
	let isPanning = false;
	let startPointer = { x: 0, y: 0 };
	let startViewport = { ...viewport };

	// Zoom with wheel
	canvas.addEventListener(
		"wheel",
		(e) => {
			e.preventDefault();
			const rect = canvas.getBoundingClientRect();
			const anchor = { x: e.clientX - rect.left, y: e.clientY - rect.top };
			const delta = Math.max(-400, Math.min(400, e.deltaY));
			const factor = Math.exp(-delta * 0.0015);
			viewport = zoomViewportAt(viewport, anchor, factor);
			updateCanvasTransform(canvas, viewport);
		},
		{ passive: false }
	);

	// Pan background with pointerdown
	canvas.addEventListener("pointerdown", (e) => {
		if (e.target.closest(".task-node") || e.target.closest(".graph-zoom-bar") || e.target.closest(".task-node-port")) {
			return;
		}
		isPanning = true;
		startPointer = { x: e.clientX, y: e.clientY };
		startViewport = { ...viewport };
		canvas.setPointerCapture(e.pointerId);
	});

	canvas.addEventListener("pointermove", (e) => {
		if (!isPanning) return;
		const dx = e.clientX - startPointer.x;
		const dy = e.clientY - startPointer.y;
		viewport = panViewport(startViewport, dx, dy);
		updateCanvasTransform(canvas, viewport);
	});

	const endPan = (e) => {
		if (!isPanning) return;
		isPanning = false;
		try {
			canvas.releasePointerCapture(e.pointerId);
		} catch (_) {}
	};

	canvas.addEventListener("pointerup", endPan);
	canvas.addEventListener("pointercancel", endPan);

	// Zoom controls
	const zoomInBtn = canvas.querySelector("[data-zoom-in]");
	if (zoomInBtn) {
		zoomInBtn.addEventListener("click", () => {
			const rect = canvas.getBoundingClientRect();
			const center = { x: rect.width / 2, y: rect.height / 2 };
			viewport = zoomViewportAt(viewport, center, ZOOM_STEP);
			updateCanvasTransform(canvas, viewport);
		});
	}

	const zoomOutBtn = canvas.querySelector("[data-zoom-out]");
	if (zoomOutBtn) {
		zoomOutBtn.addEventListener("click", () => {
			const rect = canvas.getBoundingClientRect();
			const center = { x: rect.width / 2, y: rect.height / 2 };
			viewport = zoomViewportAt(viewport, center, 1 / ZOOM_STEP);
			updateCanvasTransform(canvas, viewport);
		});
	}

	const zoomFitBtn = canvas.querySelector("[data-zoom-fit]");
	if (zoomFitBtn) {
		zoomFitBtn.addEventListener("click", () => {
			viewport = { ...DEFAULT_VIEWPORT };
			updateCanvasTransform(canvas, viewport);
		});
	}

	// Node drag-to-reposition
	setupNodeDragging(canvas, () => viewport);
}

function setupNodeDragging(canvas, getViewport) {
	let activeNode = null;
	let startPointer = { x: 0, y: 0 };
	let startNodePos = { x: 0, y: 0 };

	canvas.addEventListener("pointerdown", (e) => {
		const handle = e.target.closest(".task-node-card");
		if (!handle || e.target.closest(".task-node-port") || e.target.closest("button") || e.target.closest("a")) {
			return;
		}
		const node = handle.closest(".task-node");
		if (!node) return;

		activeNode = node;
		startPointer = { x: e.clientX, y: e.clientY };
		startNodePos = {
			x: parseFloat(node.style.left) || 0,
			y: parseFloat(node.style.top) || 0,
		};
		node.setPointerCapture(e.pointerId);
		e.stopPropagation();
	});

	canvas.addEventListener("pointermove", (e) => {
		if (!activeNode) return;
		const viewport = getViewport();
		const dx = (e.clientX - startPointer.x) / viewport.zoom;
		const dy = (e.clientY - startPointer.y) / viewport.zoom;
		activeNode.style.left = `${(startNodePos.x + dx).toFixed(0)}px`;
		activeNode.style.top = `${(startNodePos.y + dy).toFixed(0)}px`;
	});

	const endNodeDrag = async (e) => {
		if (!activeNode) return;
		const node = activeNode;
		activeNode = null;
		try {
			node.releasePointerCapture(e.pointerId);
		} catch (_) {}

		const taskId = node.dataset.taskId;
		const finalX = parseFloat(node.style.left) || 0;
		const finalY = parseFloat(node.style.top) || 0;

		if (taskId) {
			const body = new URLSearchParams();
			body.set("x", finalX.toString());
			body.set("y", finalY.toString());
			try {
				await fetch(`/tasks/${taskId}/position`, {
					method: "POST",
					headers: { "Content-Type": "application/x-www-form-urlencoded" },
					body: body.toString(),
				});
			} catch (err) {
				console.error("Failed to persist node position:", err);
			}
		}
	};

	canvas.addEventListener("pointerup", endNodeDrag);
	canvas.addEventListener("pointercancel", endNodeDrag);
}

// Auto-initialize when document loads
if (typeof document !== "undefined") {
	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", () => initGraphCanvases());
	} else {
		initGraphCanvases();
	}
	document.addEventListener("htmx:afterSettle", () => initGraphCanvases());
}
