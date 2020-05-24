// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package editor

import (
	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/mixins"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/vidar/commander/text"
	"github.com/nelsam/vidar/setting"
	"github.com/nelsam/vidar/theme"
)

type ProjectEditor struct {
	SplitEditor

	project setting.Project
}

func NewProjectEditor(driver gxui.Driver, window gxui.Window, cmdr Commander, theme *basic.Theme, syntaxTheme theme.Theme, font gxui.Font, project setting.Project) *ProjectEditor {
	p := &ProjectEditor{}
	p.driver = driver
	p.window = window
	p.cmdr = cmdr
	p.theme = theme
	p.syntaxTheme = syntaxTheme
	p.font = font
	p.SplitterLayout.Init(p, theme)
	p.SetOrientation(gxui.Horizontal)
	p.driver = driver
	p.theme = theme
	p.project = project
	p.SetMouseEventTarget(true)

	p.AddChild(NewTabbedEditor(driver, cmdr, theme, syntaxTheme, font))
	return p
}

func (p *ProjectEditor) Open(path string) (e text.Editor, existed bool) {
	return p.SplitEditor.Open(p.project.Path, path, p.project.LicenseHeader(), p.project.Environ())
}

func (p *ProjectEditor) Project() setting.Project {
	return p.project
}

type MultiProjectEditor struct {
	mixins.LinearLayout

	driver      gxui.Driver
	cmdr        Commander
	theme       *basic.Theme
	syntaxTheme theme.Theme
	font        gxui.Font
	window      gxui.Window

	current  *ProjectEditor
	projects map[string]*ProjectEditor
}

func New(driver gxui.Driver, window gxui.Window, cmdr Commander, theme *basic.Theme, syntaxTheme theme.Theme, font gxui.Font) *MultiProjectEditor {
	defaultEditor := NewProjectEditor(driver, window, cmdr, theme, syntaxTheme, font, setting.DefaultProject)

	e := &MultiProjectEditor{
		projects: map[string]*ProjectEditor{
			"*default*": defaultEditor,
		},
		driver:      driver,
		window:      window,
		cmdr:        cmdr,
		font:        font,
		theme:       theme,
		syntaxTheme: syntaxTheme,
	}
	e.LinearLayout.Init(e, theme)
	e.AddChild(defaultEditor)
	e.current = defaultEditor
	return e
}

func (e *MultiProjectEditor) SetProject(project setting.Project) {
	editor, ok := e.projects[project.Name]
	if !ok {
		editor = NewProjectEditor(e.driver, e.window, e.cmdr, e.theme, e.syntaxTheme, e.font, project)
		e.projects[project.Name] = editor
	}
	e.RemoveChild(e.current)
	e.AddChild(editor)
	e.current = editor
}

func (e *MultiProjectEditor) Elements() []interface{} {
	return []interface{}{
		e.current,
	}
}

func (e *MultiProjectEditor) CurrentEditor() text.Editor {
	return e.current.CurrentEditor()
}

func (e *MultiProjectEditor) CurrentFile() string {
	return e.current.CurrentFile()
}

func (e *MultiProjectEditor) CurrentProject() setting.Project {
	return e.current.Project()
}

func (e *MultiProjectEditor) Open(file string) (ed text.Editor, existed bool) {
	return e.current.Open(file)
}
