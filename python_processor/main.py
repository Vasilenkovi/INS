import os
import yaml
from fastapi import FastAPI, HTTPException
from fastapi.responses import PlainTextResponse
from src.schemas import AnalysisRequest
from src.config_loader import CompleteUniversityStandard
from src.prompt_builder import PromptBuilder
from src.llm_client import DeepSeekClient

app = FastAPI(title="Code-Review Assistant: LLM Gateway")
llm_client = DeepSeekClient()


@app.post("/api/v1/analyze", response_class=PlainTextResponse)
async def analyze_code(payload: AnalysisRequest):
    try:
        # парсим YAML-строку стандарта, которую Go вытащил из  БД
        try:
            raw_config = yaml.safe_load(payload.standard_yaml)
            standard = CompleteUniversityStandard(**raw_config)
        except Exception as config_err:
            raise HTTPException(
                status_code=400,
                detail=f"Ошибка валидации стандарта, присланного из БД Go: {str(config_err)}"
            )

        # формируем промты, внедряя в них file_path
        system_prompt = PromptBuilder.build_system_prompt(standard)
        user_prompt = PromptBuilder.build_user_prompt(payload.code_content, payload.file_path, standard)

        # отправляем запрос в LM Studio университета
        yaml_report = await llm_client.generate_review_as_yaml(system_prompt, user_prompt)
        return yaml_report

    except HTTPException as http_ex:
        raise http_ex
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Analysis Error: {str(e)}")


if __name__ == "__main__":
    import uvicorn

    app_port = int(os.getenv("APP_PORT", 8080))
    uvicorn.run(app, host="0.0.0.0", port=app_port)
