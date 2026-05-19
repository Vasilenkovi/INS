from python_processor.src.config_loader import CompleteUniversityStandard


class PromptBuilder:
    @staticmethod
    def build_system_prompt(standard: CompleteUniversityStandard) -> str:
        cfg = standard.llm_review

        instruction = (
            f"Ты — Senior Software Architect. Проведи ревью кода для проекта '{standard.project.name}' "
            f"на языке {standard.project.language}.\n"
            f"Используй только следующие уровни критичности: {', '.join(standard.severity_levels)}.\n\n"
        )

        if cfg.ignore_formatting:
            instruction += (
                "- ПОЛНОСТЬЮ ИГНОРИРУЙ стиль кодирования, пробелы, табы, длину функций и PEP8.\n"
                "- Сосредоточься на архитектуре, логических ошибках и уязвимостях.\n"
            )

        if cfg.require_fix_suggestions:
            instruction += "- Для каждого замечания обязательно добавь поле 'suggestion' с примером исправленного кода.\n"

        instruction += (
            f"\nОтвет верни строго в формате JSON, соответствующем типу {cfg.response_format.type}:\n"
            "{\n"
            "  'score': 10,\n"
            "  'issues': [\n"
            "    {\n"
            "      'line': 15,\n"
            "      'severity': 'critical|warning|info',\n"
            "      'category': 'architecture|security|business_logic',\n"
            "      'message': 'Текст замечания.',\n"
            "      'suggestion': 'Пример исправления'\n"
            "    }\n"
            "  ]\n"
            "}"
        )
        return instruction

    @staticmethod
    def build_user_prompt(code: str, file_path: str, standard: CompleteUniversityStandard) -> str:
        priorities = ", ".join([f"{k} (вес: {v})" for k, v in standard.review_priorities.items()])
        focus_areas = ", ".join(standard.llm_review.focus_on)

        arch_rules = ""
        for dep in standard.architecture.forbidden_dependencies:
            arch_rules += f"- Слою [{dep.from_layer}] запрещено зависеть от [{dep.to_layer}]. Причина: {dep.reason}\n"

        anti_patterns = ""
        for ap in standard.anti_patterns:
            pattern_info = f" (Паттерн: {ap.pattern})" if ap.pattern else ""
            anti_patterns += f"- ID: {ap.id}. Описание: {ap.description}{pattern_info}. Дефолтный severity: {ap.severity}\n"

        user_prompt = f"Файл для анализа: `{file_path}`\n"
        user_prompt += f"Приоритеты анализа: {priorities}\n"
        user_prompt += f"Основные фокусы модели: {focus_areas}\n\n"
        user_prompt += f"--- АРХИТЕКТУРНЫЕ ОГРАНИЧЕНИЯ ---\n{arch_rules}\n"
        user_prompt += f"--- ЗАПРЕЩЕННЫЕ КОНСТРУКЦИИ И ТРЕБОВАНИЯ БЕЗОПАСНОСТИ ---\n"
        user_prompt += f"- Запрещено использовать: {', '.join(standard.security.forbid)}\n"
        user_prompt += f"- Обязательно к реализации: {', '.join(standard.security.require)}\n\n"
        user_prompt += f"--- ИЗВЕСТНЫЕ АНТИ-ПАТТЕРНЫ ---\n{anti_patterns}\n"
        user_prompt += f"КОД ДЛЯ РЕВЬЮ:\n```\n{code}\n```"

        return user_prompt

    @staticmethod
    def build_text_to_yaml_prompt(user_text: str) -> str:
        """Промт для превращения произвольного текста в валидный YAML стандарта"""
        return (
            "Преврати следующее текстовое описание требований к коду в валидный YAML файл, "
            "строго соответствующий спецификации.\n\n"
            "Структура выходного YAML должна быть ТОЧНО такой:\n"
            "version: 1.0\n"
            "project:\n"
            "  name: string\n"
            "  language: string\n"
            "severity_levels: [critical, warning, info]\n"
            "review_priorities: {architecture: 0.4, security: 0.4, readability: 0.2}\n"
            "architecture:\n"
            "  layers: [string]\n"
            "  forbidden_dependencies: [{from: string, to: string, reason: string}]\n"
            "security:\n"
            "  forbid: [string]\n"
            "  require: [string]\n"
            "anti_patterns: [{id: string, description: string, pattern: string, severity: string}]\n"
            "llm_review:\n"
            "  ignore_formatting: true\n"
            "  focus_on: [architecture, vulnerabilities, business_logic]\n"
            "  require_fix_suggestions: true\n"
            "  response_format: {type: json}\n\n"
            f"Текст пользователя:\n\"{user_text}\"\n\n"
            "Верни СТРОГО чистый YAML код, без markdown-разметки (без ```yaml) и без лишних пояснений."
        )
