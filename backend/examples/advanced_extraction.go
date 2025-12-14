package main

import (
	"fmt"
	"log"
	"risk-analyzer/internal/models"
	"risk-analyzer/internal/services"
)

func main() {
	// Create a sample issue with technical terms
	issue := models.Issue{
		Id:          "ISSUE-456",
		Title:       "API timeout error v2.1.3 returns HTTP 500",
		Description: "The REST API endpoint /api/users is throwing timeout exceptions after 30 seconds. Error code API001 appears in logs. This started after deployment of version 2.1.3 to production environment. Database connection pool shows max connections reached. Server logs indicate memory leak in authentication module.",
		IssueType:   "bug",
		Components:  []models.Component{models.Jenkins},
	}

	fmt.Println("=== Basic Extraction ===")
	basicExtractor := services.NewKeywordExtractor()
	basicKeywords, _ := basicExtractor.ExtractTopKeywords(issue, 8)
	for i, kw := range basicKeywords {
		fmt.Printf("%d. %s (%.2f) [%s]\n", i+1, kw.Word, kw.Score, kw.PosTag)
	}

	fmt.Println("\n=== Advanced Extraction ===")
	advancedExtractor := services.NewAdvancedKeywordExtractor()
	advancedKeywords, err := advancedExtractor.ExtractAdvancedKeywords(issue)
	if err != nil {
		log.Fatal(err)
	}

	for i, kw := range advancedKeywords[:min(10, len(advancedKeywords))] {
		fmt.Printf("%d. %s (%.2f) [%s] freq:%d\n",
			i+1, kw.Word, kw.Score, kw.PosTag, kw.Frequency)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
