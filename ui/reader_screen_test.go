package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

func readerTestContext() layout.Context {
	return layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(833, 545)),
	}
}

// TestReaderListsBalanceTheirOperationStacks guards the reader against the
// double Pop that made Gio panic with "unbalanced operation" as soon as a
// manga or ranobe chapter was shown.
func TestReaderListsBalanceTheirOperationStacks(t *testing.T) {
	typo, err := newTypography()
	if err != nil {
		t.Fatal(err)
	}
	r := &readerScreen{
		typo:     typo,
		chapters: []string{"1:1", "1:2"},
		filtered: []string{"1:1", "1:2"},
	}
	r.chapterClick = make([]widget.Clickable, len(r.chapters))

	gtx := readerTestContext()
	r.layoutChapterList(gtx, image.Rect(0, 0, 280, 400))
	gtx.Ops.Reset()

	r.pages = []readerPageState{{url: "https://example.test/1.png", loading: true}}
	r.layoutManga(gtx, image.Rect(0, 0, 500, 400))
	gtx.Ops.Reset()

	r.content.Paragraphs = []string{"first paragraph", "second paragraph"}
	r.layoutRanobe(gtx, image.Rect(0, 0, 500, 400))
}
