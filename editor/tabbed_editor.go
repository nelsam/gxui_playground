// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package editor

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/math"
	"github.com/nelsam/gxui/mixins"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/vidar/command/focus"
	"github.com/nelsam/vidar/commander/text"
	"github.com/nelsam/vidar/theme"
)

type refocuser interface {
	ReFocus()
}

type TabbedEditor struct {
	mixins.PanelHolder

	editors map[string]text.Editor

	driver      gxui.Driver
	cmdr        Commander
	theme       *basic.Theme
	syntaxTheme theme.Theme
	font        gxui.Font
	cur         string
}

func NewTabbedEditor(driver gxui.Driver, cmdr Commander, theme *basic.Theme, syntaxTheme theme.Theme, font gxui.Font) *TabbedEditor {
	editor := &TabbedEditor{}
	editor.Init(editor, driver, cmdr, theme, syntaxTheme, font)
	return editor
}

func (e *TabbedEditor) Init(outer mixins.PanelHolderOuter, driver gxui.Driver, cmdr Commander, theme *basic.Theme, syntaxTheme theme.Theme, font gxui.Font) {
	e.editors = make(map[string]text.Editor)
	e.driver = driver
	e.cmdr = cmdr
	e.theme = theme
	e.syntaxTheme = syntaxTheme
	e.font = font
	e.PanelHolder.Init(outer, theme)
	e.SetMargin(math.Spacing{L: 0, T: 2, R: 0, B: 0})
}

func (e *TabbedEditor) Has(hiddenPrefix, path string) bool {
	_, ok := e.editors[relPath(hiddenPrefix, path)]
	return ok
}

func (e *TabbedEditor) Open(hiddenPrefix, path, headerText string, environ []string) (editor text.Editor, existed bool) {
	name := relPath(hiddenPrefix, path)
	if editor, ok := e.editors[name]; ok {
		e.Select(e.PanelIndex(editor.(gxui.Control)))
		gxui.SetFocus(editor.(gxui.Focusable))
		return editor, true
	}
	ce := &CodeEditor{}
	editor = ce
	// We want the OnRename trigger set up before the editor opens the file
	// in its Init method.
	ce.OnRename(func(newPath string) {
		e.driver.Call(func() {
			delete(e.editors, name)
			newName := relPath(hiddenPrefix, newPath)
			focused := e.SelectedPanel()
			e.editors[newName] = editor
			idx := e.PanelIndex(ce)
			if idx == -1 {
				return
			}
			e.RemovePanel(ce)
			e.AddPanelAt(ce, newName, idx)
			e.Select(e.PanelIndex(focused))
			gxui.SetFocus(focused.(gxui.Focusable))
		})
	})
	ce.Init(e.driver, e.theme, e.syntaxTheme, e.font, path, headerText)
	ce.SetTabWidth(4)
	e.Add(name, editor)
	return editor, false
}

func (e *TabbedEditor) Add(name string, editor text.Editor) {
	e.editors[name] = editor
	ec := editor.(gxui.Control)
	e.AddPanel(ec, name)
	e.Select(e.PanelIndex(ec))
	gxui.SetFocus(editor.(gxui.Focusable))
}

func (e *TabbedEditor) AddPanelAt(c gxui.Control, n string, i int) {
	e.PanelHolder.AddPanelAt(c, n, i)
	e.editors[n] = c.(text.Editor)
}

func (e *TabbedEditor) RemovePanel(panel gxui.Control) {
	toRemove := panel.(text.Editor)
	for name, editor := range e.editors {
		if editor == toRemove {
			delete(e.editors, name)
			break
		}
	}
	e.PanelHolder.RemovePanel(panel)
	if ed := e.CurrentEditor(); ed != nil {
		opener := e.cmdr.Bindable("focus-location").(Opener)
		e.cmdr.Execute(opener.For(focus.Path(ed.Filepath())))
	}
}

func (e *TabbedEditor) Files() []string {
	files := make([]string, 0, len(e.editors))
	for file := range e.editors {
		files = append(files, file)
	}
	return files
}

func (e *TabbedEditor) Editors() uint {
	return uint(len(e.editors))
}

func (e *TabbedEditor) CreatePanelTab() mixins.PanelTab {
	tab := basic.CreatePanelTab(e.theme)
	tab.OnMouseDown(func(ev gxui.MouseEvent) {
		if e.CurrentEditor() != nil {
			e.cur = e.CurrentEditor().Filepath()
		}
	})
	tab.OnMouseUp(func(gxui.MouseEvent) {
		if e.CurrentEditor() == nil {
			if len(e.editors) <= 1 {
				e.purgeSelf()
			} else {
				delete(e.editors, e.cur)
			}
		}
	})

	return tab
}

func (e *TabbedEditor) purgeSelf() {
	// Because of the order of events in gxui when a mouse drag happens,
	// the tab will move to a separate split *after* the SplitEditor's
	// MouseUp method is called, so the SplitEditor has no idea that
	// we're now empty.  We have to purge ourselves from the SplitEditor.
	parent := e.Parent()
	parent.(gxui.Container).RemoveChild(e)
	parent.(refocuser).ReFocus()
}

func (e *TabbedEditor) EditorAt(d Direction) text.Editor {
	panels := e.PanelCount()
	if panels < 2 {
		return e.CurrentEditor()
	}
	idx := e.PanelIndex(e.SelectedPanel())
	switch d {
	case Right:
		idx++
		if idx == panels {
			idx = 0
		}
	case Left:
		idx--
		if idx < 0 {
			idx = panels - 1
		}
	}
	return e.Panel(idx).(text.Editor)
}

func (e *TabbedEditor) CloseCurrentEditor() (name string, editor text.Editor) {
	toRemove := e.CurrentEditor()
	if toRemove == nil {
		return "", nil
	}
	name = ""
	for key, panel := range e.editors {
		if panel == toRemove {
			name = key
			break
		}
	}
	e.RemovePanel(toRemove.(gxui.Control))
	if name == "" {
		return "", nil
	}
	return name, toRemove
}

func (e *TabbedEditor) SaveAll() {
	for name, editor := range e.editors {
		f, err := os.Create(name)
		if err != nil {
			log.Printf("Could not save %s : %s", name, err)
		}
		defer f.Close()
		if _, err := f.WriteString(editor.Text()); err != nil {
			log.Printf("Could not write to file %s: %s", name, err)
		}
	}
}

func (e *TabbedEditor) CurrentEditor() text.Editor {
	if e.SelectedPanel() == nil {
		return nil
	}
	return e.SelectedPanel().(text.Editor)
}

func (e *TabbedEditor) CurrentFile() string {
	if e.SelectedPanel() == nil {
		return ""
	}
	return e.SelectedPanel().(text.Editor).Filepath()
}

func (e *TabbedEditor) Elements() []interface{} {
	if e.SelectedPanel() == nil {
		return nil
	}
	return []interface{}{e.SelectedPanel()}
}

func relPath(from, path string) string {
	rel := strings.TrimPrefix(path, from)
	if rel[0] == filepath.Separator {
		rel = rel[1:]
	}
	return rel
}
