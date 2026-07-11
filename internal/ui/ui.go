// Package ui defines DEM's visual identity in the terminal.
//
// All styled output goes through here, so aesthetics stay consistent
// and the --plain mode (or NO_COLOR, or non-TTY output) degrades to
// plain text in a single place.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// DEM's palette. Adaptive colors: one shade for light backgrounds,
// another for dark ones.
var (
	colorPrimary = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"} // purple
	colorSuccess = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
	colorWarn    = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}
	colorErr     = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	colorDim     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
)

var (
	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	styleSuccess = lipgloss.NewStyle().Foreground(colorSuccess)
	styleWarn    = lipgloss.NewStyle().Foreground(colorWarn)
	styleErr     = lipgloss.NewStyle().Bold(true).Foreground(colorErr)
	styleDim     = lipgloss.NewStyle().Foreground(colorDim)
	styleBadge   = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
)

// SetPlain forces plain text (--plain flag). NO_COLOR and the
// absence of a TTY are already honored by lipgloss automatically.
func SetPlain() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

// Title renders a section header.
func Title(s string) string { return styleTitle.Render(s) }

// Success prefixes with ✓; Warn with !; Error with ✗.
func Success(s string) string { return styleSuccess.Render("✓ ") + s }
func Warn(s string) string    { return styleWarn.Render("! ") + s }
func Error(s string) string   { return styleErr.Render("✗ ") + s }

// Dim renders secondary text (paths, hints, metadata).
func Dim(s string) string { return styleDim.Render(s) }

// Badge highlights an inline identifier, e.g. name@version.
func Badge(s string) string { return styleBadge.Render(s) }

// KeyValues aligns label/value pairs in columns:
//
//	node    22.5.1   (global)
//	java    21.0.4   (dem.yaml)
func KeyValues(rows [][2]string) string {
	width := 0
	for _, r := range rows {
		if len(r[0]) > width {
			width = len(r[0])
		}
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%-*s  %s\n", width, r[0], r[1])
	}
	return strings.TrimRight(b.String(), "\n")
}
