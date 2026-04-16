FROM python:3.12-slim

WORKDIR /app

# Install system dependencies needed for some Python packages
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies first (cache layer)
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application source
COPY . .

# Non-root user for security
RUN adduser --disabled-password --gecos "" goldagent && \
    chown -R goldagent:goldagent /app
USER goldagent

EXPOSE 8080

CMD ["python", "main.py"]
