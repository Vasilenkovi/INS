import os
from fastapi import FastAPI, HTTPException
from fastapi.responses import PlainTextResponse
from pydantic import BaseModel
from src.schemas import AnalysisRequest
from src.config_loader import load_standard
from src.prompt_builder import PromptBuilder
from src.llm_client import DeepSeekClient

app = FastAPI(title="Code-Review Assistant: LLM Gateway")
llm_client = DeepSeekClient()


class TextConfigPayload(BaseModel):
    description: str


@app.post("/api/v1/analyze", response_class=PlainTextResponse)
async def analyze_code(payload: AnalysisRequest):
    """Принимает код от Go, делает ревью, отдает YAML-отчет"""
    try:
        config_path = os.getenv("RULES_CONFIG_PATH", "config/standard.yaml")
        standard = load_standard(config_path)

        system_prompt = PromptBuilder.build_system_prompt(standard)
        user_prompt = PromptBuilder.build_user_prompt(payload.code_content, payload.file_path, standard)

        yaml_report = await llm_client.generate_review_as_yaml(system_prompt, user_prompt)
        return yaml_report
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Analysis Error: {str(e)}")


@app.post("/api/v1/config/generate", response_class=PlainTextResponse)
async def generate_config_from_text(payload: TextConfigPayload):
    """принимает сырой текст, отдает готовый YAML-стандарт"""
    try:
        yaml_config = await llm_client.convert_text_to_yaml_config(payload.description)
        return yaml_config
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Config Generation Error: {str(e)}")


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8080)
