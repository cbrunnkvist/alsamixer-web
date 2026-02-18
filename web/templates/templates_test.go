package templates

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

const controlsTemplatePath = "controls.html"

// ControlView represents the data needed to render a single mixer control
// in the controls.html template. It mirrors the fields referenced in the
// template so tests can verify execution without depending on the final
// view-model implementation.
type ControlView struct {
	ID          string
	Name        string
	Description string
	CardID      uint

	HasVolume       bool
	VolumeAriaLabel string
	VolumeMin       int
	VolumeMax       int
	VolumeNow       int
	VolumeText      string

	HasMute       bool
	MuteAriaLabel string
	Muted         bool

	HasCapture       bool
	CaptureAriaLabel string
	CaptureActive    bool
}

// CardView represents a sound card and its controls for rendering.
type CardView struct {
	ID          uint
	Name        string
	Description string
	Controls    []ControlView
}

// ControlsPage is the top-level data structure passed into the
// "controls" template.
type ControlsPage struct {
	Cards []CardView
}

func TestControlsTemplateParses(t *testing.T) {
	tmpl, err := template.ParseFiles(controlsTemplatePath)
	if err != nil {
		t.Fatalf("failed to parse controls template: %v", err)
	}

	if tmpl.Lookup("controls") == nil {
		t.Fatalf("expected template 'controls' to be defined")
	}
	if tmpl.Lookup("control") == nil {
		t.Fatalf("expected template 'control' to be defined")
	}
}

func TestControlsTemplateExecutesWithMockData(t *testing.T) {
	tmpl, err := template.ParseFiles(controlsTemplatePath)
	if err != nil {
		t.Fatalf("failed to parse controls template: %v", err)
	}

	page := ControlsPage{
		Cards: []CardView{
			{
				ID:          0,
				Name:        "Test Card",
				Description: "Primary sound card for testing",
				Controls: []ControlView{
					{
						ID:               "master",
						Name:             "Master",
						Description:      "Master playback volume",
						CardID:           0,
						HasVolume:        true,
						VolumeAriaLabel:  "Master volume",
						VolumeMin:        0,
						VolumeMax:        100,
						VolumeNow:        75,
						VolumeText:       "75%",
						HasMute:          true,
						MuteAriaLabel:    "Mute Master",
						Muted:            false,
						HasCapture:       true,
						CaptureAriaLabel: "Capture Master",
						CaptureActive:    true,
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "controls", page); err != nil {
		t.Fatalf("failed to execute controls template: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatalf("expected non-empty rendered output")
	}

	// Basic smoke checks for required ARIA attributes and roles
	checks := []string{
		"role=\"main\"",
		"role=\"slider\"",
		"role=\"switch\"",
		"aria-valuemin=\"0\"",
		"aria-valuemax=\"100\"",
		"aria-valuenow=\"75\"",
		"aria-valuetext=\"75%\"",
		"aria-live=\"polite\"",
		"class=\"sr-only\"",
		"aria-labelledby=\"card-0\"",
	}

	for _, token := range checks {
		if !strings.Contains(out, token) {
			t.Fatalf("rendered template missing expected token %q. Output: %s", token, out)
		}
	}
}
