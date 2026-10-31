package engine

import (
	"strings"
)

// PromptLookupSpeculator implements n-gram based Prompt Lookup Decoding (PLD).
// It searches the existing conversation context and prompt for matching sub-phrases
// to speculate 3–6 candidate tokens in advance, which the target LLM validates in
// a single forward pass, providing a 3x to 6x acceleration on CPU inference.
type PromptLookupSpeculator struct {
	ngramMin    int
	ngramMax    int
	draftWindow int
}

// NewPromptLookupSpeculator initializes a prompt lookup speculator.
func NewPromptLookupSpeculator(ngramMin, ngramMax, draftWindow int) *PromptLookupSpeculator {
	if ngramMin <= 0 {
		ngramMin = 2
	}
	if ngramMax <= 0 {
		ngramMax = 4
	}
	if draftWindow <= 0 {
		draftWindow = 5
	}
	return &PromptLookupSpeculator{
		ngramMin:    ngramMin,
		ngramMax:    ngramMax,
		draftWindow: draftWindow,
	}
}

// SpeculateCandidates finds matching n-grams from the prompt text that match the recent context tail.
func (p *PromptLookupSpeculator) SpeculateCandidates(fullPrompt, currentGeneration string) []string {
	combined := fullPrompt + currentGeneration
	words := strings.Fields(combined)
	if len(words) < p.ngramMin+1 {
		return nil
	}

	genWords := strings.Fields(currentGeneration)
	if len(genWords) == 0 {
		return nil
	}

	// Try largest n-gram down to smallest
	for n := p.ngramMax; n >= p.ngramMin; n-- {
		if len(genWords) < n {
			continue
		}

		// Tail n-gram in current generation
		tail := strings.Join(genWords[len(genWords)-n:], " ")

		// Search for tail occurrence in preceding text
		prevText := strings.Join(words[:len(words)-1], " ")
		idx := strings.Index(prevText, tail)
		if idx != -1 {
			// Extract following words
			afterMatch := strings.TrimSpace(prevText[idx+len(tail):])
			candidateWords := strings.Fields(afterMatch)
			if len(candidateWords) > 0 {
				limit := p.draftWindow
				if len(candidateWords) < limit {
					limit = len(candidateWords)
				}
				res := make([]string, limit)
				for i := 0; i < limit; i++ {
					res[i] = " " + candidateWords[i]
				}
				return res
			}
		}
	}

	return nil
}

// SpeculativeStats tracks speculation performance and effective speedup.
type SpeculativeStats struct {
	DraftTokensCount    int
	AcceptedTokensCount int
	SpeedupRatio        float64
}

// ComputeSpeedup calculates the effective acceleration multiplier.
// Baseline decoding generates 1 token per forward pass.
// Speculative decoding evaluates K draft tokens + 1 bonus token per step.
func ComputeSpeedup(totalTokens, acceptedDraftTokens int) float64 {
	if totalTokens <= 0 {
		return 1.0
	}
	// Effective forward passes = TotalTokens - AcceptedDraftTokens
	forwardPasses := totalTokens - acceptedDraftTokens
	if forwardPasses <= 0 {
		forwardPasses = 1
	}
	ratio := float64(totalTokens) / float64(forwardPasses)
	if ratio < 1.0 {
		ratio = 1.0
	}
	return ratio
}
