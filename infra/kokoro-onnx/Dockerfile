FROM python:3.12-slim

# pyopenjtalk (Japanese G2P for misaki) compiles from source at install time.
RUN apt-get update \
 && apt-get install -y --no-install-recommends build-essential cmake \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
# misaki's Japanese G2P needs a unidic dictionary. The full `unidic` package ships
# without one and fails at runtime, while unidic-lite bundles it, so drop the former.
RUN pip install --no-cache-dir -r requirements.txt \
 && pip uninstall -y unidic \
 && pip install --no-cache-dir unidic-lite

COPY server.py .

EXPOSE 8880
CMD ["python", "-m", "uvicorn", "server:app", "--host", "0.0.0.0", "--port", "8880"]
