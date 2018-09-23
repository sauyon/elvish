package utils

import (
	"unicode/utf8"

	"github.com/elves/elvish/edit/tty"
	"github.com/elves/elvish/edit/ui"
	"github.com/elves/elvish/newedit/types"
)

// BasicMode is a basic Mode implementation.
type BasicMode struct{}

// ModeLine returns nil.
func (BasicMode) ModeLine() ui.Renderer {
	return nil
}

// ModeRenderFlag returns 0.
func (BasicMode) ModeRenderFlag() types.ModeRenderFlag {
	return 0
}

// HandleEvent uses BasicHandler to handle the event.
func (BasicMode) HandleEvent(e tty.Event, st *types.State) types.HandlerAction {
	return BasicHandler(e, st)
}

// BasicHandler is a basic implementation of an event handler. It is used in
// BasicMode.HandleEvent, but can also be used in other modes as a fallback
// handler.
func BasicHandler(e tty.Event, st *types.State) types.HandlerAction {
	keyEvent, ok := e.(tty.KeyEvent)
	if !ok {
		return types.NoAction
	}
	k := ui.Key(keyEvent)

	st.Mutex.Lock()
	defer st.Mutex.Unlock()

	raw := &st.Raw

	switch k {
	case ui.Key{Rune: '\n'}:
		return types.CommitCode
	case ui.Key{Rune: ui.Backspace}:
		beforeDot := raw.Code[:raw.Dot]
		afterDot := raw.Code[raw.Dot:]
		_, chop := utf8.DecodeLastRuneInString(beforeDot)
		raw.Code = beforeDot[:len(beforeDot)-chop] + afterDot
		raw.Dot -= chop
	case ui.Key{Rune: ui.Left}:
		_, skip := utf8.DecodeLastRuneInString(raw.Code[:raw.Dot])
		raw.Dot -= skip
	case ui.Key{Rune: ui.Right}:
		_, skip := utf8.DecodeRuneInString(raw.Code[raw.Dot:])
		raw.Dot += skip
	default:
		if k.Mod == 0 {
			s := string(k.Rune)
			raw.Code = raw.Code[:raw.Dot] + s + raw.Code[raw.Dot:]
			raw.Dot += len(s)
		}
	}
	return types.NoAction
}
