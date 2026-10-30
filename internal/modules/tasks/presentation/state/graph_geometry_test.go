package state_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/presentation/state"
)

func approx(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestScreenAndCanvasTransformsAreInverse(t *testing.T) {
	viewport := state.Viewport{
		X:    120.0,
		Y:    -40.0,
		Zoom: 0.5,
	}
	canvas := state.Point{X: 300.0, Y: 80.0}
	screen := viewport.CanvasToScreen(canvas)
	assert.Equal(t, state.Point{X: 270.0, Y: 0.0}, screen)
	back := viewport.ScreenToCanvas(screen)
	assert.True(t, approx(back.X, canvas.X) && approx(back.Y, canvas.Y))
}

func TestPanningMovesTheViewWithoutTouchingZoom(t *testing.T) {
	viewport := state.DefaultViewport().Panned(15.0, -5.0)
	assert.Equal(t, 15.0, viewport.X)
	assert.Equal(t, -5.0, viewport.Y)
	assert.Equal(t, 1.0, viewport.Zoom)
}

func TestZoomingKeepsThePointUnderTheCursorFixed(t *testing.T) {
	viewport := state.Viewport{
		X:    30.0,
		Y:    10.0,
		Zoom: 1.0,
	}
	anchor := state.Point{X: 200.0, Y: 150.0}
	before := viewport.ScreenToCanvas(anchor)
	zoomed := viewport.ZoomedAt(anchor, 1.5)
	after := zoomed.ScreenToCanvas(anchor)
	assert.True(t, approx(zoomed.Zoom, 1.5))
	assert.True(t, approx(before.X, after.X) && approx(before.Y, after.Y))
}

func TestZoomIsClampedToItsRange(t *testing.T) {
	viewport := state.DefaultViewport()
	assert.True(t, approx(viewport.ZoomedAt(state.Point{}, 100.0).Zoom, state.MaxZoom))
	assert.True(t, approx(viewport.ZoomedAt(state.Point{}, 0.0001).Zoom, state.MinZoom))
}

func TestWheelDownZoomsOutAndUpZoomsIn(t *testing.T) {
	assert.True(t, state.WheelZoomFactor(100.0) < 1.0)
	assert.True(t, state.WheelZoomFactor(-100.0) > 1.0)
	assert.True(t, approx(state.WheelZoomFactor(0.0), 1.0))
	assert.True(t, approx(state.WheelZoomFactor(1_000_000.0), state.WheelZoomFactor(state.WheelMaxDelta)))
}

func TestBoundsCoverEveryNode(t *testing.T) {
	assert.Nil(t, state.BoundsOfNodes([]state.Point{}))
	bounds := state.BoundsOfNodes([]state.Point{{X: 10.0, Y: 20.0}, {X: 500.0, Y: -30.0}})
	require.NotNil(t, bounds)
	assert.Equal(t, 10.0, bounds.X)
	assert.Equal(t, -30.0, bounds.Y)
	assert.Equal(t, 490.0+state.NodeWidth, bounds.Width)
	assert.Equal(t, 50.0+state.NodeHeight, bounds.Height)
}

func TestFitCentersTheContentAndNeverZoomsInPastOne(t *testing.T) {
	bounds := state.RectOfNode(state.Point{X: 0.0, Y: 0.0})
	viewport := state.FitViewport(&bounds, 2000.0, 1200.0)
	assert.True(t, approx(viewport.Zoom, 1.0))
	center := viewport.CanvasToScreen(state.Point{X: bounds.Width / 2.0, Y: bounds.Height / 2.0})
	assert.True(t, approx(center.X, 1000.0) && approx(center.Y, 600.0))
}

func TestFitShrinksLargeGraphsAndResetsWhenEmpty(t *testing.T) {
	bounds := state.Rect{
		X:      0.0,
		Y:      0.0,
		Width:  4000.0,
		Height: 1000.0,
	}
	viewport := state.FitViewport(&bounds, 800.0, 600.0)
	assert.True(t, approx(viewport.Zoom, state.MinZoom))

	medium := state.Rect{
		X:      0.0,
		Y:      0.0,
		Width:  1488.0,
		Height: 300.0,
	}
	assert.True(t, approx(state.FitViewport(&medium, 800.0, 600.0).Zoom, 0.4623655913978495))
	assert.Equal(t, state.DefaultViewport(), state.FitViewport(nil, 800.0, 600.0))
	assert.Equal(t, state.DefaultViewport(), state.FitViewport(&bounds, 0.0, 0.0))
}

func TestAnchorsSitOnTheVerticalMiddleOfEachSide(t *testing.T) {
	position := state.Point{X: 100.0, Y: 50.0}
	assert.Equal(t, state.Point{X: 100.0 + state.NodeWidth, Y: 50.0 + state.NodeHeight/2.0}, state.OutputAnchor(position))
	assert.Equal(t, state.Point{X: 100.0, Y: 50.0 + state.NodeHeight/2.0}, state.InputAnchor(position))
}

func TestEdgePathIsACubicCurveBetweenTheAnchors(t *testing.T) {
	path := state.EdgePath(state.Point{X: 0.0, Y: 0.0}, state.Point{X: 400.0, Y: 100.0})
	assert.Equal(t, "M 0.00 0.00 C 200.00 0.00, 200.00 100.00, 400.00 100.00", path)
}

func TestBackwardsEdgesBulgeFurtherThanTheMinimum(t *testing.T) {
	forward := state.EdgePath(state.Point{X: 0.0, Y: 0.0}, state.Point{X: 10.0, Y: 0.0})
	backward := state.EdgePath(state.Point{X: 300.0, Y: 0.0}, state.Point{X: 0.0, Y: 0.0})
	assert.Contains(t, forward, "C 48.00 0.00")
	assert.Contains(t, backward, "C 498.00 0.00")
}

func TestMidpointLiesHalfwayOnASymmetricCurve(t *testing.T) {
	mid := state.EdgeMidpoint(state.Point{X: 0.0, Y: 0.0}, state.Point{X: 400.0, Y: 100.0})
	assert.True(t, approx(mid.X, 200.0) && approx(mid.Y, 50.0))
}

func TestRectsContainWithMarginAndIntersectWithoutTouching(t *testing.T) {
	rect := state.RectOfNode(state.Point{X: 0.0, Y: 0.0})
	assert.True(t, rect.Contains(state.Point{X: 10.0, Y: 10.0}, 0.0))
	assert.False(t, rect.Contains(state.Point{X: -5.0, Y: 10.0}, 0.0))
	assert.True(t, rect.Contains(state.Point{X: -5.0, Y: 10.0}, state.DropMargin))

	touching := state.RectOfNode(state.Point{X: state.NodeWidth, Y: 0.0})
	assert.False(t, rect.Intersects(touching))

	overlapping := state.RectOfNode(state.Point{X: state.NodeWidth - 1.0, Y: 0.0})
	assert.True(t, rect.Intersects(overlapping))
}

func TestNodeAtPrefersTheTopmostNodeAndAcceptsTheInputPort(t *testing.T) {
	nodes := []state.PlacedNode{
		{ID: "a", Position: state.Point{X: 0.0, Y: 0.0}},
		{ID: "b", Position: state.Point{X: 100.0, Y: 20.0}},
	}
	id, ok := state.NodeAt(state.Point{X: 120.0, Y: 30.0}, nodes)
	assert.True(t, ok)
	assert.Equal(t, "b", id)

	id, ok = state.NodeAt(state.Point{X: 10.0, Y: 10.0}, nodes)
	assert.True(t, ok)
	assert.Equal(t, "a", id)

	// Sobre el puerto de entrada (ligeramente fuera del borde izquierdo)
	id, ok = state.NodeAt(state.Point{X: -6.0, Y: 70.0}, nodes)
	assert.True(t, ok)
	assert.Equal(t, "a", id)

	_, ok = state.NodeAt(state.Point{X: -500.0, Y: -500.0}, nodes)
	assert.False(t, ok)
}

func TestTransformCSSScalesAndTranslates(t *testing.T) {
	viewport := state.Viewport{
		X:    10.0,
		Y:    20.5,
		Zoom: 0.75,
	}
	assert.Equal(t, "translate(10.00px, 20.50px) scale(0.7500)", viewport.TransformCSS())
}
