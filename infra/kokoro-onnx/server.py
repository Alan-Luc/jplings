"""Minimal Kokoro ONNX TTS service.

Exposes the same OpenAI-shaped endpoint the rest of the app uses, so the speech
client never learns which TTS implementation is behind it (R4, D8).

kokoro-onnx builds its own InferenceSession without thread options, so the session
factory is patched before the model loads to make intra-op threads measurable.
"""

import io
import os
import time
import wave

import numpy as np
import onnxruntime as rt
from fastapi import FastAPI
from fastapi.responses import JSONResponse, Response
from pydantic import BaseModel

MODEL_PATH = os.environ.get("KOKORO_MODEL", "/models/kokoro-v1.0.onnx")
VOICES_PATH = os.environ.get("KOKORO_VOICES", "/models/voices-v1.0.bin")
INTRA_THREADS = int(os.environ.get("ORT_INTRA_OP_THREADS", "0"))

_original_session = rt.InferenceSession


def _session_with_threads(path, *args, **kwargs):
    options = kwargs.get("sess_options") or rt.SessionOptions()
    if INTRA_THREADS > 0:
        options.intra_op_num_threads = INTRA_THREADS
        options.inter_op_num_threads = 1
    kwargs["sess_options"] = options
    return _original_session(path, *args, **kwargs)


rt.InferenceSession = _session_with_threads

from kokoro_onnx import Kokoro  # noqa: E402  (patch must land first)
from misaki import ja as misaki_ja  # noqa: E402

kokoro = Kokoro(MODEL_PATH, VOICES_PATH)

# kokoro-onnx's own tokenizer is espeak-only, which mangles Japanese. misaki's
# Japanese G2P is built once here and reused, since loading it is the slow part.
_ja_g2p = misaki_ja.JAG2P()

app = FastAPI(title="Kokoro ONNX TTS")


class SpeechRequest(BaseModel):
    model: str = "kokoro"
    input: str
    voice: str = "jf_alpha"
    response_format: str = "wav"
    speed: float = 1.0


def _language(voice: str) -> str:
    # Kokoro voice names lead with language+gender: jf_/jm_ Japanese, af_/am_ English.
    return "ja" if voice[:1] == "j" else "en-us"


def _wav(samples: np.ndarray, sample_rate: int) -> bytes:
    buffer = io.BytesIO()
    with wave.open(buffer, "wb") as writer:
        writer.setnchannels(1)
        writer.setsampwidth(2)
        writer.setframerate(sample_rate)
        writer.writeframes((np.clip(samples, -1.0, 1.0) * 32767).astype("<i2").tobytes())
    return buffer.getvalue()


@app.get("/health")
def health():
    return JSONResponse(
        {
            "status": "ok",
            "model": os.path.basename(MODEL_PATH),
            "intra_op_threads": INTRA_THREADS or "default",
        }
    )


@app.post("/v1/audio/speech")
def speech(request: SpeechRequest):
    started = time.perf_counter()
    if _language(request.voice) == "ja":
        phonemes, _ = _ja_g2p(request.input)
        samples, sample_rate = kokoro.create(
            phonemes, voice=request.voice, speed=request.speed, is_phonemes=True
        )
    else:
        samples, sample_rate = kokoro.create(
            request.input, voice=request.voice, speed=request.speed, lang=_language(request.voice)
        )
    audio = _wav(samples, sample_rate)
    elapsed = time.perf_counter() - started
    return Response(
        content=audio,
        media_type="audio/wav",
        headers={
            "X-Generation-Seconds": f"{elapsed:.3f}",
            "X-Audio-Seconds": f"{len(samples) / sample_rate:.3f}",
        },
    )
