// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package gocode

import (
	"context"
	"log"
	"unicode"

	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/math"
	"github.com/nelsam/gxui/mixins"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/vidar/commander/text"
	"github.com/nelsam/vidar/setting"
	"github.com/nelsam/vidar/suggestion"
)

type Editor interface {
	text.Editor
	gxui.Parent

	Carets() []int
	Size() math.Size
	Padding() math.Spacing
	LineIndex(caret int) int
	Line(idx int) mixins.TextBoxLine
	AddChild(gxui.Control) *gxui.Child
	RemoveChild(gxui.Control)
}

type Applier interface {
	Apply(text.Editor, ...text.Edit)
}

type suggestionList struct {
	mixins.List
	driver  gxui.Driver
	adapter *suggestion.Adapter
	font    gxui.Font
	project setting.Project
	editor  Editor
	ctrl    TextController
	applier Applier
	gocode  *GoCode
}

func newSuggestionList(driver gxui.Driver, theme *basic.Theme, proj setting.Project, editor Editor, ctrl TextController, applier Applier, gocode *GoCode) *suggestionList {
	s := &suggestionList{
		driver:  driver,
		adapter: &suggestion.Adapter{},
		font:    theme.DefaultMonospaceFont(),
		project: proj,
		editor:  editor,
		ctrl:    ctrl,
		applier: applier,
		gocode:  gocode,
	}

	s.Init(s, theme)
	s.OnGainedFocus(s.Redraw)
	s.OnLostFocus(s.Redraw)
	s.OnKeyPress(func(ev gxui.KeyboardEvent) {
		if ev.Key == gxui.KeyEnter {
			s.driver.CallSync(func() {
				s.gocode.Confirm(s.editor)
			})
		} else if ev.Key == gxui.KeyEscape {
			s.driver.CallSync(func() {
				s.gocode.Cancel(s.editor)
			})
		}
	})
	s.OnDoubleClick(func(gxui.MouseEvent) {
		s.driver.CallSync(func() {
			s.gocode.Confirm(s.editor)
		})
	})

	s.SetPadding(math.CreateSpacing(2))
	s.SetBackgroundBrush(theme.CodeSuggestionListStyle.Brush)
	s.SetBorderPen(theme.CodeSuggestionListStyle.Pen)

	s.SetAdapter(s.adapter)
	return s
}

func (s *suggestionList) show(ctx context.Context, pos int) int {
	runes := s.ctrl.TextRunes()
	if pos >= len(runes) {
		log.Printf("Warning: suggestion list sees a pos of %d while the rune length of the editor is %d", pos, len(runes))
		return 0
	}

	start := pos
	for start > 0 && wordPart(runes[start-1]) {
		start--
	}
	if s.adapter.Pos() != start {
		if ctxCancelled(ctx) {
			return 0
		}
		suggestion := s.parseSuggestions(runes, start)
		s.adapter.Set(start, suggestion...)
	}
	if ctxCancelled(ctx) {
		return 0
	}

	s.driver.CallSync(func() {
		// TODO: This doesn't always create a large enough box; it needs to be fixed.
		longest := s.adapter.Sort(runes[start:pos])
		if s.adapter.Len() == 0 {
			return
		}
		s.Select(s.adapter.ItemAt(0))

		size := s.font.GlyphMaxSize()
		size.W *= longest
		s.adapter.SetSize(size)
	})
	return s.adapter.Len()
}

func (s *suggestionList) parseSuggestions(runes []rune, start int) []suggestion.Suggestion {
	suggestion, err := suggestion.For(s.project.Environ(), s.editor.Filepath(), string(runes), start)
	if err != nil {
		log.Printf("Failed to load suggestion: %s", err)
		return nil
	}
	return suggestion
}

func (s *suggestionList) apply() {
	suggestion := s.Selected().(suggestion.Suggestion)
	start := s.adapter.Pos()
	carets := s.ctrl.Carets()
	if len(carets) != 1 {
		log.Printf("Cannot apply completion to more than one caret; got %d", len(carets))
		return
	}
	end := carets[0]
	runes := s.ctrl.TextRunes()

	if start <= end {
		go s.applier.Apply(s.editor, text.Edit{
			At:  start,
			Old: runes[start:end],
			New: []rune(suggestion.Name),
		})
	}
}

func wordPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}
