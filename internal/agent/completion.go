package agent

// NewAllComplete builds a completion message for the TUI.
func NewAllComplete(conclusion, projectDir string) AllCompleteMsg {
	return AllCompleteMsg{Summary: conclusion, ProjectDir: projectDir}
}

// NewAllCompleteWithAcceptance builds completion with full acceptance detail and LLM summary.
func NewAllCompleteWithAcceptance(conclusion, projectDir, acceptanceDetail string, passed bool, passedN, totalN int) AllCompleteMsg {
	return AllCompleteMsg{
		Summary:           conclusion,
		ProjectDir:        projectDir,
		AcceptanceDetail:  acceptanceDetail,
		AcceptancePassed:  passed,
		AcceptancePassedN: passedN,
		AcceptanceTotalN:  totalN,
	}
}
