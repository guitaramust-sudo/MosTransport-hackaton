"""Render the original, softly looping music used inside the 3D wagon.

Requires numpy and ffmpeg when regenerating; playback only needs the MP3 asset.
"""
from pathlib import Path
import subprocess
import tempfile
import wave

import numpy as np

SAMPLE_RATE = 44_100
BAR_SECONDS = 3.0
BARS = 16
DURATION = BAR_SECONDS * BARS
SAMPLES = round(SAMPLE_RATE * DURATION)
track = np.zeros(SAMPLES, dtype=np.float32)


def hz(midi: int) -> float:
    return 440.0 * 2 ** ((midi - 69) / 12)


def add_note(midi: int, start: float, length: float, level: float, kind: str) -> None:
    count = round(length * SAMPLE_RATE)
    t = np.arange(count, dtype=np.float32) / SAMPLE_RATE
    freq = hz(midi)
    phase = 2 * np.pi * freq * t
    if kind == "pad":
        attack = np.minimum(1, t / 0.38)
        release = np.minimum(1, (length - t) / 0.55)
        envelope = attack * release
        tone = np.sin(phase) + 0.19 * np.sin(2 * phase) + 0.05 * np.sin(3 * phase)
        tone *= 0.92 + 0.08 * np.sin(2 * np.pi * 0.24 * t)
    elif kind == "bell":
        envelope = np.minimum(1, t / 0.012) * np.exp(-2.2 * t)
        tone = np.sin(phase) + 0.22 * np.sin(2.01 * phase) + 0.09 * np.sin(3.97 * phase)
    else:  # felt-key arpeggio
        envelope = np.minimum(1, t / 0.025) * np.exp(-1.9 * t)
        tone = np.sin(phase) + 0.14 * np.sin(2 * phase) + 0.04 * np.sin(3 * phase)
    indices = (round(start * SAMPLE_RATE) + np.arange(count)) % SAMPLES
    np.add.at(track, indices, (level * envelope * tone).astype(np.float32))


# Cmaj9, Am7, Fmaj9, G6. The four-chord passage repeats with small melody changes.
chords = [
    ([48, 55, 59, 62, 64], [60, 64, 67, 71], [76, 74, 71, 79]),
    ([45, 52, 55, 60, 64], [57, 60, 64, 67], [76, 72, 69, 72]),
    ([41, 48, 52, 55, 57], [57, 60, 64, 69], [72, 76, 74, 69]),
    ([43, 50, 52, 59, 62], [59, 62, 67, 71], [74, 71, 67, 74]),
]
for bar in range(BARS):
    start = bar * BAR_SECONDS
    pad, arp, melody = chords[bar % 4]
    for pitch in pad:
        add_note(pitch, start, BAR_SECONDS + 0.55, 0.030 if pitch < 54 else 0.021, "pad")
    for step in range(4):
        add_note(arp[(step + bar // 4) % 4], start + step * 0.75, 1.8, 0.030, "key")
    if bar % 2 == 0:
        add_note(melody[(bar // 2) % 4], start + 1.5, 2.25, 0.012, "bell")

# Subtle room reflections soften the synth without adding noise.
original = track.copy()
for delay, gain in ((0.21, 0.10), (0.37, 0.065), (0.62, 0.035)):
    track += np.roll(original, round(delay * SAMPLE_RATE)) * gain
track = np.tanh(track * 1.6) / 1.6
track *= 0.68 / max(1.0, float(np.max(np.abs(track))) / 0.68)

output = Path(__file__).resolve().parents[1] / "assets/audio/wagon_ambient.mp3"
output.parent.mkdir(parents=True, exist_ok=True)
with tempfile.TemporaryDirectory() as temp:
    wav_path = Path(temp) / "wagon_ambient.wav"
    with wave.open(str(wav_path), "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(SAMPLE_RATE)
        wav.writeframes((np.clip(track, -1, 1) * 32767).astype("<i2").tobytes())
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(wav_path), "-codec:a", "libmp3lame", "-b:a", "112k", str(output)], check=True)
print(output)
