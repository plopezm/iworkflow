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
	if (!canvasEl) return;
	const edges = canvasEl.querySelector ? canvasEl.querySelector(".graph-edges") : null;
	const nodes = canvasEl.querySelector ? canvasEl.querySelector(".graph-nodes") : null;
	const transform = `translate(${viewport.x.toFixed(2)}px, ${viewport.y.toFixed(2)}px) scale(${viewport.zoom.toFixed(4)})`;
	if (edges) edges.style.transform = transform;
	if (nodes) nodes.style.transform = transform;
	const gridSize = GRID_SIZE * viewport.zoom;
	if (canvasEl.style) {
		canvasEl.style.backgroundSize = `${gridSize.toFixed(2)}px ${gridSize.toFixed(2)}px`;
		canvasEl.style.backgroundPosition = `${viewport.x.toFixed(2)}px ${viewport.y.toFixed(2)}px`;
	}
}

/**
 * Calculates bounds of all .task-node elements, computes center and fitted zoom
 * (clamped between 0.25 and 1.0, with margin 40), and updates viewport and canvas transform.
 */
export function fitCanvasView(canvas, viewport = null, getNodes = null) {
	const vp = viewport || (canvas?._getViewport ? canvas._getViewport() : { ...DEFAULT_VIEWPORT });
	const nodes = getNodes ? getNodes() : (canvas?.querySelectorAll ? canvas.querySelectorAll(".graph-node, .task-node") : []);

	if (!nodes || nodes.length === 0) {
		const nextViewport = { ...DEFAULT_VIEWPORT };
		vp.x = nextViewport.x;
		vp.y = nextViewport.y;
		vp.zoom = nextViewport.zoom;
		if (canvas) updateCanvasTransform(canvas, vp);
		return nextViewport;
	}

	let minX = Infinity;
	let minY = Infinity;
	let maxX = -Infinity;
	let maxY = -Infinity;

	for (const node of nodes) {
		let x = 0;
		let y = 0;
		let w = 264;
		let h = 140;

		if (node.x !== undefined && node.y !== undefined) {
			x = Number(node.x);
			y = Number(node.y);
			w = node.width !== undefined ? Number(node.width) : 264;
			h = node.height !== undefined ? Number(node.height) : 140;
		} else if (node.style) {
			x = parseFloat(node.style.left) || 0;
			y = parseFloat(node.style.top) || 0;
			w = parseFloat(node.style.width) || node.offsetWidth || 264;
			h = parseFloat(node.style.height) || node.offsetHeight || 140;
		}

		minX = Math.min(minX, x);
		minY = Math.min(minY, y);
		maxX = Math.max(maxX, x + w);
		maxY = Math.max(maxY, y + h);
	}

	const bboxWidth = Math.max(maxX - minX, 1);
	const bboxHeight = Math.max(maxY - minY, 1);
	const centerX = (minX + maxX) / 2;
	const centerY = (minY + maxY) / 2;

	let viewWidth = 800;
	let viewHeight = 600;
	if (canvas) {
		if (typeof canvas.getBoundingClientRect === "function") {
			const rect = canvas.getBoundingClientRect();
			if (rect.width > 0) viewWidth = rect.width;
			if (rect.height > 0) viewHeight = rect.height;
		} else {
			if (canvas.clientWidth) viewWidth = canvas.clientWidth;
			else if (canvas.width) viewWidth = canvas.width;
			if (canvas.clientHeight) viewHeight = canvas.clientHeight;
			else if (canvas.height) viewHeight = canvas.height;
		}
	}

	const margin = 40;
	const availableWidth = Math.max(viewWidth - 2 * margin, 1);
	const availableHeight = Math.max(viewHeight - 2 * margin, 1);

	const zoomX = availableWidth / bboxWidth;
	const zoomY = availableHeight / bboxHeight;
	const fittedZoom = Math.min(zoomX, zoomY);
	const zoom = Math.min(Math.max(fittedZoom, 0.25), 1.0);

	const nextViewport = {
		x: viewWidth / 2 - centerX * zoom,
		y: viewHeight / 2 - centerY * zoom,
		zoom,
	};

	vp.x = nextViewport.x;
	vp.y = nextViewport.y;
	vp.zoom = nextViewport.zoom;

	if (canvas) {
		updateCanvasTransform(canvas, vp);
	}

	return nextViewport;
}

/**
 * Extracts predecessor and successor IDs from an edge element's dataset or delete attributes.
 */
export function extractEdgeEndpoints(edgeEl) {
	if (!edgeEl) return null;
	const delBtn = edgeEl.querySelector ? edgeEl.querySelector(".graph-edge-delete") : null;
	const hxDelete = delBtn?.getAttribute ? delBtn.getAttribute("hx-delete") : (edgeEl.getAttribute ? edgeEl.getAttribute("hx-delete") : null);
	if (hxDelete) {
		try {
			const query = hxDelete.includes("?") ? hxDelete.split("?")[1] : hxDelete;
			const params = new URLSearchParams(query);
			const pred = params.get("predecessor");
			const succ = params.get("successor");
			if (pred && succ) {
				return { predecessor: pred, successor: succ };
			}
		} catch (_) {}
	}
	const edgeKey = edgeEl.dataset?.edgeKey || (edgeEl.getAttribute ? edgeEl.getAttribute("data-edge-key") : "") || "";
	if (edgeKey.includes("->")) {
		const [pred, succ] = edgeKey.split("->");
		if (pred && succ) {
			return { predecessor: pred, successor: succ };
		}
	}
	const pred = edgeEl.dataset?.predecessor || (edgeEl.getAttribute ? edgeEl.getAttribute("data-predecessor") : null);
	const succ = edgeEl.dataset?.successor || (edgeEl.getAttribute ? edgeEl.getAttribute("data-successor") : null);
	if (pred && succ) {
		return { predecessor: pred, successor: succ };
	}
	return null;
}

function triggerBoardRefresh() {
	if (typeof window !== "undefined" && window.htmx?.trigger) {
		window.htmx.trigger(document.body, "board:refresh");
	}
	if (typeof document !== "undefined") {
		document.body.dispatchEvent(new CustomEvent("board:refresh"));
	}
}

/**
 * Initializes interactive behaviors for all task graph canvases under root.
 */
export function initGraphCanvases(root = document) {
	const canvases = root.querySelectorAll("[data-graph-canvas]");
	for (const canvas of canvases) {
		setupCanvas(canvas);
	}
	setupLooseTasks(root);
}

const claimedCanvases = new WeakSet();

/**
 * Claims a canvas for its setup: true the first time, false afterwards. The claim is kept outside the DOM on
 * purpose. The board list is morphed on each refresh and a morph removes the attributes the server does not send,
 * so a flag stored on the element is lost, the canvas is set up again and each of its handlers runs twice.
 */
export function claimCanvas(canvas) {
	if (claimedCanvases.has(canvas)) return false;
	claimedCanvases.add(canvas);
	return true;
}

function setupCanvas(canvas) {
	if (!claimCanvas(canvas)) return;

	let viewport = { ...DEFAULT_VIEWPORT };
	let isPanning = false;
	let startPointer = { x: 0, y: 0 };
	let startViewport = { ...viewport };

	canvas._getViewport = () => viewport;
	canvas._setViewport = (vp) => {
		viewport = vp;
		updateCanvasTransform(canvas, viewport);
	};
	canvas._fitView = () => {
		viewport = fitCanvasView(canvas, viewport);
	};

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
		if (
			e.target.closest(".task-node") ||
			e.target.closest(".graph-node") ||
			e.target.closest(".graph-zoom-bar") ||
			e.target.closest(".task-node-port") ||
			e.target.closest(".graph-port") ||
			e.target.closest(".graph-edge-hit") ||
			e.target.closest(".graph-edge-line") ||
			e.target.closest(".graph-edge-delete") ||
			e.target.closest(".graph-tool")
		) {
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

	const zoomFitBtns = canvas.querySelectorAll("[data-zoom-fit]");
	for (const zoomFitBtn of zoomFitBtns) {
		zoomFitBtn.addEventListener("click", () => {
			viewport = fitCanvasView(canvas, viewport);
		});
	}

	// Node drag-to-reposition
	setupNodeDragging(canvas, () => viewport);

	// Port connection (dynamic graph edge creation)
	setupPortConnections(canvas, () => viewport);

	// Edge selection & deletion
	setupEdgeSelection(canvas);
}

function setupNodeDragging(canvas, getViewport) {
	let activeNode = null;
	let startPointer = { x: 0, y: 0 };
	let startNodePos = { x: 0, y: 0 };

	const updateConnectedEdges = (taskId, newX, newY) => {
		const edges = canvas.querySelectorAll("g[data-edge-key]");
		for (const g of edges) {
			const ep = extractEdgeEndpoints(g);
			if (!ep) continue;
			if (ep.predecessor === taskId) {
				const succNode = canvas.querySelector(`[data-task-id="${ep.successor}"]`);
				if (!succNode) continue;
				const succX = parseFloat(succNode.style.left) || 0;
				const succY = parseFloat(succNode.style.top) || 0;
				const from = { x: newX + 264, y: newY + 70 };
				const to = { x: succX, y: succY + 70 };
				const d = cubicCurve(from, to);
				const line = g.querySelector(".graph-edge-line");
				const hit = g.querySelector(".graph-edge-hit");
				if (line) line.setAttribute("d", d);
				if (hit) hit.setAttribute("d", d);
			} else if (ep.successor === taskId) {
				const predNode = canvas.querySelector(`[data-task-id="${ep.predecessor}"]`);
				if (!predNode) continue;
				const predX = parseFloat(predNode.style.left) || 0;
				const predY = parseFloat(predNode.style.top) || 0;
				const from = { x: predX + 264, y: predY + 70 };
				const to = { x: newX, y: newY + 70 };
				const d = cubicCurve(from, to);
				const line = g.querySelector(".graph-edge-line");
				const hit = g.querySelector(".graph-edge-hit");
				if (line) line.setAttribute("d", d);
				if (hit) hit.setAttribute("d", d);
			}
		}
	};

	canvas.addEventListener("pointerdown", (e) => {
		const handle = e.target.closest(".task-node-card, article.card");
		if (
			!handle ||
			e.target.closest(".task-node-port") ||
			e.target.closest(".graph-port") ||
			e.target.closest(".graph-tool") ||
			e.target.closest("button") ||
			e.target.closest("a")
		) {
			return;
		}
		const node = handle.closest(".graph-node, .task-node");
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
		const finalX = startNodePos.x + dx;
		const finalY = startNodePos.y + dy;
		activeNode.style.left = `${finalX.toFixed(0)}px`;
		activeNode.style.top = `${finalY.toFixed(0)}px`;
		const taskId = activeNode.dataset.taskId;
		if (taskId) {
			updateConnectedEdges(taskId, finalX, finalY);
		}
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

function setupPortConnections(canvas, getViewport) {
	let connecting = false;
	let sourceId = null;
	let fromPt = null;
	let previewPath = null;
	let activeTarget = null;
	let activePointerId = null;

	canvas.addEventListener("pointerdown", (e) => {
		const portOut = e.target.closest(".graph-port-out, .graph-port.graph-port-out");
		if (!portOut) return;

		const sourceNode = portOut.closest("[data-task-id]");
		if (!sourceNode) return;

		sourceId = sourceNode.dataset.taskId;
		if (!sourceId) return;

		connecting = true;
		activeTarget = portOut;
		activePointerId = e.pointerId;
		e.stopPropagation();
		e.preventDefault();

		try {
			portOut.setPointerCapture(e.pointerId);
		} catch (_) {}

		const nodeX = parseFloat(sourceNode.style.left) || 0;
		const nodeY = parseFloat(sourceNode.style.top) || 0;
		fromPt = { x: nodeX + 264, y: nodeY + 70 };

		const edgesLayer = canvas.querySelector(".graph-edges");
		if (edgesLayer) {
			previewPath = document.createElementNS("http://www.w3.org/2000/svg", "path");
			previewPath.setAttribute("class", "graph-edge-preview");
			previewPath.setAttribute("fill", "none");
			previewPath.setAttribute("stroke", "currentColor");
			previewPath.setAttribute("stroke-dasharray", "4 4");
			previewPath.setAttribute("stroke-width", "2");
			previewPath.style.pointerEvents = "none";
			edgesLayer.appendChild(previewPath);
		}
	});

	const findTargetNode = (canvasPt) => {
		const nodes = canvas.querySelectorAll(".graph-node, .task-node");
		for (const n of nodes) {
			const id = n.dataset.taskId;
			if (!id || id === sourceId) continue;
			const nx = parseFloat(n.style.left) || 0;
			const ny = parseFloat(n.style.top) || 0;
			if (
				canvasPt.x >= nx - 16 &&
				canvasPt.x <= nx + 264 + 16 &&
				canvasPt.y >= ny - 16 &&
				canvasPt.y <= ny + 140 + 16
			) {
				return n;
			}
		}
		return null;
	};

	const onPointerMove = (e) => {
		if (!connecting || !fromPt || !previewPath) return;
		const rect = canvas.getBoundingClientRect();
		const screenAnchor = { x: e.clientX - rect.left, y: e.clientY - rect.top };
		const viewport = getViewport();
		const toPt = screenToCanvas(screenAnchor, viewport);
		const targetNode = findTargetNode(toPt);
		const allNodes = canvas.querySelectorAll(".graph-node, .task-node");
		for (const n of allNodes) {
			if (n === targetNode) {
				n.classList.add("graph-node-target");
			} else {
				n.classList.remove("graph-node-target");
			}
		}
		const snapPt = targetNode ? { x: parseFloat(targetNode.style.left) || 0, y: (parseFloat(targetNode.style.top) || 0) + 70 } : toPt;
		previewPath.setAttribute("d", cubicCurve(fromPt, snapPt));
	};

	const onPointerUp = async (e) => {
		if (!connecting) return;
		connecting = false;

		const allNodes = canvas.querySelectorAll(".graph-node, .task-node");
		for (const n of allNodes) {
			n.classList.remove("graph-node-target");
		}

		if (previewPath) {
			previewPath.remove();
			previewPath = null;
		}

		if (activeTarget && activePointerId !== null) {
			try {
				activeTarget.releasePointerCapture(activePointerId);
			} catch (_) {}
		}
		activeTarget = null;
		activePointerId = null;

		const rect = canvas.getBoundingClientRect();
		const screenAnchor = { x: e.clientX - rect.left, y: e.clientY - rect.top };
		const viewport = getViewport();
		const toPt = screenToCanvas(screenAnchor, viewport);

		let targetNode = findTargetNode(toPt);
		if (!targetNode && typeof document !== "undefined" && typeof document.elementFromPoint === "function") {
			const el = document.elementFromPoint(e.clientX, e.clientY);
			targetNode = el?.closest(".graph-node, .task-node, [data-task-id]");
		}
		if (!targetNode) return;

		const targetId = targetNode.dataset.taskId;
		if (!targetId || targetId === sourceId) return;

		const body = new URLSearchParams();
		body.set("predecessor_id", sourceId);
		try {
			const res = await fetch(`/tasks/${encodeURIComponent(targetId)}/dependencies`, {
				method: "POST",
				headers: { "Content-Type": "application/x-www-form-urlencoded" },
				body: body.toString(),
			});
			if (res.ok) {
				triggerBoardRefresh();
			}
		} catch (err) {
			console.error("Failed to add dependency:", err);
		}
	};

	canvas.addEventListener("pointermove", onPointerMove);
	canvas.addEventListener("pointerup", onPointerUp);
	canvas.addEventListener("pointercancel", onPointerUp);
	if (typeof window !== "undefined") {
		window.addEventListener("pointermove", onPointerMove);
		window.addEventListener("pointerup", onPointerUp);
	}
}

function setupEdgeSelection(canvas) {
	const clearSelection = () => {
		const selected = canvas.querySelectorAll(".graph-edge-selected, g[data-edge-key].selected");
		for (const el of selected) {
			el.classList.remove("graph-edge-selected", "selected");
			const dynamicDel = el.querySelector(".graph-edge-delete-dynamic");
			if (dynamicDel) dynamicDel.remove();
		}
	};

	const deleteEdge = async (edgeEl) => {
		const endpoints = extractEdgeEndpoints(edgeEl);
		if (!endpoints) return;
		try {
			const res = await fetch(`/tasks/dependencies?predecessor=${encodeURIComponent(endpoints.predecessor)}&successor=${encodeURIComponent(endpoints.successor)}`, {
				method: "DELETE",
			});
			if (res.ok) {
				triggerBoardRefresh();
			}
		} catch (err) {
			console.error("Failed to delete dependency edge:", err);
		}
	};

	const handleEdgeClick = (e) => {
		const delBtn = e.target.closest ? e.target.closest(".graph-edge-delete") : null;
		if (delBtn) {
			e.stopPropagation();
			e.preventDefault();
			const edgeEl = delBtn.closest("g[data-edge-key]") || delBtn.closest(".graph-edge");
			if (edgeEl) deleteEdge(edgeEl);
			return;
		}

		const hit = e.target.closest ? e.target.closest(".graph-edge-hit, .graph-edge-line") : null;
		if (hit) {
			e.stopPropagation();
			const edgeEl = hit.closest("g[data-edge-key]") || hit.closest(".graph-edge");
			if (!edgeEl) return;

			const isAlreadySelected = edgeEl.classList.contains("graph-edge-selected") || edgeEl.classList.contains("selected");
			clearSelection();

			if (!isAlreadySelected) {
				edgeEl.classList.add("graph-edge-selected");
				if (!edgeEl.querySelector(".graph-edge-delete")) {
					let midX = 0;
					let midY = 0;
					const pathEl = edgeEl.querySelector(".graph-edge-line") || hit;
					if (pathEl && typeof pathEl.getTotalLength === "function") {
						try {
							const len = pathEl.getTotalLength();
							if (len > 0) {
								const pt = pathEl.getPointAtLength(len / 2);
								midX = pt.x;
								midY = pt.y;
							}
						} catch (_) {}
					}
					if (midX === 0 && midY === 0) {
						const d = pathEl?.getAttribute("d");
						if (d) {
							const nums = d.match(/[-+]?[0-9]*\.?[0-9]+/g)?.map(Number);
							if (nums && nums.length >= 8) {
								midX = (nums[0] + 3 * nums[2] + 3 * nums[4] + nums[6]) / 8;
								midY = (nums[1] + 3 * nums[3] + 3 * nums[5] + nums[7]) / 8;
							}
						}
					}
					const delG = document.createElementNS("http://www.w3.org/2000/svg", "g");
					delG.setAttribute("class", "graph-edge-delete graph-edge-delete-dynamic");
					delG.setAttribute("transform", `translate(${midX.toFixed(2)}, ${midY.toFixed(2)})`);
					delG.setAttribute("role", "button");
					delG.setAttribute("aria-label", "Eliminar dependencia");
					const circle = document.createElementNS("http://www.w3.org/2000/svg", "circle");
					circle.setAttribute("r", "10");
					delG.appendChild(circle);
					const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
					path.setAttribute("d", "M -4 -4 L 4 4 M 4 -4 L -4 4");
					delG.appendChild(path);
					edgeEl.appendChild(delG);
				}
			}
			return;
		}

		// Clicked canvas background or node: clear selection
		if (!e.target.closest || !e.target.closest(".graph-edge-delete")) {
			clearSelection();
		}
	};

	canvas.addEventListener("click", handleEdgeClick);
	const edgesSvg = canvas.querySelector(".graph-edges");
	if (edgesSvg) {
		edgesSvg.addEventListener("click", handleEdgeClick);
		edgesSvg.addEventListener("pointerdown", (e) => {
			if (e.target.closest && e.target.closest(".graph-edge-hit, .graph-edge-line, .graph-edge-delete")) {
				e.stopPropagation();
			}
		});
	}

	if (typeof document !== "undefined") {
		document.addEventListener("keydown", (e) => {
			if (e.key !== "Delete" && e.key !== "Backspace") return;
			const tag = document.activeElement?.tagName?.toLowerCase();
			if (tag === "input" || tag === "textarea" || document.activeElement?.isContentEditable) {
				return;
			}
			const selectedEdge = canvas.querySelector(".graph-edge-selected, g[data-edge-key].selected");
			if (selectedEdge) {
				e.preventDefault();
				deleteEdge(selectedEdge);
			}
		});
	}
}

/**
 * Checks if a canvas element is currently visible and interactive for drops.
 */
export function isCanvasVisible(canvas) {
	if (!canvas) return false;
	if (canvas.offsetWidth === 0 && canvas.offsetHeight === 0 && !canvas.clientWidth && !canvas.clientHeight) {
		return false;
	}
	if (typeof window !== "undefined" && window.getComputedStyle) {
		const style = window.getComputedStyle(canvas);
		if (style.visibility === "hidden" || style.display === "none") return false;
	}
	const panes = canvas.closest ? canvas.closest(".board-panes") : null;
	if (panes && panes.dataset && panes.dataset.tab === "loose") {
		const isNarrow = typeof window !== "undefined" && window.matchMedia
			? window.matchMedia("(max-width: 767px)").matches
			: false;
		if (isNarrow) return false;
	}
	return true;
}

export function setupLooseTasks(root = document) {
	if (!root || root._looseTasksInitialized) return;
	root._looseTasksInitialized = true;

	let draggingCard = null;
	let taskId = null;
	let startPointer = { x: 0, y: 0 };
	let isDragging = false;
	let dragClone = null;

	root.addEventListener("pointerdown", (e) => {
		// In mobile/narrow views (<= 767px), board panes are mutually exclusive tabs ("graph" vs "loose").
		// Inter-panel drag is disabled; adding tasks to the graph is done via the explicit button.
		const isMobile = typeof window !== "undefined" && window.matchMedia
			? window.matchMedia("(max-width: 767px)").matches
			: false;
		if (isMobile) return;

		const card = e.target.closest(".loose-list [data-loose-task], [data-loose-task]");
		if (!card) return;
		if (e.target.closest("button, a, input, textarea, select")) return;

		taskId = card.dataset.taskId;
		if (!taskId) return;

		draggingCard = card;
		startPointer = { x: e.clientX, y: e.clientY };
		isDragging = false;
	});

	const onPointerMove = (e) => {
		if (!draggingCard) return;
		const dx = e.clientX - startPointer.x;
		const dy = e.clientY - startPointer.y;
		if (!isDragging && (Math.abs(dx) > 4 || Math.abs(dy) > 4)) {
			isDragging = true;
			if (!dragClone && typeof document !== "undefined") {
				dragClone = draggingCard.cloneNode(true);
				dragClone.style.position = "fixed";
				dragClone.style.pointerEvents = "none";
				dragClone.style.zIndex = "99999";
				dragClone.style.opacity = "0.85";
				dragClone.style.width = `${draggingCard.offsetWidth || 240}px`;
				dragClone.style.boxShadow = "0 8px 24px rgba(0,0,0,0.3)";
				document.body.appendChild(dragClone);
				draggingCard.style.opacity = "0.4";
			}
		}

		if (isDragging && dragClone) {
			dragClone.style.left = `${e.clientX - 20}px`;
			dragClone.style.top = `${e.clientY - 20}px`;
		}
	};

	const onPointerUp = async (e) => {
		if (!draggingCard) return;
		const card = draggingCard;
		const id = taskId;
		const dragged = isDragging;

		draggingCard = null;
		taskId = null;
		isDragging = false;

		card.style.opacity = "";
		if (dragClone) {
			dragClone.remove();
			dragClone = null;
		}

		if (dragged) {
			// Suppress synthetic clicks that mobile/touch or desktop browsers fire upon releasing a drag
			const suppressClick = (evt) => {
				evt.preventDefault();
				evt.stopPropagation();
				root.removeEventListener("click", suppressClick, true);
			};
			root.addEventListener("click", suppressClick, true);
			setTimeout(() => {
				root.removeEventListener("click", suppressClick, true);
			}, 100);
		}

		if (!dragged) return;

		const canvases = typeof document !== "undefined" ? document.querySelectorAll("[data-graph-canvas]") : [];
		let targetCanvas = null;
		let canvasRect = null;
		for (const c of canvases) {
			if (!isCanvasVisible(c)) continue;
			const r = c.getBoundingClientRect();
			if (
				e.clientX >= r.left &&
				e.clientX <= r.right &&
				e.clientY >= r.top &&
				e.clientY <= r.bottom
			) {
				targetCanvas = c;
				canvasRect = r;
				break;
			}
		}

		if (!targetCanvas || !canvasRect) return;

		const viewport = targetCanvas._getViewport ? targetCanvas._getViewport() : DEFAULT_VIEWPORT;
		const canvasCoords = screenToCanvas(
			{ x: e.clientX - canvasRect.left, y: e.clientY - canvasRect.top },
			viewport
		);

		const body = new URLSearchParams();
		body.set("x", Math.round(canvasCoords.x).toString());
		body.set("y", Math.round(canvasCoords.y).toString());

		try {
			const res = await fetch(`/tasks/${encodeURIComponent(id)}/position`, {
				method: "POST",
				headers: { "Content-Type": "application/x-www-form-urlencoded" },
				body: body.toString(),
			});
			if (res.ok) {
				triggerBoardRefresh();
			}
		} catch (err) {
			console.error("Failed to position loose task on canvas:", err);
		}
	};

	root.addEventListener("pointermove", onPointerMove);
	root.addEventListener("pointerup", onPointerUp);
	root.addEventListener("pointercancel", onPointerUp);
}

// Global click handler for any [data-zoom-fit] button in document/toolbar
if (typeof document !== "undefined") {
	document.addEventListener("click", (e) => {
		const fitBtn = e.target.closest("[data-zoom-fit]");
		if (!fitBtn) return;
		// If the button is already inside a canvas, setupCanvas's listener or this listener will handle it
		const canvas = fitBtn.closest("[data-graph-canvas]") || document.querySelector("[data-graph-canvas]");
		if (canvas && typeof canvas._fitView === "function") {
			canvas._fitView();
		}
	});

	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", () => initGraphCanvases());
	} else {
		initGraphCanvases();
	}
	document.addEventListener("htmx:afterSettle", () => initGraphCanvases());
}
