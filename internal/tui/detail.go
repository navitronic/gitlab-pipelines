package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DetailModel renders the detail view for a selected pipeline.
type DetailModel struct {
	row PipelineRow
}

// NewDetailModel creates a detail model for the given pipeline row.
func NewDetailModel(row PipelineRow) *DetailModel {
	return &DetailModel{row: row}
}

func (d *DetailModel) View() string {
	p := d.row.Pipeline

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Pipeline #%d", p.ID)))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Project:  %s\n", d.row.ProjectPath))
	b.WriteString(fmt.Sprintf("  Status:   %s\n", statusIcon(p.Status)))
	b.WriteString(fmt.Sprintf("  Ref:      %s\n", p.Ref))
	b.WriteString(fmt.Sprintf("  SHA:      %s\n", p.SHA))
	b.WriteString(fmt.Sprintf("  Source:   %s\n", p.Source))
	b.WriteString(fmt.Sprintf("  Updated:  %s\n", formatTime(p.UpdatedAt)))
	if p.WebURL != "" {
		b.WriteString(fmt.Sprintf("  URL:      %s\n", p.WebURL))
	}
	b.WriteString("\n")
	b.WriteString(detailHelpStyle.Render("esc/backspace: back • q: quit"))
	return b.String()
}

var detailHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginLeft(2).MarginTop(1)
