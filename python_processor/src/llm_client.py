import os
import json
import yaml
import asyncio
import httpx


class DeepSeekClient:
    def __init__(self):
        self.api_key = os.getenv("DEEPSEEK_API_KEY")
        # Адрес сервера университета (например, http://192.168.1.50:8000)
        self.base_url = os.getenv("DEEPSEEK_BASE_URL", "http://localhost:8000")

        # Переключатель версий API университета (true = /api/v1, false = /v1)
        self.use_new_api_prefix = os.getenv("USE_NEW_API", "true").lower() == "true"

        # Настраиваем эндпоинты по данным университета
        if self.use_new_api_prefix:
            self.models_url = f"{self.base_url}/api/v1/models"
            self.chat_url = f"{self.base_url}/api/v1/chat"
        else:
            self.models_url = f"{self.base_url}/v1/models"
            self.chat_url = f"{self.base_url}/v1/responses"

        # Включение mock-режима, если ключ не задан
        if self.api_key and not self.api_key.startswith("mock"):
            self.is_mock = False
        else:
            self.is_mock = True
            print(f"⚠️ MOCK-РЕЖИМ. Эндпоинты университета макетированы: {self.chat_url}")

    async def generate_review_as_yaml(self, system_prompt: str, user_prompt: str) -> str:
        """Отправка запроса на анализ в API университета"""
        if self.is_mock:
            await asyncio.sleep(1.5)
            mock_json_response = {
                "score": 5,
                "issues": [
                    {
                        "line": 12,
                        "severity": "critical",
                        "category": "architecture",
                        "message": "Нарушение стандарта вуза: Контроллер обращается к репозиторию в обход слоя Service.",
                        "suggestion": "return await self.user_service.get_user(user_id)"
                    }
                ]
            }
            return yaml.dump(mock_json_response, allow_unicode=True, sort_keys=False, default_flow_style=False)

        # РЕАЛЬНЫЙ ЗАПРОС К API УНИВЕРСИТЕТА
        # Формируем стандартное тело запроса
        payload = {
            "model": os.getenv("LLM_MODEL_NAME", "qwen2.5-coder:7b"),
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt}
            ],
            "temperature": 0.2
        }

        headers = {"Authorization": f"Bearer {self.api_key}", "Content-Type": "application/json"}

        async with httpx.AsyncClient() as client:
            try:
                # Шлем POST запрос на  /api/v1/chat или /v1/responses
                response = await client.post(self.chat_url, json=payload, headers=headers, timeout=30.0)
                response.raise_for_status()

                # Парсим ответ
                response_data = response.json()

                # Извлекаем текст ответа ИИ
                ai_content = response_data["choices"][0]["message"]["content"]

                # Конвертируем строку от ИИ (JSON) в словарь, а затем в YAML для Go-бэкенда
                json_dict = json.loads(ai_content)
                return yaml.dump(json_dict, allow_unicode=True, sort_keys=False, default_flow_style=False)

            except Exception as e:
                return yaml.dump({"status": "error", "message": f"University API Error: {str(e)}"})
