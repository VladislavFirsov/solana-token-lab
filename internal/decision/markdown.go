package decision

import (
	"fmt"
	"strings"
)

// RenderMarkdown renders DecisionResult as Markdown string.
func RenderMarkdown(result *DecisionResult) string {
	var sb strings.Builder

	// Decision header
	sb.WriteString("# Decision Gate Report\n\n")
	sb.WriteString(fmt.Sprintf("## Decision: %s\n\n", result.Decision))

	// GO Criteria table
	sb.WriteString("## GO Criteria\n\n")
	sb.WriteString("| # | Criterion | Threshold | Actual | Pass |\n")
	sb.WriteString("|---|-----------|-----------|--------|------|\n")
	for i, c := range result.GOCriteria {
		passStr := "PASS"
		if !c.Pass {
			passStr = "FAIL"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
			i+1, c.Name, c.Threshold, c.Actual, passStr))
	}
	sb.WriteString("\n")

	// Count GO passes
	goPassed := 0
	for _, c := range result.GOCriteria {
		if c.Pass {
			goPassed++
		}
	}
	sb.WriteString(fmt.Sprintf("GO Criteria: %d/%d passed\n\n", goPassed, len(result.GOCriteria)))

	// NO-GO Triggers table
	sb.WriteString("## NO-GO Triggers\n\n")
	sb.WriteString("| # | Trigger | Condition | Actual | Status |\n")
	sb.WriteString("|---|---------|-----------|--------|--------|\n")
	for i, c := range result.NOGOChecks {
		statusStr := "NOT TRIGGERED"
		if !c.Pass { // Pass=false means triggered
			statusStr = "TRIGGERED"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
			i+1, c.Name, c.Threshold, c.Actual, statusStr))
	}
	sb.WriteString("\n")

	// Count NO-GO triggers
	nogoTriggered := 0
	for _, c := range result.NOGOChecks {
		if !c.Pass {
			nogoTriggered++
		}
	}
	sb.WriteString(fmt.Sprintf("NO-GO Triggers: %d/%d triggered\n\n", nogoTriggered, len(result.NOGOChecks)))

	// Summary
	sb.WriteString("## Summary\n\n")
	if result.Decision == DecisionGO {
		sb.WriteString("All GO criteria passed and no NO-GO triggers fired.\n")
	} else {
		sb.WriteString("Decision is NO-GO due to:\n")
		for _, c := range result.GOCriteria {
			if !c.Pass {
				sb.WriteString(fmt.Sprintf("- GO criterion failed: %s (actual: %s)\n", c.Name, c.Actual))
			}
		}
		for _, c := range result.NOGOChecks {
			if !c.Pass {
				sb.WriteString(fmt.Sprintf("- NO-GO trigger fired: %s (actual: %s)\n", c.Name, c.Actual))
			}
		}
	}

	return sb.String()
}
