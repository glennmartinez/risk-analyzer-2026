package services

import (
	"math"
	"strings"
)

// TFIDFExtractor implements TF-IDF keyword extraction
type TFIDFExtractor struct {
	documents  []string
	vocabulary map[string]int
	idf        map[string]float64
}

// NewTFIDFExtractor creates a new TF-IDF extractor
func NewTFIDFExtractor(documents []string) *TFIDFExtractor {
	extractor := &TFIDFExtractor{
		documents:  documents,
		vocabulary: make(map[string]int),
		idf:        make(map[string]float64),
	}
	extractor.buildVocabulary()
	extractor.calculateIDF()
	return extractor
}

// buildVocabulary creates the vocabulary from all documents
func (tfidf *TFIDFExtractor) buildVocabulary() {
	for _, doc := range tfidf.documents {
		words := strings.Fields(strings.ToLower(doc))
		for _, word := range words {
			tfidf.vocabulary[word]++
		}
	}
}

// calculateIDF calculates inverse document frequency for each term
func (tfidf *TFIDFExtractor) calculateIDF() {
	totalDocs := float64(len(tfidf.documents))

	for word := range tfidf.vocabulary {
		docCount := 0.0
		for _, doc := range tfidf.documents {
			if strings.Contains(strings.ToLower(doc), word) {
				docCount++
			}
		}
		tfidf.idf[word] = math.Log(totalDocs / docCount)
	}
}

// ExtractKeywords returns TF-IDF scores for a document
func (tfidf *TFIDFExtractor) ExtractKeywords(document string, limit int) []KeywordResult {
	words := strings.Fields(strings.ToLower(document))
	wordCount := make(map[string]int)

	// Calculate term frequency
	for _, word := range words {
		wordCount[word]++
	}

	var results []KeywordResult
	for word, freq := range wordCount {
		tf := float64(freq) / float64(len(words))
		idf, exists := tfidf.idf[word]
		if !exists {
			continue
		}

		tfidfScore := tf * idf
		results = append(results, KeywordResult{
			Word:      word,
			Frequency: freq,
			Score:     tfidfScore,
			PosTag:    "TFIDF",
		})
	}

	// Sort and limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}
