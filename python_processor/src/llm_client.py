import os
import json
import yaml
import asyncio
from openai import AsyncOpenAI


class DeepSeekClient:
    def __init__(self):
        self.api_key = os.getenv("DEEPSEEK_API_KEY")
        self.base_url = os.getenv("DEEPSEEK_BASE_URL", "https://deepseek.com")
        self.model = os.getenv("LLM_MODEL_NAME", "deepseek-coder")

        # Если ключ есть и он не похож на заглушку, инициализируем реальный клиент
        if self.api_key and not self.api_key.startswith("mock") and self.api_key != "your_key_here":
            self.client = AsyncOpenAI(api_key=self.api_key, base_url=self.base_url)
            self.is_mock = False
        else:
            self.client = None
            self.is_mock = True
            print("Инициализирован MOCK-РЕЖИМ для LLM. Запросы к API отправляться не будут.")

    async def generate_review_as_yaml(self, system_prompt: str, user_prompt: str) -> str:
        """Основной пайплайн ревью кода с поддержкой заглушки"""
        if self.is_mock:
            # Имитируем небольшую задержку сети (1.5 секунды)
            await asyncio.sleep(1.5)

            # Заготовка идеального ответа для демонстрации
            mock_json_response = {
                "score": 6,
                "issues": [
                    {
                        "line": 14,
                        "severity": "critical",
                        "category": "architecture",
                        "message": "Критическая ошибка архитектуры: Обнаружен прямой вызов базы данных из слоя Controller в обход UserService. Это нарушает правила изоляции слоев вашего YAML-стандарта.",
                        "suggestion": "# Вместо: db.execute('SELECT * FROM users')\n# Используйте:\nreturn await self.user_service.get_all_users()"
                    },
                    {
                        "line": 28,
                        "severity": "warning",
                        "category": "security",
                        "message": "Потенциальная уязвимость: Входной параметр 'user_id' встраивается в строку запроса. Это может привести к SQL-инъекции. Требуется обязательная валидация входящих данных.",
                        "suggestion": "# Вместо: f'SELECT * FROM items WHERE id = {user_id}'\n# Используйте плейсхолдеры:\ncursor.execute('SELECT * FROM items WHERE id = %s', (user_id,))"
                    }
                ]
            }
            return yaml.dump(mock_json_response, allow_unicode=True, sort_keys=False, default_flow_style=False)

        # Логика реального запроса к LLM (если ключ предоставлен)
        try:
            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_prompt}
                ],
                response_format={"type": "json_object"},
                temperature=0.2
            )
            json_data = json.loads(response.choices.message.content)
            return yaml.dump(json_data, allow_unicode=True, sort_keys=False, default_flow_style=False)
        except Exception as e:
            return yaml.dump({"status": "error", "message": f"Pipeline error: {str(e)}"})

    async def convert_text_to_yaml_config(self, user_text_description: str) -> str:
        """Заглушка для генерации конфигурации из сырого текста"""
        if self.is_mock:
            await asyncio.sleep(1.0)
            with open("config/standard.yaml", "r", encoding="utf-8") as f:
                return f.read()

        # Реальный запрос к LLM
        from python_processor.src.prompt_builder import PromptBuilder
        prompt = PromptBuilder.build_text_to_yaml_prompt(user_text_description)
        try:
            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                temperature=0.1
            )
            return response.choices.message.content.strip()
        except Exception as e:
            return f"Error creating config: {str(e)}"
