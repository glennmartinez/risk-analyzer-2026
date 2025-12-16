"""
Simplified Advanced Keyword Extractor for Python 3.14 compatibility
Uses NLTK, scikit-learn, YAKE, and TextBlob
"""

import re
import json
from typing import List, Dict, Tuple, Optional
from dataclasses import dataclass, asdict
from collections import Counter, defaultdict
import math

# NLP Libraries
import nltk
from nltk.corpus import stopwords
from nltk.stem import PorterStemmer, WordNetLemmatizer
from sklearn.feature_extraction.text import TfidfVectorizer
from textblob import TextBlob
import yake

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

try:
    nltk.data.find('taggers/averaged_perceptron_tagger')
except LookupError:
    nltk.download('averaged_perceptron_tagger')

try:
    nltk.data.find('tokenizers/punkt_tab')
except LookupError:
    nltk.download('punkt_tab')

try:
    nltk.data.find('taggers/averaged_perceptron_tagger_eng')
except LookupError:
    nltk.download('averaged_perceptron_tagger_eng')

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
    1. NLTK NLP pipeline
    2. TF-IDF scoring  
    3. YAKE (Yet Another Keyword Extractor)
    4. TextBlob analysis
    5. Custom domain-specific rules
    """
    
    def __init__(self):
        # Initialize tools
        self.stemmer = PorterStemmer()
        self.lemmatizer = WordNetLemmatizer()
        self.stop_words = set(stopwords.words('english'))
        
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
        
        # POS tag importance weights (NLTK format)
        self.pos_weights = {
            'NN': 1.5,   # noun, singular
            'NNS': 1.5,  # noun plural
            'NNP': 2.0,  # proper noun, singular
            'NNPS': 2.0, # proper noun, plural
            'JJ': 1.2,   # adjective
            'JJR': 1.2,  # adjective, comparative
            'JJS': 1.2,  # adjective, superlative
            'VB': 1.1,   # verb, base form
            'VBD': 1.1,  # verb, past tense
            'VBG': 1.1,  # verb, gerund/present participle
            'VBN': 1.1,  # verb, past participle
            'VBP': 1.1,  # verb, present tense
            'VBZ': 1.1,  # verb, 3rd person singular present
            'RB': 0.9,   # adverb
            'CD': 1.3    # cardinal number
        }
        
    def extract_keywords(self, issue: Issue, limit: int = 20) -> List[KeywordResult]:
        """
        Extract keywords using multiple methods and combine results
        """
        text = f"{issue.title} " * 3 + issue.description  # Weight title more
        
        all_keywords = []
        
        # Method 1: NLTK-based extraction
        nltk_keywords = self._extract_nltk_keywords(text)
        all_keywords.extend(nltk_keywords)
        
        # Method 2: TF-IDF
        tfidf_keywords = self._extract_tfidf_keywords([text])
        all_keywords.extend(tfidf_keywords)
        
        # Method 3: YAKE
        yake_keywords = self._extract_yake_keywords(text)
        all_keywords.extend(yake_keywords)
        
        # Method 4: TextBlob analysis
        textblob_keywords = self._extract_textblob_keywords(text)
        all_keywords.extend(textblob_keywords)
        
        # Method 5: Technical patterns
        tech_keywords = self._extract_technical_patterns(text)
        all_keywords.extend(tech_keywords)
        
        # Method 6: Domain-specific extraction
        domain_keywords = self._extract_domain_keywords(text)
        all_keywords.extend(domain_keywords)
        
        # Combine and rank results
        final_keywords = self._combine_and_rank_keywords(all_keywords)
        
        return final_keywords[:limit]
    
    def _extract_nltk_keywords(self, text: str) -> List[KeywordResult]:
        """Extract keywords using NLTK NLP pipeline"""
        # Tokenize and POS tag
        tokens = nltk.word_tokenize(text.lower())
        pos_tags = nltk.pos_tag(tokens)
        
        keywords = []
        word_counts = Counter()
        word_pos = {}
        
        # Count meaningful tokens
        for word, pos in pos_tags:
            if (len(word) > 2 and 
                word.isalpha() and 
                word not in self.stop_words and
                pos in self.pos_weights):
                
                lemma = self.lemmatizer.lemmatize(word)
                word_counts[lemma] += 1
                word_pos[lemma] = pos
        
        # Create KeywordResult objects
        for word, freq in word_counts.items():
            pos_tag = word_pos.get(word, 'NN')
            pos_weight = self.pos_weights.get(pos_tag, 1.0)
            score = freq * pos_weight
            
            keywords.append(KeywordResult(
                word=word,
                frequency=freq,
                score=score,
                method="nltk",
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
        
        try:
            vectorizer = TfidfVectorizer(
                max_features=100,
                stop_words='english',
                ngram_range=(1, 2),
                min_df=1
            )
            
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
    
    def _extract_textblob_keywords(self, text: str) -> List[KeywordResult]:
        """Extract keywords using TextBlob"""
        try:
            blob = TextBlob(text)
            keywords = []
            
            # Extract noun phrases
            for phrase in blob.noun_phrases:
                if len(phrase) > 2 and phrase not in self.stop_words:
                    keywords.append(KeywordResult(
                        word=phrase.lower(),
                        frequency=1,
                        score=2.0,  # Noun phrases are important
                        method="textblob_np",
                        pos_tag="NP"
                    ))
            
            # Extract words with sentiment consideration
            for word, pos in blob.tags:
                if (pos.startswith(('NN', 'JJ', 'VB')) and 
                    len(word) > 2 and 
                    word.lower() not in self.stop_words):
                    
                    # Simple sentiment boost for negative words (common in issue reports)
                    sentiment_score = 1.0
                    word_sentiment = TextBlob(word).sentiment.polarity
                    if word_sentiment < -0.1:  # Negative sentiment
                        sentiment_score = 1.5
                    
                    keywords.append(KeywordResult(
                        word=word.lower(),
                        frequency=1,
                        score=sentiment_score,
                        method="textblob_word",
                        pos_tag=pos
                    ))
            
            return keywords
        except Exception as e:
            print(f"TextBlob extraction failed: {e}")
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
    
    def _calculate_confidence(self, methods, frequency, pos_tag, word, is_domain_word=False):
        """Calculate confidence score based on multiple factors"""
        confidence = 0.0
        
        # Method consensus bonus (0.0-0.4)
        method_count = len(set(methods))
        confidence += min(method_count * 0.1, 0.4)
        
        # Frequency bonus (0.0-0.3)
        confidence += min(frequency * 0.05, 0.3)
        
        # POS tag reliability (0.0-0.2)
        reliable_pos = {'NN', 'NNS', 'NNP', 'NNPS', 'VB', 'VBG', 'JJ'}
        if pos_tag in reliable_pos:
            confidence += 0.2
        elif pos_tag:
            confidence += 0.1
        
        # Domain relevance bonus (0.0-0.2)
        if is_domain_word:
            confidence += 0.2
        
        # Word length penalty for very short words
        if len(word) <= 2:
            confidence *= 0.5
        
        # Cap confidence at 1.0
        return min(confidence, 1.0)
    
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
            tfidf_confidence = 0
            best_pos_tag = ""
            
            for kw in keyword_list:
                total_score += kw.score
                total_frequency += kw.frequency
                methods.append(kw.method)
                
                # Extract TF-IDF confidence if available
                if kw.method == 'tfidf' and kw.score > 0:
                    tfidf_confidence = min(kw.score / 10.0, 1.0)  # Normalize TF-IDF
                
                if kw.pos_tag:
                    best_pos_tag = kw.pos_tag
            
            # Check if it's a domain word
            is_domain_word = best_word.lower() in self.domain_keywords
            
            # Calculate confidence
            confidence = self._calculate_confidence(
                methods, total_frequency, best_pos_tag, best_word, is_domain_word
            )
            
            # Add TF-IDF confidence component
            confidence = min(confidence + tfidf_confidence * 0.3, 1.0)
            
            # Boost score if multiple methods agree
            method_diversity_bonus = len(set(methods)) * 0.5
            final_score = total_score + method_diversity_bonus
            
            combined_keywords.append(KeywordResult(
                word=best_word,
                frequency=total_frequency,
                score=final_score,
                method=",".join(set(methods)),
                pos_tag=best_pos_tag,
                confidence=round(confidence, 3)
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
    print("PYTHON ADVANCED KEYWORD EXTRACTION (Simplified)")
    print("=" * 60)
    
    extractor = AdvancedKeywordExtractor()
    keywords = extractor.extract_keywords(issue, limit=15)
    
    print(f"\nExtracting keywords from issue: {issue.id}")
    print(f"Title: {issue.title}")
    print(f"Description length: {len(issue.description)} characters\n")
    
    print("Top Keywords:")
    print("-" * 80)
    print(f"{'#':<3} {'Keyword':<20} {'Score':<8} {'Freq':<6} {'Methods':<25} {'POS':<8}")
    print("-" * 80)
    
    for i, kw in enumerate(keywords, 1):
        print(f"{i:<3} {kw.word:<20} {kw.score:<8.2f} {kw.frequency:<6} {kw.method:<25} {kw.pos_tag:<8}")
    
    # Export results for comparison
    results = {
        "issue": asdict(issue),
        "keywords": [asdict(kw) for kw in keywords],
        "extraction_methods": [
            "NLTK NLP pipeline",
            "TF-IDF scoring", 
            "YAKE algorithm",
            "TextBlob analysis",
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