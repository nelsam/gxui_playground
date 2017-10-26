// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package editor

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nelsam/gxui"
	"github.com/nelsam/gxui/math"
	"github.com/nelsam/gxui/mixins"
	"github.com/nelsam/gxui/themes/basic"
	"github.com/nelsam/vidar/commander/input"
	"github.com/nelsam/vidar/theme"
)

type CodeEditor struct {
	mixins.CodeEditor

	theme       *basic.Theme
	syntaxTheme theme.Theme
	driver      gxui.Driver

	lastModified time.Time
	hasChanges   bool
	filepath     string

	watcher *fsnotify.Watcher

	selections      gxui.TextSelectionList
	scrollPositions math.Point
	layers          []input.SyntaxLayer

	renamed  bool
	onRename func(newPath string)
}

func (e *CodeEditor) Init(driver gxui.Driver, theme *basic.Theme, syntaxTheme theme.Theme, font gxui.Font, file, headerText string) {
	e.theme = theme
	e.syntaxTheme = syntaxTheme
	e.driver = driver

	e.CodeEditor.Init(e, driver, theme, font)
	e.CodeEditor.SetScrollBarEnabled(true)
	e.SetDesiredWidth(math.MaxSize.W)
	e.watcherSetup()

	// TODO: move to hooks on the input.Handler
	e.OnTextChanged(func(changes []gxui.TextBoxEdit) {
		e.hasChanges = true
	})
	e.filepath = file
	e.open(headerText)

	e.SetTextColor(theme.TextBoxDefaultStyle.FontColor)
	e.SetMargin(math.Spacing{L: 3, T: 3, R: 3, B: 3})
	e.SetPadding(math.Spacing{L: 3, T: 3, R: 3, B: 3})
	e.SetBorderPen(gxui.TransparentPen)
}

func (e *CodeEditor) DataChanged(recreate bool) {
	e.List.DataChanged(recreate)
}

func (e *CodeEditor) Carets() []int {
	return e.Controller().Carets()
}

func (e *CodeEditor) SetCarets(carets ...int) {
	if len(carets) == 0 {
		e.Controller().ClearSelections()
		return
	}
	e.Controller().SetCaret(carets[0])
	for _, c := range carets[1:] {
		e.Controller().AddCaret(c)
	}
}

func (e *CodeEditor) OnRename(callback func(newPath string)) {
	e.onRename = callback
}

func (e *CodeEditor) open(headerText string) {
	go e.watch()
	e.load(headerText)
}

func (e *CodeEditor) watcherSetup() {
	var err error
	e.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating new fsnotify watcher: %s", err)
	}
}

func (e *CodeEditor) startWatch() error {
	err := e.watcher.Add(e.filepath)
	if os.IsNotExist(err) {
		err = e.waitForFileCreate()
		if err != nil {
			return err
		}
		return e.startWatch()
	}
	return nil
}

func (e *CodeEditor) watch() {
	if e.watcher == nil {
		return
	}
	err := e.startWatch()
	if err != nil {
		log.Printf("Error trying to watch %s for changes: %s", e.filepath, err)
		return
	}
	defer e.watcher.Remove(e.filepath)
	fileDir := filepath.Dir(e.filepath)
	err = e.watcher.Add(fileDir)
	if err != nil {
		log.Printf("Error trying to watch %s for changes: %s", fileDir, err)
		return
	}
	defer e.watcher.Remove(fileDir)
	err = e.inotifyWait(func(event fsnotify.Event) bool {
		if event.Name != e.filepath {
			if e.renamed && event.Op == fsnotify.Create {
				e.filepath = event.Name
				e.onRename(e.filepath)
				e.open("")
				return true
			}
			return false
		}
		switch event.Op {
		case fsnotify.Write:
			e.load("")
		case fsnotify.Rename:
			e.renamed = true
		case fsnotify.Remove:
			e.open("")
			return true
		}
		return false
	})
	if err != nil {
		log.Printf("Failed to wait on events for %s: %s", e.filepath, err)
		return
	}
}

func (e *CodeEditor) waitForFileCreate() error {
	dir := filepath.Dir(e.filepath)
	if err := os.MkdirAll(dir, 0750|os.ModeDir); err != nil {
		return err
	}
	if err := e.watcher.Add(dir); err != nil {
		return err
	}
	defer e.watcher.Remove(dir)

	return e.inotifyWait(func(event fsnotify.Event) bool {
		return event.Name == e.filepath && event.Op&fsnotify.Create == fsnotify.Create
	})
}

func (e *CodeEditor) inotifyWait(eventFunc func(fsnotify.Event) (done bool)) error {
	for {
		select {
		case event := <-e.watcher.Events:
			if eventFunc(event) {
				return nil
			}
		case err := <-e.watcher.Errors:
			return err
		}
	}
}

func (e *CodeEditor) load(headerText string) {
	f, err := os.Open(e.filepath)
	if os.IsNotExist(err) {
		e.driver.Call(func() {
			e.SetText(headerText)
		})
		return
	}
	if err != nil {
		log.Printf("Error opening file %s: %s", e.filepath, err)
		return
	}
	defer f.Close()
	finfo, err := f.Stat()
	if err != nil {
		log.Printf("Error stating file %s: %s", e.filepath, err)
		return
	}
	e.lastModified = finfo.ModTime()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("Error reading file %s: %s", e.filepath, err)
		return
	}
	newText := string(b)
	if !strings.HasPrefix(newText, headerText) {
		log.Printf("%s: header text does not match requested header text", e.filepath)
	}
	e.driver.Call(func() {
		if e.Text() == newText {
			return
		}
		e.SetText(newText)
		e.restorePositions()
	})
}

func (e *CodeEditor) HasChanges() bool {
	return e.hasChanges
}

func (e *CodeEditor) LastKnownMTime() time.Time {
	return e.lastModified
}

func (e *CodeEditor) Filepath() string {
	return e.filepath
}

func (e *CodeEditor) FlushedChanges() {
	e.hasChanges = false
	e.lastModified = time.Now()
}

func (e *CodeEditor) Elements() []interface{} {
	return []interface{}{
		e.Controller(),
	}
}

func (e *CodeEditor) SetSyntaxLayers(layers []input.SyntaxLayer) {
	defer e.syntaxTheme.Rainbow.Reset()
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].Construct < layers[j].Construct
	})
	e.layers = layers
	gLayers := make(gxui.CodeSyntaxLayers, 0, len(layers))
	for _, l := range layers {
		highlight, found := e.syntaxTheme.Constructs[l.Construct]
		if !found {
			highlight = e.syntaxTheme.Rainbow.Next()
		}
		gLayer := gxui.CreateCodeSyntaxLayer()
		gLayer.SetColor(gxui.Color(highlight.Foreground))
		gLayer.SetBackgroundColor(gxui.Color(highlight.Background))
		for _, s := range l.Spans {
			gLayer.Add(s.Start, s.End-s.Start)
		}
		gLayers = append(gLayers, gLayer)
	}
	e.CodeEditor.SetSyntaxLayers(gLayers)
}

func (e *CodeEditor) SyntaxLayers() []input.SyntaxLayer {
	return e.layers
}

func (e *CodeEditor) Paint(c gxui.Canvas) {
	e.CodeEditor.Paint(c)

	if e.HasFocus() {
		r := e.Size().Rect()
		c.DrawRoundedRect(r, 3, 3, 3, 3, e.theme.FocusedStyle.Pen, e.theme.FocusedStyle.Brush)
	}
}

func (e *CodeEditor) storePositions() {
	e.selections = e.Controller().Selections()
	e.scrollPositions = math.Point{
		X: e.HorizOffset(),
		Y: e.ScrollOffset(),
	}
}

func (e *CodeEditor) restorePositions() {
	e.Controller().SetSelections(e.selections)
	e.SetHorizOffset(e.scrollPositions.X)
	e.SetScrollOffset(e.scrollPositions.Y)
}

func (e *CodeEditor) KeyPress(event gxui.KeyboardEvent) bool {
	defer e.storePositions()
	if event.Modifier != 0 && event.Modifier != gxui.ModShift {
		// ctrl-, alt-, and cmd-/super- keybindings should be dealt
		// with by the commands mapped to key bindings.
		return false
	}
	switch event.Key {
	case gxui.KeyPageUp, gxui.KeyPageDown:
		// These are all bindings that the TextBox handles fine.
		return e.TextBox.KeyPress(event)
	case gxui.KeyTab:
		// TODO: Gain knowledge about scope, so we know how much to indent.
		switch {
		case event.Modifier.Shift():
			e.Controller().UnindentSelection()
		default:
			e.Controller().IndentSelection()
		}
		return true
	case gxui.KeyEscape:
		// TODO: Keep track of some sort of concept of a "focused" caret and
		// focus that.
		e.Controller().SetCaret(e.Controller().FirstCaret())
	}
	return false
}

func (e *CodeEditor) KeyStroke(event gxui.KeyStrokeEvent) (consume bool) {
	return false
}

func (e *CodeEditor) CreateLine(theme gxui.Theme, index int) (mixins.TextBoxLine, gxui.Control) {
	lineNumber := theme.CreateLabel()
	lineNumber.SetText(fmt.Sprintf("%4d", index+1))

	line := &mixins.CodeEditorLine{}
	line.Init(line, theme, &e.CodeEditor, index)

	layout := theme.CreateLinearLayout()
	layout.SetDirection(gxui.LeftToRight)
	layout.AddChild(lineNumber)
	layout.AddChild(line)

	return line, layout
}
