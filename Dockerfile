FROM python:3.12-slim

WORKDIR /app

COPY app.py .

ENV PYTHONUNBUFFERED=1
ENV TARGET_URL=http://platform-proxy:3000
ENV CHECK_INTERVAL_SECONDS=3
ENV REQUEST_TIMEOUT_SECONDS=2
ENV NOTIFY_URL=http://sound-notifier:8765/notify

CMD ["python", "/app/app.py", "monitor"]
