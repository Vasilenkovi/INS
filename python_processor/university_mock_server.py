import json
from fastapi import FastAPI, Header, HTTPException
from pydantic import BaseModel
from typing import List, Dict, Any

app = FastAPI(title="University Local LLM Server (MOCK)")

# Модели данных для эндпоинтов
class ChatMessage(BaseModel):
    role: str
    content: str

class ChatPayload(BaseModel):
    model: str
    messages: List[ChatMessage]
    temperature: float = 0.2

# Идеальный ответ ИИ
MOCK_AI_REVIEW = {
    "score": 7,
    "issues": [
        {
            "line": 10,
            "severity": "critical",
            "category": "architecture",
            "message": "Обнаружена критическая ошибка: Слой контроллеров напрямую вызывает методы репозитория, минуя бизнес-логику (Service).",
            "suggestion": "return await self.user_service.get_user_data(user_id)"
        }
    ]
}

def generate_openai_response(model_name: str) -> Dict[str, Any]:
    """Вспомогательная функция: генерирует стандартный формат ответа OpenAI/HuggingFace"""
    return {
        "id": "chatcmpl-mock12345",
        "object": "chat.completion",
        "model": model_name,
        "choices": [
            {
                "index": 0,
                "message": {
                    "role": "assistant",
                    # ответ должен быть внутри строки JSON
                    "content": json.dumps(MOCK_AI_REVIEW, ensure_unicode=False)
                },
                "finish_reason": "stop"
            }
        ]
    }

# ==================== ВАРИАНТ 1: /api/v1/... ====================

@app.get("/api/v1/models")
async def get_models_v1():
    return {"models": ["qwen2.5-coder:7b", "qwen2.5-coder:32b", "deepseek-coder-v2:16b"]}

@app.post("/api/v1/chat")
async def chat_v1(payload: ChatPayload, authorization: str = Header(None)):
    if not authorization:
        raise HTTPException(status_code=401, detail="Missing Authorization Header")
    return generate_openai_response(payload.model)


# ==================== ВАРИАНТ 2: /v1/... ====================

@app.get("/v1/models")
async def get_models_v2():
    return {"data": [{"id": "qwen2.5-coder:7b"}, {"id": "deepseek-coder-v2:16b"}]}

@app.post("/v1/responses")
async def chat_v2(payload: ChatPayload, authorization: str = Header(None)):
    if not authorization:
        raise HTTPException(status_code=401, detail="Missing Authorization Header")
    return generate_openai_response(payload.model)

if __name__ == "__main__":
    import uvicorn
    # Запускаем сервер университета на порту 8001
    uvicorn.run(app, host="0.0.0.0", port=8001)
