"""
Simple comparison script to test both Go and Python implementations
"""

import json
import requests
import subprocess
import time

def test_python_extraction():
    """Test the Python standalone extractor"""
    print("🐍 Testing Python extraction...")
    try:
        result = subprocess.run(['python', 'advanced_extractor.py'], 
                              capture_output=True, text=True, cwd='.')
        if result.returncode == 0:
            print("✅ Python extraction successful")
            # Load the results
            try:
                with open('python_keyword_results.json', 'r') as f:
                    return json.load(f)
            except FileNotFoundError:
                return None
        else:
            print(f"❌ Python extraction failed: {result.stderr}")
            return None
    except Exception as e:
        print(f"❌ Python extraction error: {e}")
        return None

def test_go_server(port=8080):
    """Test the Go server if it's running"""
    print("🔷 Testing Go server...")
    try:
        response = requests.get(f'http://localhost:{port}/issues/keywords-only?limit=10', 
                              timeout=5)
        if response.status_code == 200:
            print("✅ Go server responded")
            return response.json()
        else:
            print(f"❌ Go server error: {response.status_code}")
            return None
    except requests.exceptions.ConnectionError:
        print("❌ Go server not running or not accessible")
        return None
    except Exception as e:
        print(f"❌ Go server error: {e}")
        return None

def test_python_server(port=8081):
    """Test the Python FastAPI server if it's running"""
    print("🐍 Testing Python server...")
    try:
        # Test sample issue endpoint
        response = requests.get(f'http://localhost:{port}/compare-with-go', timeout=5)
        if response.status_code == 200:
            print("✅ Python server responded")
            return response.json()
        else:
            print(f"❌ Python server error: {response.status_code}")
            return None
    except requests.exceptions.ConnectionError:
        print("❌ Python server not running or not accessible")
        return None
    except Exception as e:
        print(f"❌ Python server error: {e}")
        return None

def compare_results():
    """Compare results from both implementations"""
    print("\n" + "="*60)
    print("KEYWORD EXTRACTION COMPARISON: GO vs PYTHON")
    print("="*60)
    
    # Test Python standalone
    python_results = test_python_extraction()
    
    # Test servers
    go_results = test_go_server()
    python_server_results = test_python_server()
    
    print("\n" + "="*60)
    print("COMPARISON RESULTS")
    print("="*60)
    
    if python_results:
        print("\n🐍 PYTHON STANDALONE RESULTS:")
        keywords = python_results.get('keywords', [])[:10]
        for i, kw in enumerate(keywords, 1):
            print(f"{i:2d}. {kw['word']:<15} | Score: {kw['score']:<8.2f} | Methods: {kw['method']}")
    
    if python_server_results:
        print("\n🐍 PYTHON SERVER RESULTS:")
        top_10 = python_server_results.get('results', {}).get('top_10', [])
        for i, kw in enumerate(top_10, 1):
            print(f"{i:2d}. {kw['word']:<15} | Score: {kw['score']:<8.2f} | Method: {kw['method']}")
    
    if go_results:
        print("\n🔷 GO SERVER RESULTS:")
        # Go results format might be different
        if isinstance(go_results, dict):
            for issue_id, keywords in go_results.items():
                print(f"Issue {issue_id}: {keywords}")
        else:
            print(f"Go results: {go_results}")
    
    # Analysis
    print("\n" + "="*60)
    print("ANALYSIS")
    print("="*60)
    
    python_available = python_results is not None
    go_available = go_results is not None
    
    print(f"🐍 Python extraction: {'✅ Available' if python_available else '❌ Not available'}")
    print(f"🔷 Go extraction: {'✅ Available' if go_available else '❌ Not available'}")
    
    if python_available:
        print("\n🐍 Python advantages:")
        print("   • Multiple extraction methods (spaCy, YAKE, KeyBERT, TF-IDF)")
        print("   • BERT-based semantic understanding")
        print("   • Advanced linguistic analysis")
        print("   • Mature NLP ecosystem")
        
    if go_available:
        print("\n🔷 Go advantages:")
        print("   • Faster execution")
        print("   • Lower memory usage")
        print("   • Better for production deployment")
        print("   • Single binary distribution")
    
    print("\n📊 Recommendations:")
    if python_available and go_available:
        print("   • Use Python for prototyping and complex NLP tasks")
        print("   • Use Go for production services with high throughput")
        print("   • Consider hybrid approach: Python for training, Go for serving")
    elif python_available:
        print("   • Python setup successful - rich NLP capabilities available")
        print("   • Consider implementing similar algorithms in Go for production")
    elif go_available:
        print("   • Go setup working - good for production deployment")
        print("   • Consider adding Python analysis for enhanced accuracy")
    else:
        print("   • Neither system fully operational - check setup")

def main():
    print("Starting keyword extraction comparison...")
    print("\nInstructions:")
    print("1. Make sure Go server is running: go run cmd/grok-server/main.go")
    print("2. Make sure Python environment is set up: ./setup.sh")
    print("3. Optionally start Python server: python main.py")
    print("\nPress Enter to continue...")
    input()
    
    compare_results()

if __name__ == "__main__":
    main()