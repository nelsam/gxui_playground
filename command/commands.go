// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package command

import (
	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/vidar/command/caret"
	"github.com/nelsam/vidar/command/focus"
	"github.com/nelsam/vidar/command/history"
	"github.com/nelsam/vidar/command/project"
	"github.com/nelsam/vidar/command/scroll"
	"github.com/nelsam/vidar/commander/bind"
	"github.com/nelsam/vidar/plugin/command"
)

// Bindables returns all known bindables, in the order they should be
// added to the menu.
func Bindables(cmdr command.Commander, driver gxui.Driver, theme *basic.Theme) []bind.Bindable {
	var b []bind.Bindable
	b = append(b, project.Bindables(driver, theme)...)
	b = append(b,
		NewFileOpener(driver, theme),
		Quit{},
		Fullscreen{},
		&caret.Mover{},
		&scroll.Scroller{},
		focus.NewLocation(driver),
		FileHook{Theme: theme, Driver: driver},
		EditHook{Theme: theme, Driver: driver},
		ViewHook{},
		NavHook{Commander: cmdr},
	)
	b = append(b, history.Bindables(cmdr, driver, theme)...)
	return b
}
