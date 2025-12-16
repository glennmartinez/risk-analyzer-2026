#!/bin/bash

echo "Setting up Python backend for keyword extraction..."

# Create virtual environment
python3 -m venv venv
source venv/bin/activate

# Upgrade pip
pip install --upgrade pip

# Install requirements
pip install -r requirements.txt

# Download spaCy English model
python -m spacy download en_core_web_sm

echo "Setup complete!"
echo ""
echo "To activate the environment and run the server:"
echo "  source venv/bin/activate"
echo "  python main.py"
echo ""
echo "Or run the standalone test:"
echo "  python advanced_extractor.py"