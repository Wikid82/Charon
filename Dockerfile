# Generic multi-stage Dockerfile for a Python web backend (FastAPI example).
# Adapt this to your chosen stack (Go, Node, etc.) as needed.

# ---- Builder ----
FROM python:3.12-slim AS builder
WORKDIR /app

# Install build dependencies
RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential gcc libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy only dependency files first to leverage cache
COPY requirements.txt requirements.dev.txt ./
RUN pip install --upgrade pip
RUN pip install --no-cache-dir -r requirements.txt

# Copy source
COPY . .

# ---- Final image ----
FROM python:3.12-slim
WORKDIR /app

# Copy installed packages from builder
COPY --from=builder /usr/local/lib/python3.12 /usr/local/lib/python3.12
COPY --from=builder /usr/local/bin /usr/local/bin

# Copy application code
COPY --from=builder /app /app

ENV PYTHONUNBUFFERED=1

# Expose default port (change if needed)
EXPOSE 8000

# Default command - update to your actual app entrypoint
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
