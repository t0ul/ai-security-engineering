import requests
from presidio_analyzer import AnalyzerEngine
from presidio_anonymizer import AnonymizerEngine

class SecurityMiddleware:
    def __init__(self):
        print("🛡️  Initializing Presidio & NeMo Guardrails Middleware...")
        self.analyzer = AnalyzerEngine()
        self.anonymizer = AnonymizerEngine()
        self.nemo_url = "http://localhost:4001/check"

    def sanitize_input(self, raw_text: str) -> str:
        # 1. Topic Guardrails (Semantic Routing via NeMo Microservice)
        print("   [NeMo] Checking semantic boundaries...")
        try:
            nemo_response = requests.post(self.nemo_url, json={"text": raw_text}, timeout=10).json()
            if "restricted to formatting terminal logs" in nemo_response.get("content", ""):
                print("   🚫 [NeMo] BLOCKED: Out-of-bounds topic detected.")
                return nemo_response["content"]
        except Exception as e:
            print(f"   ⚠️ [NeMo] Warning: Guardrails service unreachable ({e}). Proceeding without semantic check.")

        # 2. PII Scrubbing (Presidio)
        print("   [Presidio] Scrubbing PII/PHI...")
        results = self.analyzer.analyze(
            text=raw_text, 
            entities=["IP_ADDRESS", "EMAIL_ADDRESS", "SECRET_KEY"], 
            language='en'
        )
        sanitized = self.anonymizer.anonymize(text=raw_text, analyzer_results=results)
        return sanitized.text