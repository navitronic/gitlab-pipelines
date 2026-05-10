package tui

import "github.com/charmbracelet/lipgloss"

var (
	listTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginLeft(2).MarginTop(1)
	itemBaseStyle  = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2).
			PaddingTop(1).
			PaddingBottom(1)
	itemSelectedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				PaddingRight(2).
				PaddingTop(1).
				PaddingBottom(1).
				Background(lipgloss.Color("238")).
				Foreground(lipgloss.Color("229"))
	repoItemStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(1)
	repoSelectedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				PaddingRight(1).
				Background(lipgloss.Color("238")).
				Foreground(lipgloss.Color("229"))
	toastStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("237")).
			PaddingLeft(1).
			PaddingRight(1)
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("236")).
			PaddingLeft(1).
			PaddingRight(1)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginLeft(2)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).MarginLeft(2)
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	runningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	pendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	canceledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	skippedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	detailTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginLeft(2).MarginTop(1)
	jobHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginLeft(2)
	stageStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	runningBoldStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231"))
)
