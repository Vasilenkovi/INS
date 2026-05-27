from pydantic import BaseModel

class AnalysisRequest(BaseModel):
    file_path: str      # Путь к файлу (нужен ИИ для определения слоя архитектуры)
    code_content: str   # Сам текст диффа/кода одного файла
    standard_yaml: str  # Актуальный текст стандарта, присланный Go из базы данных
