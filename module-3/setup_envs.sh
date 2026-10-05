#!/usr/bin/env bash
set -e

echo "🚀 [1/3] Provisioning Primary Control Plane Environment (.venv)..."
python3.12 -m venv .venv
source .venv/bin/activate
pip install --upgrade pip
pip install \
    presidio-analyzer==2.2.355 \
    presidio-anonymizer==2.2.355 \
    langgraph==0.2.20 \
    langchain-core==0.3.5 \
    requests==2.32.3 \
    pydantic==2.10.6
python -m spacy download en_core_web_sm
deactivate

echo "🚀 [2/3] Provisioning AI Gateway Environment (.venv-gateway)..."
python3.12 -m venv .venv-gateway
source .venv-gateway/bin/activate
pip install --upgrade pip
pip install "litellm[proxy]==1.83.7"
deactivate

echo "🚀 [3/3] Provisioning NeMo Guardrails Microservice (.venv-nemo)..."
python3.12 -m venv .venv-nemo
source .venv-nemo/bin/activate
pip install --upgrade pip
pip install \
    nemoguardrails==0.9.0 \
    fastapi==0.115.8 \
    uvicorn==0.34.0
deactivate

echo "✅ All 3 isolated environments provisioned successfully!"