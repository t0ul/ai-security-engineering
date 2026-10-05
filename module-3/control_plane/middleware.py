import requests
from presidio_analyzer import AnalyzerEngine, PatternRecognizer, Pattern
from presidio_anonymizer import AnonymizerEngine
from telemetry import get_audit

class SecurityMiddleware:
    def __init__(self):
        print("🛡️  Initializing Presidio & NeMo Guardrails Middleware...")
        self.analyzer = AnalyzerEngine()
        self.anonymizer = AnonymizerEngine()
        self.nemo_url = "http://localhost:4001/check"

        # Presidio ships NO built-in "SECRET_KEY" recognizer, so the original
        # entities=[...,"SECRET_KEY"] matched nothing and credentials leaked
        # straight through the scrubber. Register a real recognizer covering the
        # shapes the red-team PoCs leak: AWS keys, env-var secret assignments,
        # API tokens, JWTs, and bearer tokens.
        secret_patterns = [
            Pattern(
                "secret_assignment",
                r"(?i)\b\w*(?:secret|passwd|password|token|apikey|api[_-]?key|access[_-]?key|credential|private[_-]?key)\w*\s*=\s*\S+",
                0.9,
            ),
            Pattern(
                "aws_access_key_id",
                r"\b(?:AKIA|ASIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|A3T[A-Z0-9])[A-Z0-9]{16}\b",
                0.9,
            ),
            Pattern(
                "generic_token_prefix",
                r"\b(?:sk|pk|rk|ghp|gho|ghs|xox[baprs])[-_][A-Za-z0-9]{16,}\b",
                0.85,
            ),
            Pattern(
                "jwt",
                r"\beyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b",
                0.9,
            ),
            Pattern(
                "bearer_token",
                r"(?i)\bbearer\s+[A-Za-z0-9._\-]{10,}\b",
                0.85,
            ),
        ]
        secret_recognizer = PatternRecognizer(
            supported_entity="SECRET_KEY",
            patterns=secret_patterns,
            context=["key", "secret", "token", "password", "aws", "credential", "api", "bearer"],
        )
        self.analyzer.registry.add_recognizer(secret_recognizer)

    def sanitize_input(self, raw_text: str, trace_id: str = "") -> str:
        audit = get_audit()

        # 1. Topic Guardrails (Semantic Routing via NeMo Microservice)
        print("   [NeMo] Checking semantic boundaries...")
        try:
            nemo_response = requests.post(self.nemo_url, json={"text": raw_text}, timeout=10).json()
            content = nemo_response.get("content", "")
            if "restricted to formatting terminal logs" in content:
                print("   🚫 [NeMo] BLOCKED: Out-of-bounds topic detected.")
                audit.emit(trace_id, "nemo", "decision", blocked=True)
                return content
            audit.emit(trace_id, "nemo", "decision", blocked=False)
        except Exception as e:
            audit.emit(trace_id, "nemo", "unreachable", error=str(e))
            print(f"   ⚠️ [NeMo] Warning: Guardrails service unreachable ({e}). Proceeding without semantic check.")

        # 2. PII + Secret Scrubbing (Presidio). Log WHAT was found (entity types
        # + count), never the values — the audit trail must not become the leak.
        print("   [Presidio] Scrubbing PII/PHI + secrets...")
        results = self.analyzer.analyze(
            text=raw_text,
            entities=["IP_ADDRESS", "EMAIL_ADDRESS", "SECRET_KEY"],
            language='en'
        )
        entity_types = sorted({r.entity_type for r in results})
        audit.emit(trace_id, "presidio", "scrub", hits=len(results), entities=entity_types)
        sanitized = self.anonymizer.anonymize(text=raw_text, analyzer_results=results)
        return sanitized.text
