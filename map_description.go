package main

// describeFile is Roshan's paired-programming exercise. The surrounding code
// supplies evidence-only facts; replace this neutral fallback with one short,
// factual sentence without guessing intent that the facts cannot support.
func describeFile(facts fileDescriptionFacts) string {
	if facts.Language == "" {
		return "Source file."
	}
	return facts.Language + " source file."
}
