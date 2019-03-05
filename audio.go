package main

import (
	"encoding/base64"
	"log"
	"time"

	"github.com/gordonklaus/portaudio"
	opus "gopkg.in/hraban/opus.v2"
)

const (
	BLOCK_SIZE     = 480  // 480 == 60ms at 8kHz/8bit, for Opus encoding
	FRAME_SIZE_MS  = 60   // 60ms
	SAMPLE_RATE    = 8000 // kHz
	CHANNELS       = 1    // mono
	OPUS_BIT_RATE  = 16000
	OPUS_BUFF_SIZE = 1024
)

type Audio struct {
	stream *portaudio.Stream

	opusEncoder *opus.Encoder
	opusDecoder *opus.Decoder

	opusDecodeBuf []byte
	opusEncodeBuf []byte
	// pcmBuff       []int16
	silence []int16

	inChan, outChan chan []byte
}

func InitAudio(ingoing, outgoing chan []byte) (*Audio, error) {
	a := Audio{}
	a.inChan = ingoing
	a.outChan = outgoing
	a.opusEncodeBuf = make([]byte, OPUS_BUFF_SIZE)
	// a.pcmBuff = make([]int16, BLOCK_SIZE)
	a.silence = make([]int16, BLOCK_SIZE)
	for i, _ := range a.silence {
		a.silence[i] = 128
	}

	err := portaudio.Initialize()
	if err != nil {
		return nil, err
	}

	a.opusEncoder, err = opus.NewEncoder(SAMPLE_RATE, 1, opus.AppVoIP)
	if err != nil {
		return nil, err
	}
	err = a.opusEncoder.SetBitrate(OPUS_BIT_RATE)
	if err != nil {
		return nil, err
	}

	a.opusDecoder, err = opus.NewDecoder(SAMPLE_RATE, 1)
	if err != nil {
		return nil, err
	}

	a.stream, err = portaudio.OpenDefaultStream(
		1, 1, SAMPLE_RATE, BLOCK_SIZE, a.CallBack,
	)
	if err != nil {
		return nil, err
	}

	if err = a.stream.Start(); err != nil {
		return nil, err
	}

	return &a, nil
}

func (a *Audio) Close() {
	a.stream.Stop()
	a.stream.Close()
	portaudio.Terminate()
}

func (a *Audio) CallBack(inBuf, outBuf []int16) {
	// Player
	select {
	case a.opusDecodeBuf = <-a.inChan:
		_, err := a.opusDecoder.Decode(a.opusDecodeBuf, outBuf)
		if err != nil {
			log.Println(err)
		}

	case <-time.After(2 * time.Millisecond):
		copy(outBuf, a.silence)
	}

	// Recorder
	n, err := a.opusEncoder.Encode(inBuf, a.opusEncodeBuf)
	if err != nil {
		log.Println(err)
		return
	}
	a.outChan <- a.opusEncodeBuf[:n]
}

// DecodePayload converts base64 to []byte
func DecodePayload(payload []byte) ([]byte, error) {
	buf := make([]byte, base64.StdEncoding.DecodedLen(len(payload)))
	n, err := base64.StdEncoding.Decode(buf, payload)
	return buf[:n], err
	//return base64.StdEncoding.DecodeString(string(payload))
}

// EncodePayload retruns the base64 encoding of input
func EncodePayload(chunk []byte) []byte {
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(chunk)))
	base64.StdEncoding.Encode(buf, chunk)
	return buf
}
