package tui

import (
	"testing"
)

func TestTitleStyleIsNonNil(t *testing.T) {
	if TitleStyle.GetBold() != true {
		t.Error("TitleStyle should be bold")
	}
}

func TestActiveStyleIsNonNil(t *testing.T) {
	if ActiveStyle.GetForeground() == nil {
		t.Error("ActiveStyle should have a foreground color")
	}
}

func TestInactiveStyleIsNonNil(t *testing.T) {
	if InactiveStyle.GetForeground() == nil {
		t.Error("InactiveStyle should have a foreground color")
	}
}

func TestBorderStyleIsNonNil(t *testing.T) {
	rendered := BorderStyle.Render("test")
	if rendered == "test" {
		t.Error("BorderStyle should render with border decoration")
	}
}
