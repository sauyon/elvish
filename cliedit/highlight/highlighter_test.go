package highlight

import (
	"reflect"
	"testing"
	"time"

	"github.com/elves/elvish/diag"
	"github.com/elves/elvish/parse"
	"github.com/elves/elvish/styled"
	"github.com/elves/elvish/tt"
)

var any = anyMatcher{}
var noErrors []error

var styles = map[rune]string{
	'x': "bg-red",
	'v': "magenta",
	'q': "yellow",
	'G': "green",
	'B': "red",
}

func TestHighlighter_HighlightRegions(t *testing.T) {
	hl := NewHighlighter(Config{})

	tt.Test(t, tt.Fn("hl.Get", hl.Get), tt.Table{
		Args("ls").Rets(
			styled.MarkLines(
				"ls", styles,
				"GG",
			),
			noErrors),
		Args(" ls\n").Rets(
			styled.MarkLines(
				" ls\n", styles,
				" GG"),
			noErrors),
		Args("ls $x 'y'").Rets(
			styled.MarkLines(
				"ls $x 'y'", styles,
				"GG vv qqq"),
			noErrors),
	})
}

func TestHighlighter_ParseErrors(t *testing.T) {
	hl := NewHighlighter(Config{})
	tt.Test(t, tt.Fn("hl.Get", hl.Get), tt.Table{
		// Parse error is highlighted and returned
		Args("ls ]").Rets(
			styled.MarkLines(
				"ls ]", styles,
				"GG x"),
			matchErrors(parseErrorMatcher{3, 4})),
		// Errors at the end are ignored
		Args("ls $").Rets(any, noErrors),
		Args("ls [").Rets(any, noErrors),
	})
}

func TestHighlighter_CheckErrors(t *testing.T) {
	var checkError error
	// Make a highlighter whose Check callback returns checkError.
	hl := NewHighlighter(Config{
		Check: func(*parse.Chunk) error { return checkError }})
	getWithCheckError := func(code string, err error) (styled.Text, []error) {
		checkError = err
		return hl.Get(code)
	}

	tt.Test(t, tt.Fn("getWithCheckError", getWithCheckError), tt.Table{
		// Check error is highlighted and returned
		Args("code 1", fakeCheckError{5, 6}).Rets(
			styled.MarkLines(
				"code 1", styles,
				"GGGG x"),
			[]error{fakeCheckError{5, 6}}),
		// Check errors at the end are ignored
		Args("code 2", fakeCheckError{6, 6}).
			Rets(any, noErrors),
	})
}

const lateTimeout = 100 * time.Millisecond

func TestHighlighter_HasCommand_LateResult(t *testing.T) {
	// Make a highlighter whose HasCommand callback only recognizes "ls".
	hl := NewHighlighter(Config{
		HasCommand: func(cmd string) bool { return cmd == "ls" }})

	test := func(code string, wantInitial, wantLate styled.Text) {
		initial, _ := hl.Get(code)
		if !reflect.DeepEqual(wantInitial, initial) {
			t.Errorf("want %v from initial Get, got %v", wantInitial, initial)
		}
		select {
		case late := <-hl.LateUpdates():
			if !reflect.DeepEqual(wantLate, late) {
				t.Errorf("want %v from LateUpdates, got %v", wantLate, late)
			}
			late, _ = hl.Get(code)
			if !reflect.DeepEqual(wantLate, late) {
				t.Errorf("want %v from late Get, got %v", wantLate, late)
			}
		case <-time.After(lateTimeout):
			t.Errorf("want %v from LateUpdates, but timed out after %v",
				wantLate, lateTimeout)
		}
	}

	test("ls",
		styled.Plain("ls"),
		styled.MakeText("ls", "green"))
	test("echo",
		styled.Plain("echo"),
		styled.MakeText("echo", "red"))
}

const (
	// The delay to deliver the result for the first highlight after the second
	// highlight has been requested.
	hlDelay = 10 * time.Millisecond
	// The duration to wait to make sure that the first highlight has completed
	// and there is nothing delivered on LateUpdates. The test will wait this
	// long to make sure that the late update is dropped, so it shouldn't be too
	// large.
	hlWait = 50 * time.Millisecond
)

func TestHighlighter_HasCommand_LateResultOutOfOrder(t *testing.T) {
	// When late results are delivered out of order, the ones that do not match
	// the current code are dropped. In this test, hl.Get is called with "l"
	// first and then "ls". The late result for "l" is delivered after that of
	// "ls" and is dropped.

	hlSecond := make(chan struct{})
	hl := NewHighlighter(Config{
		HasCommand: func(cmd string) bool {
			if cmd == "l" {
				// Make sure that the second highlight has been requested before
				// returning.
				<-hlSecond
				time.Sleep(hlDelay)
				return false
			}
			close(hlSecond)
			return cmd == "ls"
		}})

	hl.Get("l")

	initial, _ := hl.Get("ls")
	late := <-hl.LateUpdates()

	wantInitial := styled.Plain("ls")
	wantLate := styled.MakeText("ls", "green")
	if !reflect.DeepEqual(wantInitial, initial) {
		t.Errorf("want %v from initial Get, got %v", wantInitial, initial)
	}
	if !reflect.DeepEqual(wantLate, late) {
		t.Errorf("want %v from late Get, got %v", wantLate, late)
	}

	// Make sure that no more late updates are delivered.
	select {
	case late := <-hl.LateUpdates():
		t.Errorf("want nothing from LateUpdates, got %v", late)
	case <-time.After(hlWait):
		// No late updates; test passed.
	}
}

// Matchers.

type anyMatcher struct{}

func (anyMatcher) Match(tt.RetValue) bool { return true }

type errorsMatcher struct{ matchers []tt.Matcher }

func (m errorsMatcher) Match(v tt.RetValue) bool {
	errs := v.([]error)
	if len(errs) != len(m.matchers) {
		return false
	}
	for i, matcher := range m.matchers {
		if !matcher.Match(errs[i]) {
			return false
		}
	}
	return true
}

func matchErrors(m ...tt.Matcher) errorsMatcher { return errorsMatcher{m} }

type parseErrorMatcher struct{ begin, end int }

func (m parseErrorMatcher) Match(v tt.RetValue) bool {
	err := v.(*parse.Error)
	return m.begin == err.Context.Begin && m.end == err.Context.End
}

// Fake check error, used in tests for check callback.
type fakeCheckError struct{ from, to int }

func (e fakeCheckError) Range() diag.Ranging { return diag.Ranging{e.from, e.to} }
func (fakeCheckError) Error() string         { return "fake check error" }
