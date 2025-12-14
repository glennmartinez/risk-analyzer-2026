"""
Advanced Keyword Extractor using multiple Python NLP libraries
Combines spaCy, scikit-learn, YAKE, KeyBERT, and custom techniques
"""

import re
import json
from typing import List, Dict, Tuple, Optional
from dataclasses import dataclass, asdict
from collections import Counter, defaultdict
import math

# NLP Libraries
import spacy
import nltk
from nltk.corpus import stopwords
from nltk.stem import PorterStemmer, WordNetLemmatizer
from sklearn.feature_extraction.text import TfidfVectorizer
from textblob import TextBlob
import yake
from keybert import KeyBERT
from sentence_transformers import SentenceTransformer

# Download required NLTK data
try:
    nltk.data.find('tokenizers/punkt')
except LookupError:
    nltk.download('punkt')
    
try:
    nltk.data.find('corpora/stopwords')
except LookupError:
    nltk.download('stopwords')
    
try:
    nltk.data.find('corpora/wordnet')
except LookupError:
    nltk.download('wordnet')

@dataclass
class KeywordResult:
    word: str
    frequency: int
    score: float
    method: str
    pos_tag: str = ""
    confidence: float = 0.0

@dataclass
class Issue:
    id: str
    title: str
    description: str
    issue_type: str
    components: List[str]

class AdvancedKeywordExtractor:
    """
    Multi-method keyword extractor combining:
    1. spaCy NLP pipeline
    2. TF-IDF scoring
    3. YAKE (Yet Another Keyword Extractor)
    4. KeyBERT (BERT-based extraction)
    5. Custom domain-specific rules
    """
    
    def __init__(self):
        # Load spaCy model
        try:
            self.nlp = spacy.load("en_core_web_sm")
        except OSError:
            print("Please install spaCy English model: python -m spacy download en_core_web_sm")
            self.nlp = None
            
        # Initialize other tools
        self.stemmer = PorterStemmer()
        self.lemmatizer = WordNetLemmatizer()
        self.stop_words = set(stopwords.words('english'))
        
        # Initialize advanced extractors
        self.keybert = None
        try:
            self.keybert = KeyBERT()
        except Exception as e:
            print(f"KeyBERT initialization failed: {e}")
            
        # YAKE extractor
        self.yake_extractor = yake.KeywordExtractor(
            lan="en",
            n=3,  # n-gram size
            dedupLim=0.7,
            top=20
        )
        
        # Domain-specific keywords for software issues
        self.domain_keywords = {
            'error', 'exception', 'fail', 'failure', 'bug', 'issue',
            'timeout', 'crash', 'performance', 'slow', 'fast',
            'memory', 'cpu', 'disk', 'network', 'database', 'db',
            'api', 'rest', 'endpoint', 'service', 'server', 'client',
            'security', 'auth', 'authentication', 'authorization',
            'deployment', 'deploy', 'build', 'pipeline', 'ci', 'cd',
            'jenkins', 'docker', 'kubernetes', 'k8s', 'aws', 'cloud',
            'version', 'release', 'update', 'patch', 'hotfix',
            'connection', 'ssl', 'https', 'http', 'tcp', 'udp'
        }
        
        # Technical patterns
        self.tech_patterns = {
            'version': re.compile(r'v?\d+\.\d+(?:\.\d+)?'),
            'error_code': re.compile(r'[A-Z]+\d{3,}'),
            'http_status': re.compile(r'HTTP\s*\d{3}|\b[45]\d{2}\b'),
            'url': re.compile(r'https?://[^\s]+'),
            'file_path': re.compile(r'/[^\s]+|[A-Z]:\\[^\s]+'),
            'port': re.compile(r':\d{2,5}\b'),
            'ip_address': re.compile(r'\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b')
        }
        
        # POS tag importance weights
        self.pos_weights = {
            'NOUN': 1.5,
            'PROPN': 2.0,  # Proper nouns (names, technologies)
            'ADJ': 1.2,
            'VERB': 1.1,
            'NUM': 1.3,
            'X': 1.8  # Foreign words, technical terms
        }
        
    def extract_keywords(self, issue: Issue, limit: int = 20) -> List[KeywordResult]:
        """
        Extract keywords using multiple methods and combine results
        """
        text = f"{issue.title} " * 3 + issue.description  # Weight title more
        
        all_keywords = []
        
        # Method 1: spaCy-based extraction
        if self.nlp:
            spacy_keywords = self._extract_spacy_keywords(text)
            all_keywords.extend(spacy_keywords)
        
        # Method 2: TF-IDF
        tfidf_keywords = self._extract_tfidf_keywords([text])
        all_keywords.extend(tfidf_keywords)
        
        # Method 3: YAKE
        yake_keywords = self._extract_yake_keywords(text)
        all_keywords.extend(yake_keywords)
        
        # Method 4: KeyBERT (BERT-based)
        if self.keybert:
            keybert_keywords = self._extract_keybert_keywords(text)
            all_keywords.extend(keybert_keywords)
        
        # Method 5: Technical patterns
        tech_keywords = self._extract_technical_patterns(text)
        all_keywords.extend(tech_keywords)
        
        # Method 6: Domain-specific extraction
        domain_keywords = self._extract_domain_keywords(text)
        all_keywords.extend(domain_keywords)
        
        # Combine and rank results
        final_keywords = self._combine_and_rank_keywords(all_keywords)
        
        return final_keywords[:limit]
    
    def _extract_spacy_keywords(self, text: str) -> List[KeywordResult]:
        """Extract keywords using spaCy NLP pipeline"""
        doc = self.nlp(text)
        keywords = []
        word_counts = Counter()
        
        # Count meaningful tokens
        for token in doc:
            if (not token.is_stop and 
                not token.is_punct and 
                not token.is_space and 
                len(token.text) > 2 and
                token.pos_ in ['NOUN', 'PROPN', 'ADJ', 'VERB', 'NUM']):
                
                lemma = token.lemma_.lower()
                word_counts[lemma] += 1
        
        # Create KeywordResult objects
        for word, freq in word_counts.items():
            # Get POS tag for scoring
            pos_weight = 1.0
            for token in doc:
                if token.lemma_.lower() == word:
                    pos_weight = self.pos_weights.get(token.pos_, 1.0)
                    pos_tag = token.pos_
                    break
            
            score = freq * pos_weight
            keywords.append(KeywordResult(
                word=word,
                frequency=freq,
                score=score,
                method="spacy",
                pos_tag=pos_tag
            ))
        
        return keywords
    
    def _extract_tfidf_keywords(self, texts: List[str]) -> List[KeywordResult]:
        """Extract keywords using TF-IDF scoring"""
        if len(texts) < 2:
            # Add some dummy documents for TF-IDF calculation
            texts.extend([
                "software development programming code",
                "system server network database error",
                "user interface application frontend backend"
            ])
        
        vectorizer = TfidfVectorizer(
            max_features=100,
            stop_words='english',
            ngram_range=(1, 2),
            min_df=1
        )
        
        try:
            tfidf_matrix = vectorizer.fit_transform(texts)
            feature_names = vectorizer.get_feature_names_out()
            
            # Get scores for the first document (our issue)
            doc_scores = tfidf_matrix[0].toarray()[0]
            
            keywords = []
            for i, score in enumerate(doc_scores):
                if score > 0:
                    keywords.append(KeywordResult(
                        word=feature_names[i],
                        frequency=1,
                        score=score * 10,  # Scale up for comparison
                        method="tfidf"
                    ))
            
            return keywords
        except Exception as e:
            print(f"TF-IDF extraction failed: {e}")
            return []
    
    def _extract_yake_keywords(self, text: str) -> List[KeywordResult]:
        """Extract keywords using YAKE algorithm"""
        try:
            yake_results = self.yake_extractor.extract_keywords(text)
            keywords = []
            
            for keyword, score in yake_results:
                # YAKE returns lower scores for better keywords
                normalized_score = 1.0 / (score + 0.1)  # Invert and normalize
                keywords.append(KeywordResult(
                    word=keyword.lower(),
                    frequency=1,
                    score=normalized_score,
                    method="yake"
                ))
            
            return keywords
        except Exception as e:
            print(f"YAKE extraction failed: {e}")
            return []
    
    def _extract_keybert_keywords(self, text: str) -> List[KeywordResult]:
        """Extract keywords using KeyBERT (BERT-based)"""
        try:
            keybert_results = self.keybert.extract_keywords(
                text, 
                keyphrase_ngram_range=(1, 2), 
                stop_words='english',
                top_k=15
            )
            
            keywords = []
            for keyword, score in keybert_results:
                keywords.append(KeywordResult(
                    word=keyword.lower(),
                    frequency=1,
                    score=score * 5,  # Scale for comparison
                    method="keybert",
                    confidence=score
                ))
            
            return keywords
        except Exception as e:
            print(f"KeyBERT extraction failed: {e}")
            return []
    
    def _extract_technical_patterns(self, text: str) -> List[KeywordResult]:
        """Extract technical terms using regex patterns"""
        keywords = []
        
        for pattern_name, pattern in self.tech_patterns.items():
            matches = pattern.findall(text.lower())
            for match in set(matches):  # Remove duplicates
                keywords.append(KeywordResult(
                    word=match,
                    frequency=text.lower().count(match),
                    score=2.0,  # Technical terms are important
                    method=f"technical_{pattern_name}"
                ))
        
        return keywords
    
    def _extract_domain_keywords(self, text: str) -> List[KeywordResult]:
        """Extract domain-specific keywords"""
        text_lower = text.lower()
        keywords = []
        
        for domain_word in self.domain_keywords:
            if domain_word in text_lower:
                frequency = text_lower.count(domain_word)
                keywords.append(KeywordResult(
                    word=domain_word,
                    frequency=frequency,
                    score=frequency * 2.5,  # Domain terms are very important
                    method="domain_specific"
                ))
        
        return keywords
    
    def _combine_and_rank_keywords(self, all_keywords: List[KeywordResult]) -> List[KeywordResult]:
        """Combine keywords from different methods and create final ranking"""
        word_groups = defaultdict(list)
        
        # Group by word (considering stemming)
        for kw in all_keywords:
            stem = self.stemmer.stem(kw.word)
            word_groups[stem].append(kw)
        
        combined_keywords = []
        
        for stem, keyword_list in word_groups.items():
            # Find the best representative word (shortest, most frequent)
            best_word = min(keyword_list, key=lambda x: (len(x.word), -x.frequency)).word
            
            # Combine scores from different methods
            total_score = 0
            total_frequency = 0
            methods = []
            max_confidence = 0
            
            for kw in keyword_list:
                total_score += kw.score
                total_frequency += kw.frequency
                methods.append(kw.method)
                max_confidence = max(max_confidence, kw.confidence)
            
            # Boost score if multiple methods agree
            method_diversity_bonus = len(set(methods)) * 0.5
            final_score = total_score + method_diversity_bonus
            
            combined_keywords.append(KeywordResult(
                word=best_word,
                frequency=total_frequency,
                score=final_score,
                method=",".join(set(methods)),
                confidence=max_confidence
            ))
        
        # Sort by score
        combined_keywords.sort(key=lambda x: x.score, reverse=True)
        
        return combined_keywords

def main():
    """Test the advanced keyword extractor"""
    # Sample issue (same as Go version for comparison)
    issue = Issue(
        id="ISSUE-123",
        title="Jenkins build fails with timeout error in production deployment",
        description="The Jenkins CI/CD pipeline is consistently failing during the production deployment phase. The error occurs after 30 minutes with a timeout exception. This issue affects the BigRedButton component and prevents automatic deployments to the China region. The build logs show memory allocation errors and network connectivity issues with the database connection pool.",
        issue_type="bug",
        components=["Jenkins", "BigRedButton", "China"]
    )
    
    print("=" * 60)
    print("PYTHON ADVANCED KEYWORD EXTRACTION")
    print("=" * 60)
    
    extractor = AdvancedKeywordExtractor()
    keywords = extractor.extract_keywords(issue, limit=15)
    
    print(f"\nExtracting keywords from issue: {issue.id}")
    print(f"Title: {issue.title}")
    print(f"Description length: {len(issue.description)} characters\n")
    
    print("Top Keywords:")
    print("-" * 80)
    print(f"{'#':<3} {'Keyword':<20} {'Score':<8} {'Freq':<6} {'Methods':<25} {'Confidence':<10}")
    print("-" * 80)
    
    for i, kw in enumerate(keywords, 1):
        print(f"{i:<3} {kw.word:<20} {kw.score:<8.2f} {kw.frequency:<6} {kw.method:<25} {kw.confidence:<10.3f}")
    
    # Export results for comparison
    results = {
        "issue": asdict(issue),
        "keywords": [asdict(kw) for kw in keywords],
        "extraction_methods": [
            "spaCy NLP pipeline",
            "TF-IDF scoring", 
            "YAKE algorithm",
            "KeyBERT (BERT-based)",
            "Technical pattern matching",
            "Domain-specific extraction"
        ]
    }
    
    with open('python_keyword_results.json', 'w') as f:
        json.dump(results, f, indent=2)
    
    print(f"\nResults saved to 'python_keyword_results.json'")
    print(f"Total keywords extracted: {len(keywords)}")

if __name__ == "__main__":
    main()