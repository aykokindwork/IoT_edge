# Используем ту же мажорную версию, что и у автора модели
FROM dockerhub.timeweb.cloud/library/python:3.12-slim

WORKDIR /app

# Системные зависимости
RUN apt-get update && apt-get install -y build-essential python3-dev

# Копируем и устанавливаем зависимости
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Копируем весь код и модели
COPY . .

EXPOSE 50051

CMD ["python", "server.py"]