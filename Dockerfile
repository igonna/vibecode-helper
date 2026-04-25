FROM python:3.12-slim

WORKDIR /app

COPY app.py .
COPY config.json .

ENV PYTHONUNBUFFERED=1

CMD ["python", "/app/app.py", "monitor"]
