package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelUpdateNavigatesBetweenScreens(t *testing.T) {
	model := New(OverviewData{}, true)
	next, command := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	updated := next.(Model)
	if command != nil || updated.screen != projectsScreen {
		t.Fatalf("right update = screen %v, command %v", updated.screen, command)
	}

	next, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if got := next.(Model).screen; got != overviewScreen {
		t.Fatalf("escape screen = %v, want overview", got)
	}
}

func TestModelUpdateTogglesHelpAndQuits(t *testing.T) {
	model := New(OverviewData{}, true)
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !next.(Model).help {
		t.Fatal("help was not enabled")
	}

	_, command := next.(Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if command == nil {
		t.Fatal("ctrl+c did not return quit command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command result = %T, want tea.QuitMsg", command())
	}
}

func TestModelUpdateAppliesResponsiveWarnings(t *testing.T) {
	model := New(OverviewData{}, true)
	next, _ := model.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
	if view := next.(Model).View(); !strings.Contains(view, "Compact layout") {
		t.Fatalf("compact view = %q", view)
	}
	next, _ = next.(Model).Update(tea.WindowSizeMsg{Width: 59, Height: 24})
	if view := next.(Model).View(); !strings.Contains(view, "Terminal too narrow") {
		t.Fatalf("narrow view = %q", view)
	}
}

func TestNoColorViewContainsNoANSISequences(t *testing.T) {
	model := New(OverviewData{}, true)
	if view := model.View(); strings.Contains(view, "\x1b[") {
		t.Fatalf("NO_COLOR view contains ANSI sequence: %q", view)
	}
}
