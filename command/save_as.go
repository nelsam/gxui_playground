// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package command

import (
	"errors"
	"fmt"
	"os"

	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/vidar/command/focus"
	"github.com/nelsam/vidar/command/fs"
	"github.com/nelsam/vidar/commander/bind"
	"github.com/nelsam/vidar/plugin/status"
)

type Binder interface {
	Pop() []bind.Bindable
	Execute(bind.Bindable)
}

type FileCopySaver struct {
	status.General

	driver gxui.Driver

	file  *fs.Locator
	input <-chan gxui.Focusable

	focuser Focuser
	closer  CurrentEditorCloser
	binder  Binder
}

func NewSaveAs(driver gxui.Driver, theme *basic.Theme) *FileCopySaver {
	o := &FileCopySaver{
		driver: driver,
	}
	o.file = fs.NewLocator(driver, theme, fs.All)
	o.Theme = theme
	return o
}

func (s *FileCopySaver) Name() string {
	return "save-as"
}

func (s *FileCopySaver) Menu() string {
	return "File"
}

func (s *FileCopySaver) Defaults() []fmt.Stringer {
	return []fmt.Stringer{gxui.KeyboardEvent{
		Modifier: gxui.ModControl | gxui.ModShift,
		Key:      gxui.KeyC,
	}}
}

func (f *FileCopySaver) Start(control gxui.Control) gxui.Control {
	f.file.LoadDir(control)
	input := make(chan gxui.Focusable, 1)
	f.input = input
	input <- f.file
	close(input)
	return nil
}

func (f *FileCopySaver) Next() gxui.Focusable {
	return <-f.input
}

func (f *FileCopySaver) Reset() {
	f.focuser = nil
	f.closer = nil
	f.binder = nil
}

func (f *FileCopySaver) Store(elem interface{}) bind.Status {
	switch src := elem.(type) {
	case Focuser:
		f.focuser = src
	case Binder:
		f.binder = src
	case CurrentEditorCloser:
		f.closer = src
	}

	if f.focuser == nil || f.binder == nil || f.closer == nil {
		return bind.Waiting
	}
	return bind.Executing
}

func (f *FileCopySaver) Exec() error {
	filepath := f.file.Path()
	if filepath == "" {
		f.Err = "command.FileCopySaver: No file path provided"
		return errors.New(f.Err)
	}
	out, err := os.Create(filepath)
	if err != nil {
		f.Err = fmt.Sprintf("Could not open %s for writing: %s", filepath, err)
		return errors.New(f.Err)
	}
	if _, err := out.WriteString(f.closer.CurrentEditor().Text()); err != nil {
		f.Err = fmt.Sprintf("Could not write to file %s: %s", filepath, err)
		return errors.New(f.Err)
	}
	out.Close()
	f.closer.CloseCurrentEditor()
	f.binder.Pop()
	f.binder.Execute(f.focuser.For(focus.Path(filepath)))
	f.Info = fmt.Sprintf("Successfully saved %s", filepath)
	return nil
}
