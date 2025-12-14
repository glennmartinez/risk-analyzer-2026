package services

import (
	"risk-analyzer/internal/models"
	"sort"
	"strings"
	"unicode"

	"github.com/jdkato/prose/v2"
)

// KeywordExtractor handles keyword extraction from text
type KeywordExtractor struct {
	// Common stop words to filter out
	stopWords map[string]bool
	// Minimum keyword length
	minLength int
}

// NewKeywordExtractor creates a new keyword extractor
func NewKeywordExtractor() *KeywordExtractor {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "could": true, "should": true,
		"this": true, "that": true, "these": true, "those": true, "i": true, "you": true,
		"he": true, "she": true, "it": true, "we": true, "they": true, "my": true,
		"your": true, "his": true, "her": true, "its": true, "our": true, "their": true,
	}

	return &KeywordExtractor{
		stopWords: stopWords,
		minLength: 2,
	}
}

// KeywordResult represents a keyword with its frequency and importance
type KeywordResult struct {
	Word      string  `json:"word"`
	Frequency int     `json:"frequency"`
	Score     float64 `json:"score"`
	PosTag    string  `json:"pos_tag"`
}

// ExtractKeywords extracts keywords from title and description
func (ke *KeywordExtractor) ExtractKeywords(issue models.Issue) ([]KeywordResult, error) {
	// Combine title and description, giving title more weight
	text := strings.Repeat(issue.Title+" ", 2) + " " + issue.Description

	// Create a new document for NLP processing
	doc, err := prose.NewDocument(text)
	if err != nil {
		return nil, err
	}

	wordFreq := make(map[string]*KeywordResult)

	// Extract tokens with POS tags
	for _, tok := range doc.Tokens() {
		word := strings.ToLower(tok.Text)

		// Filter out unwanted words
		if ke.shouldSkipWord(word, tok.Tag) {
			continue
		}

		// Calculate importance score based on POS tag
		score := ke.calculateScore(tok.Tag)

		if existing, exists := wordFreq[word]; exists {
			existing.Frequency++
			existing.Score += score
		} else {
			wordFreq[word] = &KeywordResult{
				Word:      word,
				Frequency: 1,
				Score:     score,
				PosTag:    tok.Tag,
			}
		}
	}

	// Extract named entities (they get higher scores)
	for _, ent := range doc.Entities() {
		word := strings.ToLower(ent.Text)
		if len(word) >= ke.minLength && !ke.stopWords[word] {
			if existing, exists := wordFreq[word]; exists {
				existing.Score += 2.0 // Boost named entities
			} else {
				wordFreq[word] = &KeywordResult{
					Word:      word,
					Frequency: 1,
					Score:     2.0,
					PosTag:    "NE_" + ent.Label,
				}
			}
		}
	}

	// Convert to slice and sort by score
	var keywords []KeywordResult
	for _, result := range wordFreq {
		// Final score calculation (frequency * base score)
		result.Score = result.Score * float64(result.Frequency)
		keywords = append(keywords, *result)
	}

	// Sort by score (highest first)
	sort.Slice(keywords, func(i, j int) bool {
		return keywords[i].Score > keywords[j].Score
	})

	return keywords, nil
}

// shouldSkipWord determines if a word should be filtered out
func (ke *KeywordExtractor) shouldSkipWord(word, posTag string) bool {
	// Skip if too short
	if len(word) < ke.minLength {
		return true
	}

	// Skip stop words
	if ke.stopWords[word] {
		return true
	}

	// Skip pure numbers or punctuation
	if ke.isPureNumber(word) || ke.isPunctuation(word) {
		return true
	}

	// Skip certain POS tags (determiners, prepositions, etc.)
	skipTags := map[string]bool{
		"DT":   true, // determiner
		"IN":   true, // preposition
		"TO":   true, // to
		"CC":   true, // coordinating conjunction
		"PRP":  true, // personal pronoun
		"PRP$": true, // possessive pronoun
		"WP":   true, // wh-pronoun
		"WDT":  true, // wh-determiner
	}

	return skipTags[posTag]
}

// calculateScore assigns importance based on POS tag
func (ke *KeywordExtractor) calculateScore(posTag string) float64 {
	scores := map[string]float64{
		"NN":   1.5, // noun
		"NNS":  1.5, // plural noun
		"NNP":  2.0, // proper noun
		"NNPS": 2.0, // plural proper noun
		"VB":   1.2, // verb
		"VBD":  1.2, // past tense verb
		"VBG":  1.2, // gerund/present participle
		"VBN":  1.2, // past participle
		"VBP":  1.2, // present tense verb
		"VBZ":  1.2, // 3rd person singular present verb
		"JJ":   1.3, // adjective
		"JJR":  1.3, // comparative adjective
		"JJS":  1.3, // superlative adjective
		"RB":   0.8, // adverb
		"RBR":  0.8, // comparative adverb
		"RBS":  0.8, // superlative adverb
	}

	if score, exists := scores[posTag]; exists {
		return score
	}
	return 1.0 // default score
}

// isPureNumber checks if string contains only digits
func (ke *KeywordExtractor) isPureNumber(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return len(s) > 0
}

// isPunctuation checks if string contains only punctuation
func (ke *KeywordExtractor) isPunctuation(s string) bool {
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) {
			return false
		}
	}
	return len(s) > 0
}

// ExtractTopKeywords returns the top N keywords
func (ke *KeywordExtractor) ExtractTopKeywords(issue models.Issue, limit int) ([]KeywordResult, error) {
	keywords, err := ke.ExtractKeywords(issue)
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(keywords) > limit {
		return keywords[:limit], nil
	}

	return keywords, nil
}

// ExtractKeywordStrings returns just the keyword strings (simplified version)
func (ke *KeywordExtractor) ExtractKeywordStrings(issue models.Issue, limit int) ([]string, error) {
	keywords, err := ke.ExtractTopKeywords(issue, limit)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(keywords))
	for i, kw := range keywords {
		result[i] = kw.Word
	}

	return result, nil
}
