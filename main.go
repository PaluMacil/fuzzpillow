package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gordonklaus/portaudio"
)

const (
	sampleRate   = 44100
	noteLength   = 2   // length of each note in seconds
	vibratoRate  = 5   // The speed of the vibrato effect, in oscillations per second
	vibratoDepth = 0.1 // The depth (i.e., the amount of pitch variation) of the vibrato effect
)

// NoiseType represents different types of noise.
type NoiseType int

const (
	White NoiseType = iota
	Brown
	Pink
	Minor
)

func (n NoiseType) String() string {
	switch n {
	case White:
		return "White"
	case Brown:
		return "Brown"
	case Pink:
		return "Pink"
	case Minor:
		return "Minor"
	default:
		return "Unknown"
	}
}

func ParseNoiseType(s string) (NoiseType, error) {
	switch strings.ToLower(s) {
	case "white":
		return White, nil
	case "brown":
		return Brown, nil
	case "pink":
		return Pink, nil
	case "minor":
		return Minor, nil
	default:
		return White, fmt.Errorf("invalid noise type: %s", s)
	}
}

func main() {
	playCommand := flag.NewFlagSet("play", flag.ExitOnError)
	duration := playCommand.String("duration", "", "Duration to play the noise (default: until interrupted, format: 10s, 2m, 3h)")
	noiseType := playCommand.String("type", "white", "Type of noise to play (options: white, brown, pink)")

	if len(os.Args) < 2 {
		fmt.Println("Expected 'play' or 'list' commands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "play":
		playCommand.Parse(os.Args[2:])
		parsedNoiseType, err := ParseNoiseType(*noiseType)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		playNoise(*duration, parsedNoiseType)
	case "list":
		listDevices()
	case "help":
		printHelp()
	default:
		fmt.Println("Expected 'play' or 'list' commands")
		os.Exit(1)
	}
}

func playNoise(duration string, noiseType NoiseType) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)

	numSamples := sampleRate * 60 // one minute of noise
	samples := make([]float32, numSamples)
	rand.Seed(time.Now().UnixNano())

	// Code that generates the samples
	switch noiseType {
	case White:
		samples = generateWhiteNoise(samples)
	case Brown:
		samples = generateBrownNoise(samples)
	case Pink:
		samples = generatePinkNoise(samples)
	case Minor:
		samples = generateMinorNoise(samples, numSamples)
	}

	// Initialize portaudio
	err := portaudio.Initialize()
	if err != nil {
		fmt.Printf("Could not initialize portaudio: %v", err)
		os.Exit(1)
	}
	defer portaudio.Terminate()

	// Open default audio stream
	out := make([]float32, len(samples))
	stream, err := portaudio.OpenDefaultStream(0, 1, float64(sampleRate), len(samples), &out)
	if err != nil {
		fmt.Printf("Could not open default stream: %v", err)
		os.Exit(1)
	}
	defer stream.Close()

	stream.Start()
	defer stream.Stop()

	stop := make(chan bool)

	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				copy(out, samples)
				stream.Write()
			}
		}
	}()

	// If a duration has been set, send a stop signal after that many minutes
	if duration != "" {
		parsedDuration, err := time.ParseDuration(duration)
		if err != nil {
			fmt.Printf("Could not parse duration: %v", err)
			os.Exit(1)
		}
		go func() {
			time.Sleep(parsedDuration)
			stop <- true
		}()
	}

	// Exit immediately when receiving an interrupt signal
	<-signalChannel
	os.Exit(0)
}

func generateWhiteNoise(samples []float32) []float32 {
	for i := range samples {
		randomValue := rand.Float64()*2 - 1 // between -1 and 1
		samples[i] = float32(randomValue)
	}
	return samples
}

func generateBrownNoise(samples []float32) []float32 {
	var lastValue float64
	for i := range samples {
		randomValue := rand.NormFloat64()
		currentValue := (lastValue + (0.02 * randomValue)) / 1.02
		samples[i] = float32(currentValue)
		lastValue = currentValue
	}
	return samples
}

func generatePinkNoise(samples []float32) []float32 {
	b := [16]float64{}
	for i := range b {
		b[i] = rand.Float64()*2 - 1 // Random value between -1 and 1
	}
	for i := range samples {
		k := rand.Intn(len(b))
		output := b[k]
		b[k] = rand.Float64()*2 - 1 // Random value between -1 and 1
		output -= b[k]
		samples[i] = float32(output)
	}
	return samples
}

func generateMinorNoise(samples []float32, numSamples int) []float32 {
	notes := make([]float64, 16)
	for i := range notes {
		octave := 2 + rand.Intn(3) // 2nd tof 4th octave
		step := rand.Intn(7)       // step within the octave
		notes[i] = 440 * math.Pow(2, float64(octave+step)/12)
	}

	noteSamples := noteLength * sampleRate

	for i := 0; i < numSamples-noteSamples; i += noteSamples {
		omega := notes[rand.Intn(len(notes))] * 2 * math.Pi / float64(sampleRate)

		for j := 0; j < noteSamples; j++ {
			vibrato := 1 + vibratoDepth*math.Sin(2*math.Pi*vibratoRate*float64(j)/float64(sampleRate)) // Add a vibrato effect
			t := float64(j) / float64(noteSamples)
			amplitude := math.Sin(t * math.Pi) // Fade in and out over the duration of the note
			samples[i+j] = float32(amplitude * vibrato * math.Sin(float64(j)*omega))
		}
	}
	return samples
}

func listDevices() {
	// List devices using PortAudio
	devices, err := portaudio.Devices()
	if err != nil {
		fmt.Printf("Could not list devices: %v", err)
		os.Exit(1)
	}
	for i, dev := range devices {
		fmt.Printf("ID: %d, Name: %s, MaxInputChannels: %d, MaxOutputChannels: %d, DefaultSampleRate: %f\n", i, dev.Name, dev.MaxInputChannels, dev.MaxOutputChannels, dev.DefaultSampleRate)
	}
}

func printHelp() {
	fmt.Println("play: Play noise. Options: --duration [int], --type [white, brown, pink]")
	fmt.Println("list: List audio devices. No options.")
	fmt.Println("help: Show this help message.")
}
