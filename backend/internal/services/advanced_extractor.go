package services

import (
	"regexp"
	"risk-analyzer/internal/models"
	"sort"
	"strings"
)

// AdvancedKeywordExtractor combines multiple techniques
type AdvancedKeywordExtractor struct {
	basicExtractor *KeywordExtractor
	domainKeywords map[string]float64 // Domain-specific keywords with weights
	stopWords      map[string]bool
	stemmer        *PorterStemmer
}

// NewAdvancedKeywordExtractor creates an enhanced extractor
func NewAdvancedKeywordExtractor() *AdvancedKeywordExtractor {
	// Domain-specific keywords for software issues
	domainKeywords := map[string]float64{
		"error": 2.0, "exception": 2.0, "fail": 2.0, "bug": 2.5,
		"timeout": 2.0, "crash": 2.5, "performance": 1.8,
		"memory": 1.8, "cpu": 1.8, "database": 2.0, "api": 1.8,
		"server": 1.8, "network": 1.8, "security": 2.2,
		"authentication": 2.0, "authorization": 2.0,
		"deployment": 1.8, "build": 1.5, "pipeline": 1.8,
		"jenkins": 2.5, "docker": 2.0, "kubernetes": 2.0,
	}

	return &AdvancedKeywordExtractor{
		basicExtractor: NewKeywordExtractor(),
		domainKeywords: domainKeywords,
		stopWords:      make(map[string]bool),
		stemmer:        NewPorterStemmer(),
	}
}

// ExtractAdvancedKeywords uses multiple techniques
func (ake *AdvancedKeywordExtractor) ExtractAdvancedKeywords(issue models.Issue) ([]KeywordResult, error) {
	// 1. Basic prose extraction
	basicKeywords, err := ake.basicExtractor.ExtractKeywords(issue)
	if err != nil {
		return nil, err
	}

	// 2. Add domain-specific scoring
	ake.enhanceWithDomainKnowledge(basicKeywords)

	// 3. Extract technical patterns (version numbers, error codes, etc.)
	technicalTerms := ake.extractTechnicalPatterns(issue.Title + " " + issue.Description)

	// 4. Merge and re-score
	merged := ake.mergeKeywords(basicKeywords, technicalTerms)

	// 5. Apply stemming and grouping
	grouped := ake.groupSimilarTerms(merged)

	// Sort by final score
	sort.Slice(grouped, func(i, j int) bool {
		return grouped[i].Score > grouped[j].Score
	})

	return grouped, nil
}

// enhanceWithDomainKnowledge boosts scores for domain-specific terms
func (ake *AdvancedKeywordExtractor) enhanceWithDomainKnowledge(keywords []KeywordResult) {
	for i := range keywords {
		if boost, exists := ake.domainKeywords[keywords[i].Word]; exists {
			keywords[i].Score *= boost
			keywords[i].PosTag += "_DOMAIN"
		}
	}
}

// extractTechnicalPatterns finds technical terms using regex
func (ake *AdvancedKeywordExtractor) extractTechnicalPatterns(text string) []KeywordResult {
	var results []KeywordResult
	text = strings.ToLower(text)

	patterns := map[string]*regexp.Regexp{
		"version":    regexp.MustCompile(`v?\d+\.\d+(\.\d+)?`),
		"error_code": regexp.MustCompile(`[a-z]+\d{3,}`),
		"url":        regexp.MustCompile(`https?://[^\s]+`),
		"file_ext":   regexp.MustCompile(`\.[a-z]{2,4}\b`),
		"port":       regexp.MustCompile(`:\d{2,5}\b`),
	}

	for category, pattern := range patterns {
		matches := pattern.FindAllString(text, -1)
		for _, match := range matches {
			results = append(results, KeywordResult{
				Word:      match,
				Frequency: 1,
				Score:     2.0, // Technical terms get high scores
				PosTag:    "TECHNICAL_" + strings.ToUpper(category),
			})
		}
	}

	return results
}

// mergeKeywords combines different keyword sources
func (ake *AdvancedKeywordExtractor) mergeKeywords(basic, technical []KeywordResult) []KeywordResult {
	wordMap := make(map[string]*KeywordResult)

	// Add basic keywords
	for _, kw := range basic {
		wordMap[kw.Word] = &kw
	}

	// Merge technical keywords
	for _, kw := range technical {
		if existing, exists := wordMap[kw.Word]; exists {
			existing.Score += kw.Score
			existing.Frequency++
		} else {
			wordMap[kw.Word] = &kw
		}
	}

	var result []KeywordResult
	for _, kw := range wordMap {
		result = append(result, *kw)
	}

	return result
}

// groupSimilarTerms uses stemming to group similar words
func (ake *AdvancedKeywordExtractor) groupSimilarTerms(keywords []KeywordResult) []KeywordResult {
	stemGroups := make(map[string]*KeywordResult)

	for _, kw := range keywords {
		stem := ake.stemmer.Stem(kw.Word)

		if existing, exists := stemGroups[stem]; exists {
			// Keep the shorter/more common word form
			if len(kw.Word) < len(existing.Word) || kw.Frequency > existing.Frequency {
				existing.Word = kw.Word
			}
			existing.Score += kw.Score * 0.5 // Reduce score for grouped terms
			existing.Frequency += kw.Frequency
		} else {
			stemGroups[stem] = &kw
		}
	}

	var result []KeywordResult
	for _, kw := range stemGroups {
		result = append(result, *kw)
	}

	return result
}

// Simple Porter Stemmer implementation
type PorterStemmer struct{}

func NewPorterStemmer() *PorterStemmer {
	return &PorterStemmer{}
}

func (ps *PorterStemmer) Stem(word string) string {
	// Simplified stemming rules
	word = strings.ToLower(word)

	// Remove common suffixes
	suffixes := []string{"ing", "ed", "er", "est", "ly", "tion", "sion"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(word, suffix) && len(word) > len(suffix)+2 {
			return word[:len(word)-len(suffix)]
		}
	}

	return word
}
