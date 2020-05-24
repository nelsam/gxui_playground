// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package comments

import (
	"fmt"
	"regexp"

	"github.com/nelsam/gxui"
	"github.com/nelsam/vidar/commander/bind"
	"github.com/nelsam/vidar/commander/text"
)

type Applier interface {
	Apply(text.Editor, ...text.Edit)
}

type Editor interface {
	text.Editor
}

type Selecter interface {
	SelectionSlice() []gxui.TextSelection
}

type Toggle struct {
	editor   text.Editor
	applier  Applier
	selecter Selecter
}

func NewToggle() *Toggle {
	return &Toggle{}
}

func (c *Toggle) Name() string {
	return "toggle-comments"
}

func (c *Toggle) Menu() string {
	return "Golang"
}

func (c *Toggle) Defaults() []fmt.Stringer {
	return []fmt.Stringer{gxui.KeyboardEvent{
		Modifier: gxui.ModControl,
		Key:      gxui.KeySlash,
	}}
}

func (t *Toggle) Reset() {
	t.editor = nil
	t.applier = nil
	t.selecter = nil
}

func (t *Toggle) Store(target interface{}) bind.Status {
	switch src := target.(type) {
	case Applier:
		t.applier = src
	case text.Editor:
		t.editor = src
	case Selecter:
		t.selecter = src
	}
	if t.editor != nil && t.applier != nil && t.selecter != nil {
		return bind.Done
	}
	return bind.Waiting
}

func (t *Toggle) Exec() error {
	selections := t.selecter.SelectionSlice()

	var edits []text.Edit
	for i := len(selections) - 1; i >= 0; i-- {
		begin, end := selections[i].Start(), selections[i].End()
		runes := t.editor.Runes()[begin:end]
		str := string(runes)
		re, replace := regexpReplace(str)
		newstr := re.ReplaceAllString(str, replace)
		edits = append(edits, text.Edit{
			At:  int(begin),
			Old: runes,
			New: []rune(newstr),
		})
	}
	t.applier.Apply(t.editor, edits...)
	return nil
}

func regexpReplace(str string) (*regexp.Regexp, string) {
	if regexp.MustCompile(`^(\s*?)//`).MatchString(str) {
		return regexp.MustCompile(`(?m)^(\s*?)//(.*)$`), `${1}${2}`
	}
	return regexp.MustCompile("(?m)^(.*)$"), `//${1}`
}
