package agent

import "fmt"

// NewAllComplete builds a completion message for the TUI.
func NewAllComplete(conclusion, projectDir string) AllCompleteMsg {
	return AllCompleteMsg{Summary: conclusion, ProjectDir: projectDir}
}

// NewAllCompleteWithAcceptance builds completion with collapsible acceptance detail in the TUI.
func NewAllCompleteWithAcceptance(projectDir, acceptanceDetail string, passed bool, passedN, totalN int) AllCompleteMsg {
	summary := ""
	if totalN > 0 {
		if passed {
			summary = fmt.Sprintf("验收通过 %d/%d", passedN, totalN)
		} else {
			summary = fmt.Sprintf("验收未通过 %d/%d", passedN, totalN)
		}
	}
	return AllCompleteMsg{
		Summary:           summary,
		ProjectDir:        projectDir,
		AcceptanceDetail:  acceptanceDetail,
		AcceptancePassed:  passed,
		AcceptancePassedN: passedN,
		AcceptanceTotalN:  totalN,
	}
}
