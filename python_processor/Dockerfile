FROM python:3.13-slim

WORKDIR /app

#  Копируем файл зависимостей
COPY requirements.txt .

RUN pip install --no-cache-dir -r requirements.txt

#  Копируем весь остальной код
COPY . .

EXPOSE 8080

#  Команда для запуска сервера при старте контейнера
CMD ["python", "main.py"]
