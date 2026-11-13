package play

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestViewKeepsLongInputCursorVisibleAndAlignsBoard(t *testing.T) {
	model := activeModel(t)
	model.input = strings.Repeat("界", 60) + "e2e4"
	view := model.View(74, true)
	input := strings.Split(view, "\n")[1]
	if lipgloss.Width(input) > 74 || !strings.HasSuffix(input, "e2e4▏") {
		t.Fatalf("input overflowed or hid cursor: %q", input)
	}
	if !strings.HasPrefix(model.board(), " A B C D E F G H\n8") {
		t.Fatalf("board labels do not align: %s", model.board())
	}
}
