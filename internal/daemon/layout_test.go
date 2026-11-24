package daemon

import (
	"testing"

	"vector-foundry-144/internal/model"
)

// leaf builds a pane node holding a window with the given id.
func leaf(id string) *model.PaneNode {
	return &model.PaneNode{Leaf: &model.Window{ID: id}}
}

// split builds a split pane node with the given axis, ratio and children.
func split(horizontal bool, ratio float64, a, b *model.PaneNode) *model.PaneNode {
	return &model.PaneNode{Split: &model.SplitNode{
		Horizontal: horizontal,
		Ratio:      ratio,
		Children:   [2]*model.PaneNode{a, b},
	}}
}

func TestSplitRatioDefaultsAndClamps(t *testing.T) {
	if got := splitRatio(0); got != 0.5 {
		t.Fatalf("splitRatio(0) = %v, want 0.5", got)
	}
	if got := splitRatio(-1); got != 0.5 {
		t.Fatalf("splitRatio(-1) = %v, want 0.5", got)
	}
	if got := splitRatio(0.42); got != 0.42 {
		t.Fatalf("splitRatio(0.42) = %v, want 0.42", got)
	}
	if got := splitRatio(0.99); got != 0.9 {
		t.Fatalf("splitRatio(0.99) = %v, want 0.9", got)
	}
	if got := splitRatio(0.01); got != 0.1 {
		t.Fatalf("splitRatio(0.01) = %v, want 0.1", got)
	}
}

func TestSplitPaneNodeUsesUsableRatio(t *testing.T) {
	d := &Daemon{}
	root := leaf("window-1")
	got := d.splitPaneNode(root, "window-1", &model.Window{ID: "window-2"}, true, 0)

	if got.Split == nil {
		t.Fatal("expected a split node")
	}
	if !got.Split.Horizontal {
		t.Fatal("expected horizontal split")
	}
	if got.Split.Ratio != 0.5 {
		t.Fatalf("ratio = %v, want 0.5 for an unset ratio", got.Split.Ratio)
	}
	if !paneContains(got, "window-1") || !paneContains(got, "window-2") {
		t.Fatal("split must contain both panes")
	}
}

func TestResizePaneOnlyTouchesMatchingAxis(t *testing.T) {
	d := &Daemon{}
	// root: horizontal split [ A, vertical split [ B, C ] ]
	root := split(true, 0.5,
		leaf("A"),
		split(false, 0.5, leaf("B"), leaf("C")),
	)

	// "h" names the horizontal split axis: it resizes the outer split that
	// holds B, and leaves the inner vertical split alone.
	got := d.resizePaneNode(root, "B", 10, "h")
	if got.Split.Ratio != 0.6 {
		t.Fatalf("outer ratio = %v, want 0.6", got.Split.Ratio)
	}
	if inner := got.Split.Children[1]; inner.Split.Ratio != 0.5 {
		t.Fatalf("inner ratio = %v, want unchanged 0.5", inner.Split.Ratio)
	}

	// "v" names the vertical split axis: it resizes the inner split that
	// holds B, and leaves the outer horizontal split alone.
	got = d.resizePaneNode(root, "B", 10, "v")
	if inner := got.Split.Children[1]; inner.Split.Ratio != 0.6 {
		t.Fatalf("inner ratio = %v, want 0.6", inner.Split.Ratio)
	}
	if got.Split.Ratio != 0.6 {
		t.Fatalf("outer ratio = %v, want unchanged 0.6", got.Split.Ratio)
	}
}

func TestResizePaneClampsRatio(t *testing.T) {
	d := &Daemon{}
	root := split(true, 0.5, leaf("A"), leaf("B"))

	d.resizePaneNode(root, "A", 500, "h")
	if root.Split.Ratio != 0.9 {
		t.Fatalf("ratio = %v, want clamped 0.9", root.Split.Ratio)
	}

	d.resizePaneNode(root, "A", -500, "h")
	if root.Split.Ratio != 0.1 {
		t.Fatalf("ratio = %v, want clamped 0.1", root.Split.Ratio)
	}
}

func TestPaneTreeNodeCarriesFocusAndSplits(t *testing.T) {
	root := split(true, 0.4, leaf("A"), leaf("B"))
	node := paneTreeNode(root, &model.Window{ID: "B"})

	if node.Split == nil || !node.Split.Horizontal || node.Split.Ratio != 0.4 {
		t.Fatalf("unexpected split node: %+v", node)
	}
	if node.Split.Children[0].Focused {
		t.Fatal("A must not be focused")
	}
	if !node.Split.Children[1].Focused {
		t.Fatal("B must be focused")
	}
	if node.Split.Children[1].WindowID != "B" {
		t.Fatalf("window id = %q, want B", node.Split.Children[1].WindowID)
	}
}
