package speech

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrNotPCM marks audio this app cannot analyse: the energy gate needs 16-bit PCM.
var ErrNotPCM = errors.New("audio is not 16-bit PCM WAV")

// HasSpeech reports whether a recording contains energy above the silence floor, and
// returns the measured RMS.
//
// This gate is deliberately independent of the recogniser. Measured on this machine,
// three seconds of digital silence produced "(音楽)" from Whisper at avg_logprob
// -0.001 — fluent, confident, entirely invented. Silence must therefore be detected
// from the waveform, never from the model's confidence.
func HasSpeech(wav []byte, floor float64) (bool, float64, error) {
	samples, err := pcm16Samples(wav)
	if err != nil {
		return false, 0, err
	}
	if len(samples) == 0 {
		return false, 0, nil
	}

	var sumSquares float64
	for _, sample := range samples {
		value := float64(sample) / 32768.0
		sumSquares += value * value
	}
	rms := math.Sqrt(sumSquares / float64(len(samples)))
	return rms >= floor, rms, nil
}

// SilenceFloor is the default RMS threshold under which a clip counts as silent.
// Conversational speech is far above this; room tone and digital silence are below.
const SilenceFloor = 0.005

// pcm16Samples extracts interleaved 16-bit samples from a RIFF/WAVE payload.
func pcm16Samples(wav []byte) ([]int16, error) {
	if len(wav) < 12 || string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, ErrNotPCM
	}

	var (
		audioFormat, bitsPerSample uint16
		data                       []byte
	)

	for offset := 12; offset+8 <= len(wav); {
		id := string(wav[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(wav[offset+4 : offset+8]))
		body := offset + 8
		end := body + size
		if end > len(wav) {
			end = len(wav)
		}

		switch id {
		case "fmt ":
			if size < 16 {
				return nil, ErrNotPCM
			}
			audioFormat = binary.LittleEndian.Uint16(wav[body : body+2])
			bitsPerSample = binary.LittleEndian.Uint16(wav[body+14 : body+16])
		case "data":
			data = wav[body:end]
		}

		offset = end
		if size%2 == 1 {
			offset++
		}
	}

	if audioFormat != 1 || bitsPerSample != 16 {
		return nil, fmt.Errorf("%w: format=%d bits=%d", ErrNotPCM, audioFormat, bitsPerSample)
	}
	if len(data) < 2 {
		return nil, nil
	}

	samples := make([]int16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		samples = append(samples, int16(binary.LittleEndian.Uint16(data[i:i+2])))
	}
	return samples, nil
}

// EncodeWAV builds a 16-bit mono WAV from int16 samples.
func EncodeWAV(samples []int16, sampleRate int) []byte {
	var buffer bytes.Buffer
	dataLen := len(samples) * 2

	buffer.WriteString("RIFF")
	binary.Write(&buffer, binary.LittleEndian, uint32(36+dataLen))
	buffer.WriteString("WAVE")

	buffer.WriteString("fmt ")
	binary.Write(&buffer, binary.LittleEndian, uint32(16))
	binary.Write(&buffer, binary.LittleEndian, uint16(1))
	binary.Write(&buffer, binary.LittleEndian, uint16(1))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buffer, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(&buffer, binary.LittleEndian, uint16(2))
	binary.Write(&buffer, binary.LittleEndian, uint16(16))

	buffer.WriteString("data")
	binary.Write(&buffer, binary.LittleEndian, uint32(dataLen))
	for _, sample := range samples {
		binary.Write(&buffer, binary.LittleEndian, sample)
	}
	return buffer.Bytes()
}
