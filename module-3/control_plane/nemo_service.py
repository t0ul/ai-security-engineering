import os
import uvicorn
from fastapi import FastAPI
from pydantic import BaseModel
from nemoguardrails import RailsConfig, LLMRails

app = FastAPI()
print("🛡️  Initializing NeMo Guardrails Engine...")

# Dynamically resolve the path to the config folder right next to this script
current_dir = os.path.dirname(os.path.abspath(__file__))
config_path = os.path.join(current_dir, "nemo_config")

config = RailsConfig.from_path(config_path)
rails = LLMRails(config)

class GuardRequest(BaseModel):
    text: str

@app.post("/check")
async def check_topic(req: GuardRequest):
    response = await rails.generate_async(messages=[{"role": "user", "content": req.text}])
    return {"content": response["content"]}

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=4001)