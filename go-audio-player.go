package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
)

type Ctrl struct {
	fileName string
	Streamer beep.StreamSeekCloser
	Format   beep.Format
	Paused   bool
	Loop     bool
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
func printPlaybackStatus(playInstance Ctrl) {
	// Clear terminal
	fmt.Print("\033[H\033[2J")
	fmt.Println(playInstance.fileName + "\n")
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

func readAudio(fileArg string) (string, beep.StreamSeekCloser, beep.Format) {
	file, fileErr := os.Open(fileArg)
	check(fileErr)

	fileName := filepath.Base(fileArg)
	fileExtension := filepath.Ext(fileArg)

	var streamerIn beep.StreamSeekCloser
	var formatIn beep.Format
	var decodeErr error

	switch fileExtension {
	case ".mp3":
		strm, frm, err := mp3.Decode(file)
		streamerIn, formatIn, decodeErr = strm, frm, err
	case ".flac":
		strm, frm, err := flac.Decode(file)
		streamerIn, formatIn, decodeErr = strm, frm, err
	default:
		log.Fatal("Invalid file type.")
	}

	check(decodeErr)
	return fileName, streamerIn, formatIn
}

func playAudio(playInstance Ctrl) chan bool {

	speaker.Init(playInstance.Format.SampleRate, playInstance.Format.SampleRate.N(time.Second/10))
	done := make(chan bool)
	printPlaybackStatus(playInstance)
	speaker.Play(beep.Seq(playInstance.Streamer, beep.Callback(func() {
		done <- true
	})))
	return done
}

func startPlayback(argsWithProg []string, preLoop bool) (Ctrl, chan bool) {
	fileNameRead, streamerRead, formatRead := readAudio(argsWithProg[1])
	playInstance := new(Ctrl)
	playInstance.fileName = fileNameRead
	playInstance.Streamer = streamerRead
	playInstance.Format = formatRead

	// Checks if to keep looping from previous instance
	playInstance.Paused = false

	playInstance.Loop = preLoop
	return *playInstance, playAudio(*playInstance)
}

func readInput(playInstance Ctrl, format beep.Format, done chan bool, argsWithProg []string) Ctrl {
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
					printPlaybackStatus(playInstance)
				} else {
					printPlaybackStatus(playInstance)
				}
				if strings.TrimSpace(stdin) == "2" {
					switch playInstance.Loop {
					case false:
						playInstance.Loop = true
					case true:
						playInstance.Loop = false
					}
					printPlaybackStatus(playInstance)
				} else {
					printPlaybackStatus(playInstance)
				}
			}
		case <-time.After(1 * time.Second):
			// Do something when there is nothing read from stdin
			select {
			case ok := <-done:
				if ok {
					// Return updated playInstance
					return playInstance
				} else {
					log.Fatal("Channel closed.")
				}
			default:
				if !playInstance.Paused {
					printPlaybackStatus(playInstance)
				}
			}

		}
	}
	return playInstance
}

func playbackLoop(argsWithProg []string, preLoop bool) Ctrl {
	playInstance, done := startPlayback(argsWithProg, preLoop)

	defer playInstance.Streamer.Close()
	for {
		select {
		case <-done:
		default:
			switch playInstance.Paused {
			case false:
				playInstance = readInput(playInstance, playInstance.Format, done, argsWithProg)
				return playInstance
			case true:
				playInstance = readInput(playInstance, playInstance.Format, done, argsWithProg)
				return playInstance
			default:
			}
		}
	}
}

func main() {
	argsWithProg := os.Args
	//argsWithoutProg := os.Args[1:]

	if len(argsWithProg) == 1 || len(argsWithProg) > 2 {
		log.Fatal("Improper arguments: ./go-music-player.exe [file]")
	}
	playInstance := playbackLoop(argsWithProg, false)
	for playInstance.Loop {
		playInstance = playbackLoop(argsWithProg, true)
	}
}
