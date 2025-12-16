# Python Backend for Keyword Extraction

This directory contains a comprehensive Python implementation using state-of-the-art NLP libraries for keyword extraction, designed to compare with the Go implementation.

## Features

### 🧠 Advanced NLP Libraries

- **spaCy**: Industrial-strength NLP with POS tagging and NER
- **scikit-learn**: TF-IDF vectorization and machine learning
- **NLTK**: Natural language processing toolkit
- **YAKE**: Yet Another Keyword Extractor algorithm
- **KeyBERT**: BERT-based keyword extraction
- **sentence-transformers**: Semantic similarity and embeddings

### 🔍 Multiple Extraction Methods

1. **spaCy NLP Pipeline**: POS tagging, lemmatization, linguistic analysis
2. **TF-IDF Scoring**: Statistical term importance across documents
3. **YAKE Algorithm**: Unsupervised keyword extraction
4. **KeyBERT**: BERT transformer-based semantic extraction
5. **Technical Pattern Recognition**: Regex patterns for versions, error codes, URLs
6. **Domain-specific Keywords**: Software engineering terminology boost

### 🚀 API Endpoints

- `GET /` - API information
- `GET /health` - Health check
- `POST /extract-keywords` - Extract from single issue
- `POST /extract-keywords/batch` - Batch processing
- `GET /compare-with-go` - Direct comparison with Go results
- `GET /issues/sample` - Sample issue for testing

## Setup

```bash
# Make setup script executable
chmod +x setup.sh

# Run setup (creates venv, installs packages, downloads models)
./setup.sh

# Activate environment
source venv/bin/activate
```

## Usage

### Standalone Testing

```bash
# Run the advanced extractor directly
python advanced_extractor.py
```

### Web API Server

```bash
# Start FastAPI server on port 8081
python main.py
```

### Comparison Tool

```bash
# Compare Go vs Python results
python compare.py
```

## Comparison with Go Implementation

| Aspect           | Python                        | Go                     |
| ---------------- | ----------------------------- | ---------------------- |
| **Accuracy**     | Higher (multiple algorithms)  | Good (prose/v2)        |
| **Speed**        | Slower (more processing)      | Faster                 |
| **Memory**       | Higher usage                  | Lower usage            |
| **Setup**        | Complex (many dependencies)   | Simple (single binary) |
| **NLP Features** | Extensive (BERT, spaCy, etc.) | Basic (POS tagging)    |
| **Production**   | Good for analysis/training    | Better for serving     |

## Libraries Installed

```
spacy>=3.7.0          # Industrial NLP
scikit-learn>=1.3.0   # Machine learning
nltk>=3.8.1           # Natural language toolkit
transformers>=4.35.0  # Hugging Face transformers
keybert>=0.8.3        # BERT-based extraction
yake>=0.4.8           # YAKE algorithm
sentence-transformers # Semantic embeddings
fastapi>=0.104.0      # Web API framework
```

## Sample Output

```
Top Keywords:
--------------------------------------------------------------------------------
#   Keyword              Score    Freq   Methods                   Confidence
--------------------------------------------------------------------------------
1   jenkins              45.50    4      spacy,domain_specific     0.000
2   deployment           32.75    4      spacy,tfidf,yake         0.245
3   timeout              28.00    3      spacy,domain_specific     0.000
4   error                25.50    3      spacy,domain_specific     0.000
5   production           24.00    4      spacy,tfidf              0.180
```

The Python implementation provides:

- **Multi-method consensus**: Keywords confirmed by multiple algorithms
- **Semantic understanding**: BERT-based contextual analysis
- **Technical pattern recognition**: Automatic detection of versions, codes, etc.
- **Domain expertise**: Software engineering terminology awareness
- **Confidence scores**: Reliability indicators for each extraction
