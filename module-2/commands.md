## downloading to local macOS
using macos as control plane host

```
# 1. Deactivate and remove the bleeding-edge virtual environment
deactivate
rm -rf .venv

# 2. Recreate it specifically using Python 3.12 (install via Homebrew if missing: brew install python@3.12)
python3.12 -m venv .venv

# 3. Activate the new environment
source .venv/bin/activate

# 4. Install the exact syllabus dependencies (this will now download the pre-built wheels instantly)
pip install requests langgraph==0.2.20 langchain-core==0.3.5 presidio-analyzer==2.2.353 presidio-anonymizer==2.2.353 spacy==3.7.4

# 5. Download the spaCy model
python3 -m spacy download en_core_web_sm
```