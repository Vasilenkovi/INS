import os
import yaml
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from src.schemas import AnalysisRequest
from src.config_loader import CompleteUniversityStandard
from src.prompt_builder import PromptBuilder
from src.llm_client import DeepSeekClient

app = FastAPI(title="Code-Review Assistant: LLM Gateway")
llm_client = DeepSeekClient()


# Основной эндпоинт  /review, а /api/v1/analyze оставлен как алиас для совместимости
@app.post("/review")
@app.post("/api/v1/analyze")
async def analyze_code(payload: AnalysisRequest):
    try:
        system_prompt = ""
        user_prompt = ""

        # Определяем тип содержимого в standard_yaml
        try:
            raw_config = yaml.safe_load(payload.standard_yaml)

            # Проверяем, пришел ли полный структурированный YAML
            if isinstance(raw_config, dict) and ("project" in raw_config or "version" in raw_config):
                # парсим как полную Pydantic-схему
                standard = CompleteUniversityStandard(**raw_config)
                system_prompt = PromptBuilder.build_system_prompt(standard)
                user_prompt = PromptBuilder.build_user_prompt(payload.code_content, payload.file_path, standard)
            else:
                # Если пришел plain-text, используем дефолтные промты
                raise ValueError("Plain text detected")

        except Exception:
            # если Go прислал plain-text из BuildPromptContext()
            # Формируем базовую системную инструкцию и подмешиваем туда присланные правила
            system_prompt = (
                "Ты — Senior Software Architect. Проведи ревью кода. "
                "Игнорируй форматирование и пробелы. Фокусируйся на архитектурных слоях и безопасности.\n"
                f"ДОПОЛНИТЕЛЬНЫЕ ПРАВИЛА И КОНТЕКСТ ОТ УНИВЕРСИТЕТА:\n{payload.standard_yaml}\n\n"
                "Ответ верни СТРОГО в формате JSON:\n"
                "{\n"
                "  \"score\": 10,\n"
                "  \"issues\": [\n"
                "    {\n"
                "      \"line\": 15,\n"
                "      \"severity\": \"critical|warning|info\",\n"
                "      \"category\": \"architecture|security|business_logic\",\n"
                "      \"message\": \"Текст замечания.\",\n"
                "      \"suggestion\": \"Пример исправления\"\n"
                "    }\n"
                "  ]\n"
                "}"
            )
            # пользовательский промт в этом случае  упрощаем
            user_prompt = f"Файл для анализа: `{payload.file_path}`\n\nКОД ДЛЯ РЕВЬЮ:\n```\n{payload.code_content}\n```"

        # Отправляем запрос в LM Studio университета
        # Внутри llm_client модель возвращает YAML-строку
        raw_yaml_report = await llm_client.generate_review_as_yaml(system_prompt, user_prompt)

        # Конвертируем YAML-ответ от ИИ обратно в JSON-объект
        try:
            parsed_response = yaml.safe_load(raw_yaml_report)
            return JSONResponse(content=parsed_response)
        except Exception:
            #  если ИИ выдал невалидный формат
            return JSONResponse(
                status_code=422,
                content={"score": 1, "issues": [
                    {"line": 1, "severity": "critical", "category": "logic", "message": "Failed to parse LLM response",
                     "suggestion": ""}]}
            )

    except HTTPException as http_ex:
        raise http_ex
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Internal Analysis Error: {str(e)}")


if __name__ == "__main__":
    import uvicorn

    app_port = int(os.getenv("APP_PORT", 8080))
    uvicorn.run(app, host="0.0.0.0", port=app_port)
