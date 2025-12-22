package main

import (
	"fmt"
	"log"
	"risk-analyzer/internal/models"
	"risk-analyzer/internal/services"
)

func main() {
	// Create a sample issue
	issue := models.Issue{
		Id:          "ISSUE-123",
		Title:       "Jenkins build fails with timeout error in production deployment",
		Description: "The Jenkins CI/CD pipeline is consistently failing during the production deployment phase. The error occurs after 30 minutes with a timeout exception. This issue affects the BigRedButton component and prevents automatic deployments to the China region. The build logs show memory allocation errors and network connectivity issues with the database connection pool.",
		IssueType:   "bug",
		Components:  []models.Component{models.Jenkins, models.BigRedButton, models.China},
	}

	// Create keyword extractor
	extractor := services.NewKeywordExtractor()

	// Extract all keywords with details
	fmt.Println("=== Detailed Keywords ===")
	keywords, err := extractor.ExtractKeywords(issue)
	if err != nil {
		log.Fatal(err)
	}

	for i, kw := range keywords {
		if i >= 15 { // Show top 15
			break
		}
		fmt.Printf("%-15s | Freq: %d | Score: %.2f | POS: %s\n",
			kw.Word, kw.Frequency, kw.Score, kw.PosTag)
	}

	// Extract top 10 keywords
	fmt.Println("\n=== Top 10 Keywords ===")
	topKeywords, err := extractor.ExtractTopKeywords(issue, 10)
	if err != nil {
		log.Fatal(err)
	}

	for i, kw := range topKeywords {
		fmt.Printf("%d. %s (%.2f)\n", i+1, kw.Word, kw.Score)
	}

	// Extract just keyword strings (simple version)
	fmt.Println("\n=== Simple Keyword List ===")
	keywordStrings, err := extractor.ExtractKeywordStrings(issue, 8)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Keywords: %v\n", keywordStrings)
}
