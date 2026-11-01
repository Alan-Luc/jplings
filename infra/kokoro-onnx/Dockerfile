FROM python:3.12-slim

# pyopenjtalk (Japanese G2P for misaki) compiles from source at install time.
RUN apt-get update \
 && apt-get install -y --no-install-recommends build-essential cmake \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
# misaki's Japanese G2P tokenizes with a unidic dictionary. unidic-lite is a reduced
# dictionary that mis-segments geminated digraphs (はっぴゃく became "haʔpʲi ja"),
# producing the wrong word, so the full dictionary is downloaded at build time.
RUN pip install --no-cache-dir -r requirements.txt \
 && python -m unidic download

COPY server.py .

EXPOSE 8880
CMD ["python", "-m", "uvicorn", "server:app", "--host", "0.0.0.0", "--port", "8880"]
