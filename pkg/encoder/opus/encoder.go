package opus

import (
	"fmt"
	"gopkg.in/hraban/opus.v2"
)

type Encoder struct {
	*opus.Encoder

	buffer          Buffer
	channels        int
	inFrequency     int
	outFrequency    int
	resampleBufSize int
	resampler       Resampler
}

func NewEncoder(inputSampleRate, outputSampleRate, channels int, options ...func(*Encoder) error) (Encoder, error) {
	encoder, err := opus.NewEncoder(
		outputSampleRate,
		channels,
		// be aware that low delay option is not optimized for voice
		opus.AppRestrictedLowdelay,
	)
	if err != nil {
		return Encoder{}, err
	}
	enc := &Encoder{
		Encoder:      encoder,
		buffer:       Buffer{Data: make([]int16, inputSampleRate*20/1000*channels)},
		channels:     channels,
		inFrequency:  inputSampleRate,
		outFrequency: outputSampleRate,
		resampler:    &Giongto35LinearResampler{},
		//resampler: &SoxResampler{
		//},
	}

	_ = enc.SetMaxBandwidth(opus.Fullband)
	_ = enc.SetBitrate(192000)
	_ = enc.SetComplexity(10)

	for _, option := range options {
		err := option(enc)
		if err != nil {
			return Encoder{}, err
		}
	}
	return *enc, nil
}

func SampleBuffer(ms int, resampling bool) func(*Encoder) error {
	return func(e *Encoder) (err error) {
		e.buffer = Buffer{Data: make([]int16, e.inFrequency*ms/1000*e.channels)}
		if resampling {
			e.resampleBufSize = e.outFrequency * ms / 1000 * e.channels
			//err := e.resampler.Init(e.inFrequency, e.outFrequency)
			//if err != nil {
			//	return err
			//}
		}
		return
	}
}

func (e *Encoder) BufferWrite(samples []int16) (written int) { return e.buffer.Write(samples) }

func (e *Encoder) BufferEncode() ([]byte, error) { return e.Encode(e.buffer.Data) }

func (e *Encoder) BufferFull() bool { return e.buffer.Full() }

func (e *Encoder) Encode(pcm []int16) ([]byte, error) {
	if e.resampleBufSize > 0 {
		err := e.resampler.Init(e.inFrequency, e.outFrequency)
		if err != nil {
			return nil, err
		}
		pcm = e.resampler.Resample(pcm, e.resampleBufSize)
		err = e.resampler.Close()
		if err != nil {
			return nil, err
		}
	}

	if len(pcm) < e.resampleBufSize {
		gap := make([]int16, e.resampleBufSize-len(pcm))
		for i := range gap {
			gap[i] = pcm[len(pcm)-1]
		}
		pcm = append(pcm, gap...)
	}

	data := make([]byte, 1024)
	n, err := e.Encoder.Encode(pcm, data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (e *Encoder) GetInfo() string {
	bitrate, _ := e.Encoder.Bitrate()
	complexity, _ := e.Encoder.Complexity()
	dtx, _ := e.Encoder.DTX()
	fec, _ := e.Encoder.InBandFEC()
	maxBandwidth, _ := e.Encoder.MaxBandwidth()
	lossPercent, _ := e.Encoder.PacketLossPerc()
	sampleRate, _ := e.Encoder.SampleRate()
	return fmt.Sprintf(
		"Bitrate: %v bps, Complexity: %v, DTX: %v, FEC: %v, Max bandwidth: *%v, Loss%%: %v, Rate: %v Hz",
		bitrate, complexity, dtx, fec, maxBandwidth, lossPercent, sampleRate,
	)
}

// Close cleanups external resources for encoders.
// ! OPUS lib wrapper doesn't have proper close function.
func (e *Encoder) Close() {
	_ = e.resampler.Close()
}
