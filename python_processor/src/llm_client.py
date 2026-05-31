import os
import json
import yaml
import asyncio
import httpx


class DeepSeekClient:
    def __init__(self):
        self.api_key = os.getenv("DEEPSEEK_API_KEY")
        self.base_url = os.getenv("DEEPSEEK_BASE_URL", "http://localhost:1234")

        # переключатель эндпоинтов LM Studio (true = /api/v1/chat, false = /v1/responses)
        self.use_new_api_prefix = os.getenv("USE_NEW_API", "true").lower() == "true"

        if self.use_new_api_prefix:
            self.chat_url = f"{self.base_url}/api/v1/chat"
        else:
            self.chat_url = f"{self.base_url}/v1/responses"

        # включение mock-режима, если ключ/токен не заданы
        if self.api_key and not self.api_key.startswith("mock"):
            self.is_mock = False
        else:
            self.is_mock = True
            print(f"MOCK-РЕЖИМ. Запросы идут на эмуляцию LM Studio: {self.chat_url}")

    async def generate_review_as_yaml(self, system_prompt: str, user_prompt: str) -> str:
        """Отправка запроса на анализ в LM Studio университета"""
        if self.is_mock:
            await asyncio.sleep(1.5)
            mock_json_response = {
                "score": 6,
                "issues": [
                    {
                        "line": 14,
                        "severity": "critical",
                        "category": "architecture",
                        "message": "Критическая ошибка: Обнаружен прямой вызов базы данных из слоя Controller в обход UserService. Это нарушает правила изоляции слоев вашего YAML-стандарта.",
                        "suggestion": "return await self.user_service.get_all_users()"
                    }
                ]
            }
            return yaml.dump(mock_json_response, allow_unicode=True, sort_keys=False, default_flow_style=False)

        # формируем Payload по спецификации LM Studio REST API
        if self.use_new_api_prefix:
            #формат LM Studio /api/v1/chat
            payload = {
                "messages": [
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt}
                ],
                "model": os.getenv("LLM_MODEL_NAME", "qwen2.5-coder-7b-instruct"),
                "temperature": 0.2
            }
        else:
            # формат для /v1/responses
            payload = {
                "model": os.getenv("LLM_MODEL_NAME", "qwen2.5-coder-7b-instruct"),
                "messages": [
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt}
                ],
                "temperature": 0.2
            }

        headers = {"Authorization": f"Bearer {self.api_key}", "Content-Type": "application/json"}

        async with httpx.AsyncClient() as client:
            try:
                response = await client.post(self.chat_url, json=payload, headers=headers, timeout=30.0)
                response.raise_for_status()

                response_data = response.json()
                ai_content = response_data["choices"][0]["message"]["content"]

                # Парсим JSON от модели и переводим в YAML для Go-бэкенда
                json_dict = json.loads(ai_content)
                return yaml.dump(json_dict, allow_unicode=True, sort_keys=False, default_flow_style=False)

            except Exception as e:
                return yaml.dump({"status": "error", "message": f"LM Studio API Error: {str(e)}"})
