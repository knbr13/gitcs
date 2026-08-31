package main

import (
	"fmt"
	"strings"
)

type changeSummaryFacts struct {
	Label             string
	Language          string
	Status            changeStatus
	Additions         int
	Deletions         int
	FirstChangedLine  int
	TouchedSymbols    []string
	PreviousSymbols   []string
	CurrentSymbols    []string
	PreviousLineCount int
	CurrentLineCount  int
	IncomingCount     int
	OutgoingCount     int
	Neighbors         []string
}

func describeFile(facts fileDescriptionFacts) string {
	language := facts.Language
	if language == "" {
		language = "Source"
	}

	parts := []string{fmt.Sprintf("%s source file", language)}
	if len(facts.Symbols) > 0 {
		parts = append(parts, "defining "+limitedList(facts.Symbols, 4))
	}
	if facts.IsRoot {
		parts = append(parts, "marked as a root file")
	}
	if facts.IncomingCount > 0 || facts.OutgoingCount > 0 {
		parts = append(parts, fmt.Sprintf("connected by %d incoming and %d outgoing code links", facts.IncomingCount, facts.OutgoingCount))
	}
	if facts.ChangeStatus != "" {
		change := fmt.Sprintf("%s with +%d/-%d", changeStatusWord(facts.ChangeStatus), facts.Additions, facts.Deletions)
		if len(facts.TouchedSymbols) > 0 {
			change += " touching " + limitedList(facts.TouchedSymbols, 3)
		}
		parts = append(parts, change)
	}

	return sentence(strings.Join(parts, "; "))
}

func summarizeChange(facts changeSummaryFacts) changeSummary {
	return changeSummary{
		Previous: previousSummary(facts),
		Current:  currentSummary(facts),
		Changed:  changedSummary(facts),
		Impact:   impactSummary(facts),
	}
}

func previousSummary(facts changeSummaryFacts) string {
	if facts.Status == changeAdded {
		return "This file did not exist at HEAD."
	}
	return stateSummary("Previously", facts.Language, facts.PreviousLineCount, facts.PreviousSymbols)
}

func currentSummary(facts changeSummaryFacts) string {
	if facts.Status == changeDeleted {
		return "The file is removed from the working tree."
	}
	return stateSummary("Now", facts.Language, facts.CurrentLineCount, facts.CurrentSymbols)
}

func changedSummary(facts changeSummaryFacts) string {
	switch facts.Status {
	case changeAdded:
		return sentence(fmt.Sprintf(
			"Added %s with %d lines and %s",
			labelOrFile(facts.Label),
			facts.CurrentLineCount,
			symbolPhrase(facts.CurrentSymbols, "no detected top-level Go symbols"),
		))
	case changeDeleted:
		return sentence(fmt.Sprintf(
			"Deleted %s that previously had %d lines and %s",
			labelOrFile(facts.Label),
			facts.PreviousLineCount,
			symbolPhrase(facts.PreviousSymbols, "no detected top-level Go symbols"),
		))
	}

	parts := []string{fmt.Sprintf(
		"%s starting near line %d with +%d/-%d",
		changeStatusWord(facts.Status),
		max(1, facts.FirstChangedLine),
		facts.Additions,
		facts.Deletions,
	)}
	if len(facts.TouchedSymbols) > 0 {
		parts = append(parts, "touching "+limitedList(facts.TouchedSymbols, 4))
	}
	if added := difference(facts.CurrentSymbols, facts.PreviousSymbols); len(added) > 0 {
		parts = append(parts, "adding "+limitedList(added, 3))
	}
	if removed := difference(facts.PreviousSymbols, facts.CurrentSymbols); len(removed) > 0 {
		parts = append(parts, "removing "+limitedList(removed, 3))
	}
	return sentence(strings.Join(parts, "; "))
}

func impactSummary(facts changeSummaryFacts) string {
	if facts.IncomingCount == 0 && facts.OutgoingCount == 0 {
		return "No direct code connections were detected for this file."
	}

	impact := fmt.Sprintf(
		"This file has %d incoming and %d outgoing code links",
		facts.IncomingCount,
		facts.OutgoingCount,
	)
	if len(facts.Neighbors) > 0 {
		impact += ", including " + limitedList(facts.Neighbors, 4)
	}
	return sentence(impact)
}

func stateSummary(prefix, language string, lineCount int, symbols []string) string {
	if language == "" {
		language = "source"
	}
	lineVerb := "had"
	defineVerb := "defined"
	if strings.EqualFold(prefix, "Now") {
		lineVerb = "has"
		defineVerb = "defines"
	}
	if len(symbols) > 0 {
		return sentence(fmt.Sprintf("%s this %s file %s %d lines and %s %s", prefix, language, lineVerb, lineCount, defineVerb, limitedList(symbols, 5)))
	}
	return sentence(fmt.Sprintf("%s this %s file %s %d lines and no detected top-level Go symbols", prefix, language, lineVerb, lineCount))
}

func contentLineCount(content string) int {
	if strings.TrimSpace(content) == "" {
		return 0
	}
	return countDiffLines(content)
}

func changeStatusWord(status changeStatus) string {
	switch status {
	case changeAdded:
		return "Added"
	case changeModified:
		return "Modified"
	case changeDeleted:
		return "Deleted"
	case changeRenamed:
		return "Renamed"
	default:
		return "Changed"
	}
}

func symbolPhrase(symbols []string, empty string) string {
	if len(symbols) == 0 {
		return empty
	}
	return "defines " + limitedList(symbols, 5)
}

func labelOrFile(label string) string {
	if label == "" {
		return "the file"
	}
	return label
}

func limitedList(values []string, limit int) string {
	values = uniqueSortedStrings(values)
	if len(values) == 0 {
		return ""
	}
	if limit <= 0 || len(values) <= limit {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:limit], ", ") + fmt.Sprintf(", and %d more", len(values)-limit)
}

func difference(left, right []string) []string {
	rightSet := make(map[string]bool, len(right))
	for _, value := range right {
		rightSet[value] = true
	}
	var result []string
	for _, value := range left {
		if value != "" && !rightSet[value] {
			result = append(result, value)
		}
	}
	return uniqueSortedStrings(result)
}

func sentence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "No local evidence available."
	}
	if strings.HasSuffix(value, ".") {
		return value
	}
	return value + "."
}
