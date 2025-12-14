package config

import (
	"encoding/json"
	"os"
	"risk-analyzer/internal/models"
)

func LoadFromFile(path string) ([]models.Issue, error) {
	//open the file
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	//decode the json data
	var issues []models.Issue
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&issues); err != nil {
		return nil, err
	}

	return issues, nil
}
