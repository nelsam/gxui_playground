// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package command

import (
	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/vidar/commander/bind"
)

type FileHook struct {
	Theme  *basic.Theme
	Driver gxui.Driver
}

func (h FileHook) Name() string {
	return "file-hook"
}

func (h FileHook) OpName() string {
	return "focus-location"
}

func (h FileHook) FileBindables(string) []bind.Bindable {
	return []bind.Bindable{
		NewSave(h.Theme),
		NewSaveAs(h.Driver, h.Theme),
		NewSaveAll(h.Theme),
		NewCloseTab(),
		&EditorRedraw{},
	}
}
