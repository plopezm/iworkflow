package state

import (
	"fmt"
	"math"
)

// Geometría del lienzo de dependencias: dimensiones de los nodos, transformaciones de vista,
// puntos de anclaje y trazado de las aristas. Sin Leptos/htmx ni DOM, para poder probarla.

const (
	// NodeWidth es el ancho de un nodo en coordenadas de lienzo.
	NodeWidth = 264.0
	// NodeHeight es el alto de un nodo en coordenadas de lienzo.
	NodeHeight = 140.0
	// MinZoom es el zoom mínimo permitido.
	MinZoom = 0.25
	// MaxZoom es el zoom máximo permitido.
	MaxZoom = 2.0
	// FitPadding es el margen (en píxeles de pantalla) que deja «ajustar vista» alrededor del grafo.
	FitPadding = 56.0
	// ZoomStep es el paso de los botones de zoom.
	ZoomStep = 1.2
	// DropMargin es el margen extra (coordenadas de lienzo) alrededor de un nodo al soltar una arista sobre él,
	// para que también valga soltar sobre su puerto de entrada.
	DropMargin = 14.0
	// WheelSensitivity es la sensibilidad de la rueda del ratón.
	WheelSensitivity = 0.0015
	// WheelMaxDelta es el desplazamiento máximo de rueda que se tiene en cuenta en un solo evento.
	WheelMaxDelta = 400.0
	// MinControlOffset es la separación mínima entre un extremo de arista y sus puntos de control.
	MinControlOffset = 48.0
)

// Point es un punto 2D en punto flotante.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NewPoint crea un nuevo punto con las coordenadas dadas.
func NewPoint(x, y float64) Point {
	return Point{X: x, Y: y}
}

// Rect es un rectángulo alineado con los ejes.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// RectOfNode calcula el rectángulo de un nodo cuya esquina superior izquierda es position.
func RectOfNode(position Point) Rect {
	return Rect{
		X:      position.X,
		Y:      position.Y,
		Width:  NodeWidth,
		Height: NodeHeight,
	}
}

// Contains indica si point cae dentro del rectángulo ampliado margin por cada lado.
func (r Rect) Contains(point Point, margin float64) bool {
	return point.X >= r.X-margin &&
		point.X <= r.X+r.Width+margin &&
		point.Y >= r.Y-margin &&
		point.Y <= r.Y+r.Height+margin
}

// Intersects indica si dos rectángulos se solapan (compartir solo un borde no cuenta).
func (r Rect) Intersects(other Rect) bool {
	return r.X < other.X+other.Width &&
		other.X < r.X+r.Width &&
		r.Y < other.Y+other.Height &&
		other.Y < r.Y+r.Height
}

// Viewport es la vista del lienzo: pantalla = lienzo * zoom + (x, y).
type Viewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

// DefaultViewport devuelve la vista inicial por defecto: sin desplazamiento y con zoom 1.0.
func DefaultViewport() Viewport {
	return Viewport{
		X:    0.0,
		Y:    0.0,
		Zoom: 1.0,
	}
}

// ClampZoom limita un zoom al rango permitido [MinZoom, MaxZoom].
func ClampZoom(zoom float64) float64 {
	return min(max(zoom, MinZoom), MaxZoom)
}

// ScreenToCanvas convierte un punto de pantalla (relativo al lienzo) a coordenadas de lienzo.
func (v Viewport) ScreenToCanvas(screen Point) Point {
	return Point{
		X: (screen.X - v.X) / v.Zoom,
		Y: (screen.Y - v.Y) / v.Zoom,
	}
}

// CanvasToScreen convierte un punto de lienzo a pantalla.
func (v Viewport) CanvasToScreen(canvas Point) Point {
	return Point{
		X: canvas.X*v.Zoom + v.X,
		Y: canvas.Y*v.Zoom + v.Y,
	}
}

// Panned devuelve la vista desplazada dx, dy píxeles de pantalla.
func (v Viewport) Panned(dx, dy float64) Viewport {
	return Viewport{
		X:    v.X + dx,
		Y:    v.Y + dy,
		Zoom: v.Zoom,
	}
}

// ZoomedAt devuelve la vista con el zoom multiplicado por factor, manteniendo fijo el punto de pantalla
// anchor (el cursor).
func (v Viewport) ZoomedAt(anchor Point, factor float64) Viewport {
	zoom := ClampZoom(v.Zoom * factor)
	canvas := v.ScreenToCanvas(anchor)
	return Viewport{
		X:    anchor.X - canvas.X*zoom,
		Y:    anchor.Y - canvas.Y*zoom,
		Zoom: zoom,
	}
}

// TransformCSS devuelve el valor de la propiedad CSS transform de la capa de contenido.
func (v Viewport) TransformCSS() string {
	return fmt.Sprintf("translate(%.2fpx, %.2fpx) scale(%.4f)", v.X, v.Y, v.Zoom)
}

// WheelZoomFactor devuelve el factor de zoom que corresponde a un desplazamiento de rueda (deltaY > 0 aleja).
func WheelZoomFactor(deltaY float64) float64 {
	delta := min(max(deltaY, -WheelMaxDelta), WheelMaxDelta)
	return math.Exp(-delta * WheelSensitivity)
}

// BoundsOfNodes calcula el rectángulo mínimo que contiene a todos los nodos situados en positions.
func BoundsOfNodes(positions []Point) *Rect {
	if len(positions) == 0 {
		return nil
	}
	first := positions[0]
	minX, minY := first.X, first.Y
	maxX, maxY := first.X, first.Y
	for _, p := range positions[1:] {
		minX = min(minX, p.X)
		minY = min(minY, p.Y)
		maxX = max(maxX, p.X)
		maxY = max(maxY, p.Y)
	}
	return &Rect{
		X:      minX,
		Y:      minY,
		Width:  maxX - minX + NodeWidth,
		Height: maxY - minY + NodeHeight,
	}
}

// FitViewport devuelve la vista que encuadra bounds dentro de un lienzo de size (width, height) en píxeles.
// Sin contenido vuelve a la vista por defecto. No amplía por encima de zoom 1.
func FitViewport(bounds *Rect, width, height float64) Viewport {
	if bounds == nil || width <= 0.0 || height <= 0.0 || bounds.Width <= 0.0 || bounds.Height <= 0.0 {
		return DefaultViewport()
	}
	availableW := max(width-2.0*FitPadding, 1.0)
	availableH := max(height-2.0*FitPadding, 1.0)
	zoom := min(ClampZoom(min(availableW/bounds.Width, availableH/bounds.Height)), 1.0)
	return Viewport{
		X:    (width-bounds.Width*zoom)/2.0 - bounds.X*zoom,
		Y:    (height-bounds.Height*zoom)/2.0 - bounds.Y*zoom,
		Zoom: zoom,
	}
}

// OutputAnchor es el punto de salida (puerto derecho) de un nodo.
func OutputAnchor(position Point) Point {
	return Point{
		X: position.X + NodeWidth,
		Y: position.Y + NodeHeight/2.0,
	}
}

// InputAnchor es el punto de entrada (puerto izquierdo) de un nodo.
func InputAnchor(position Point) Point {
	return Point{
		X: position.X,
		Y: position.Y + NodeHeight/2.0,
	}
}

// controlOffset calcula la separación de los puntos de control de una Bézier entre from y to:
// proporcional a la distancia horizontal y mayor cuando la arista vuelve hacia atrás.
func controlOffset(from, to Point) float64 {
	dx := to.X - from.X
	if dx >= 0.0 {
		return max(dx*0.5, MinControlOffset)
	}
	return MinControlOffset + math.Abs(dx)*0.5
}

// controlPoints devuelve los puntos de control de la Bézier cúbica de una arista.
func controlPoints(from, to Point) (Point, Point) {
	offset := controlOffset(from, to)
	return Point{X: from.X + offset, Y: from.Y},
		Point{X: to.X - offset, Y: to.Y}
}

// EdgePath devuelve el atributo d de un <path> SVG: Bézier cúbica suave de from a to.
func EdgePath(from, to Point) string {
	c1, c2 := controlPoints(from, to)
	return fmt.Sprintf(
		"M %.2f %.2f C %.2f %.2f, %.2f %.2f, %.2f %.2f",
		from.X, from.Y, c1.X, c1.Y, c2.X, c2.Y, to.X, to.Y,
	)
}

// EdgeMidpoint calcula el punto central de la curva (t = 0.5): ancla de la etiqueta y del botón de borrado.
func EdgeMidpoint(from, to Point) Point {
	c1, c2 := controlPoints(from, to)
	return Point{
		X: (from.X + 3.0*c1.X + 3.0*c2.X + to.X) / 8.0,
		Y: (from.Y + 3.0*c1.Y + 3.0*c2.Y + to.Y) / 8.0,
	}
}

// PlacedNode representa un nodo situado en el lienzo con su identificador y posición.
type PlacedNode struct {
	ID       string
	Position Point
}

// NodeAt devuelve el identificador del nodo bajo point (el último de la lista, que es el que se pinta encima).
func NodeAt(point Point, nodes []PlacedNode) (string, bool) {
	for i := len(nodes) - 1; i >= 0; i-- {
		node := nodes[i]
		if RectOfNode(node.Position).Contains(point, DropMargin) {
			return node.ID, true
		}
	}
	return "", false
}
