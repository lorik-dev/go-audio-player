package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/gopxl/beep/wav"

	wavmeta "github.com/go-audio/wav"
)

type Ctrl struct {
	fileName      string
	fileExtension string
	metadata      tag.Metadata
	Streamer      beep.StreamSeekCloser
	Format        beep.Format
	Paused        bool
	Loop          bool
}

type metadata struct {
	nonWavMetadata tag.Metadata
	WavMetadata    *wavmeta.Decoder
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func returnPosition(playInstance Ctrl) (int, int, int) {
	//Returns current position in track. Returns three separate ints: hours, minutes, seconds.
	position := playInstance.Streamer.Position()
	timeInSeconds := float64(position) / float64(playInstance.Format.SampleRate)

	//Convert time in seconds to "HH:MM:SS" format
	duration := time.Duration(timeInSeconds) * time.Second
	return int(duration.Hours()), int(duration.Minutes()) % 60, int(duration.Seconds()) % 60
}

// Prints current playback, including time and options
func printPlaybackStatus(playInstance Ctrl, metadataInstance metadata) {
	// Clear terminal
	fmt.Print("\033[H\033[2J")

	switch playInstance.fileExtension {
	case ".wav":
		fmt.Printf("%s\n%s\n", metadataInstance.WavMetadata.Metadata.Title, metadataInstance.WavMetadata.Metadata.Artist)
		fmt.Printf("WAV %02dKHz/%02dbit", metadataInstance.WavMetadata.SampleRate/1000, metadataInstance.WavMetadata.BitDepth)
	default:
		fmt.Printf("%s\n%s\n", metadataInstance.nonWavMetadata.Title(), metadataInstance.nonWavMetadata.Composer())
		fmt.Printf("%s %02dKHz\n", metadataInstance.nonWavMetadata.FileType(), playInstance.Format.SampleRate/1000)
	}
	fmt.Print("\n\n")

	// Retrieve playback position and print
	hours, minutes, seconds := returnPosition(playInstance)
	fmt.Printf("Time: %02d:%02d:%02d\n", hours, minutes, seconds)

	// Display options
	fmt.Println("1: Pause/Resume | 2: Loop")
	printPlaybackOptions(playInstance)
}

func printPlaybackOptions(playInstance Ctrl) {
	fmt.Printf("\n")
	switch playInstance.Paused {
	case true:
		fmt.Printf("| PAUSED |\n")
	}
	switch playInstance.Loop {
	case true:
		fmt.Printf("| LOOP |")
	}
}

func readAudio(fileArg string) (*os.File, string, string, beep.StreamSeekCloser, beep.Format) {
	file, fileErr := os.Open(fileArg)
	check(fileErr)

	fileName := filepath.Base(fileArg)
	fileExtension := filepath.Ext(fileArg)

	var streamerIn beep.StreamSeekCloser
	var formatIn beep.Format
	var decodeErr error

	file, fileErr = os.Open(fileArg)

	switch fileExtension {
	case ".mp3":
		strm, frm, err := mp3.Decode(file)
		streamerIn, formatIn, decodeErr = strm, frm, err
	case ".flac":
		strm, frm, err := flac.Decode(file)
		streamerIn, formatIn, decodeErr = strm, frm, err
	case ".ogg":
		strm, frm, err := vorbis.Decode(file)
		streamerIn, formatIn, decodeErr = strm, frm, err
	case ".wav":
		strm, frm, err := wav.Decode(file)
		streamerIn, formatIn, decodeErr = strm, frm, err
	default:
		panic("FATAL: Invalid file type.")
	}

	check(decodeErr)
	return file, fileExtension, fileName, streamerIn, formatIn
}

func readNonWavMetadata(fileRead os.File) tag.Metadata {
	file := os.File(fileRead)
	metadata, err := tag.ReadFrom(&file)
	check(err)

	return metadata
}

func playAudio(playInstance Ctrl, metadataInstance metadata) chan bool {
	speaker.Init(playInstance.Format.SampleRate, playInstance.Format.SampleRate.N(time.Second/10))
	printPlaybackStatus(playInstance, metadataInstance)
	done := make(chan bool)
	speaker.Play(beep.Seq(playInstance.Streamer, beep.Callback(func() {
		done <- true
	})))
	return done
}

func startPlayback(argsWithProg []string, preLoop bool) (Ctrl, metadata, chan bool) {
	file, fileExtension, fileNameRead, streamerRead, formatRead := readAudio(argsWithProg[1])
	playInstance := new(Ctrl)
	metadataInstance := new(metadata)
	playInstance.fileName = fileNameRead
	playInstance.fileExtension = fileExtension
	playInstance.Streamer = streamerRead
	playInstance.Format = formatRead

	file, fileErr := os.Open(argsWithProg[1])
	check(fileErr)

	switch fileExtension {
	//Separate metadata handling for .wav as tag package doesn't support wav
	//Use go-audio wav package instead
	//Use metadataInstance to contain either
	case ".wav":
		metadataInstance.WavMetadata = wavmeta.NewDecoder(file)
		metadataInstance.WavMetadata.ReadMetadata()
		if metadataInstance.WavMetadata.Metadata == nil {
			panic("FATAL: Couldn't read song metadata")
		}
	default:
		metadataInstance.nonWavMetadata = readNonWavMetadata(*file)
	}

	playInstance.Paused = false

	// Checks if to keep looping from previous instance
	playInstance.Loop = preLoop
	return *playInstance, *metadataInstance, playAudio(*playInstance, *metadataInstance)
}

func readInput(playInstance Ctrl, metadataInstance metadata, format beep.Format, done chan bool, argsWithProg []string) Ctrl {
	ch := make(chan string)
	go func(ch chan string) {
		reader := bufio.NewReader(os.Stdin)
		for {
			s, err := reader.ReadString('\n')
			if err != nil { // Maybe log non io.EOF errors, if you want
				close(ch)
				return
			}
			ch <- s
		}
	}(ch)
stdinloop:
	for {
		select {
		case stdin, ok := <-ch:
			if !ok {
				break stdinloop
			} else {
				if strings.TrimSpace(stdin) == "1" {
					switch playInstance.Paused {
					case false:
						speaker.Lock()
						playInstance.Paused = true
					case true:
						speaker.Unlock()
						playInstance.Paused = false
					}
					printPlaybackStatus(playInstance, metadataInstance)
				} else {
					printPlaybackStatus(playInstance, metadataInstance)
				}
				if strings.TrimSpace(stdin) == "2" {
					switch playInstance.Loop {
					case false:
						playInstance.Loop = true
					case true:
						playInstance.Loop = false
					}
					printPlaybackStatus(playInstance, metadataInstance)
				} else {
					printPlaybackStatus(playInstance, metadataInstance)
				}
			}
		case <-time.After(1 * time.Second):
			select {
			case ok := <-done:
				if ok {
					// Return updated playInstance for possible next instance if looping
					return playInstance
				} else {
					log.Fatal("Channel closed.")
				}
			default:
				if !playInstance.Paused {
					printPlaybackStatus(playInstance, metadataInstance)
				}
			}

		}
	}
	return playInstance
}

func playbackLoop(argsWithProg []string, preLoop bool) Ctrl {
	playInstance, metadataInstance, done := startPlayback(argsWithProg, preLoop)

	defer playInstance.Streamer.Close()
	for {
		select {
		case <-done:
		default:
			switch playInstance.Paused {
			case false:
				playInstance = readInput(playInstance, metadataInstance, playInstance.Format, done, argsWithProg)
				return playInstance
			case true:
				playInstance = readInput(playInstance, metadataInstance, playInstance.Format, done, argsWithProg)
				return playInstance
			default:
			}
		}
	}
}

func main() {
	argsWithProg := os.Args

	if len(argsWithProg) == 1 || len(argsWithProg) > 2 {
		log.Fatal("Improper arguments: ./go-music-player.exe [file]")
	}

	playInstance := playbackLoop(argsWithProg, false)
	for playInstance.Loop {
		playInstance = playbackLoop(argsWithProg, true)
	}
}
