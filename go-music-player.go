package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
)

type Ctrl struct {
	Streamer beep.StreamSeekCloser
	Paused   bool
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func readAudio(fileArg string) (beep.StreamSeekCloser, beep.Format) {
	file, fileErr := os.Open(fileArg)
	check(fileErr)

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
	return streamerIn, formatIn
}

func playAudio(streamer beep.StreamSeekCloser, format beep.Format) (beep.StreamSeekCloser, chan bool) {

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan bool)
	fmt.Println("Playing...")
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))
	return streamer, done
}

func readInput(playInstance Ctrl, format beep.Format, done chan bool) {
	ch := make(chan string)
	go func(ch chan string) {
		reader := bufio.NewReader(os.Stdin)
		for {
			s, err := reader.ReadString('\n')
			if err != nil { // Maybe log non io.EOF errors, if you want
				fmt.Println("closing")
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
				fmt.Println(stdin)
				if stdin == "1" {
					switch playInstance.Paused {
					case false:
						speaker.Lock()
						playInstance.Paused = true
						fmt.Println("Paused")
					case true:
						speaker.Unlock()
						playInstance.Paused = false
						fmt.Println("Resume")
					}
				}
			}
		case <-time.After(1 * time.Second):
			// Do something when there is nothing read from stdin
			select {
			case ok := <-done:
				if ok {
					fmt.Println("Playback finished.")
					return
				} else {
					log.Fatal("Channel closed.")
				}
			default:
				fmt.Print("\033[H\033[2J")
				position := playInstance.Streamer.Position()
				timeInSeconds := float64(position) / float64(format.SampleRate)

				//Convert time in seconds to "HH:MM:SS" format
				duration := time.Duration(timeInSeconds) * time.Second
				hours := int(duration.Hours())
				minutes := int(duration.Minutes()) % 60
				seconds := int(duration.Seconds()) % 60
				fmt.Printf("Time: %02d:%02d:%02d\n", hours, minutes, seconds)
			}
		}
	}
	fmt.Println("Done, stdin must be closed")
}

func main() {
	argsWithProg := os.Args
	//argsWithoutProg := os.Args[1:]

	if len(argsWithProg) == 1 || len(argsWithProg) > 2 {
		log.Fatal("Improper arguments: ./go-music-player.exe [file]")
	}

	streamerRead, formatRead := readAudio(argsWithProg[1])
	streamer, done := playAudio(streamerRead, formatRead)
	playInstance := new(Ctrl)
	playInstance.Streamer = streamer
	playInstance.Paused = false

	defer playInstance.Streamer.Close()
	for {
		select {
		case <-done:
			fmt.Println("Playback finished")
			return
		default:
			switch playInstance.Paused {
			case false:
				//fmt.Printf("%d | Press 1 to pause at any time.", playInstance.Streamer.Position())
				/*reader := bufio.NewReader(os.Stdin)
				fmt.Print("-> ")

				text, _ := reader.ReadString('\n')
				text = strings.Replace(text, "\r\n", "", -1)
				if strings.Compare("1", text) == 0 {
					speaker.Lock()
					playInstance.Paused = true
				}*/
				readInput(*playInstance, formatRead, done)
				os.Exit(0)
			case true:
				/*fmt.Printf("%d | Press 1 to resume at any time.", playInstance.Streamer.Position())
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("-> ")

				text, _ := reader.ReadString('\n')
				text = strings.Replace(text, "\r\n", "", -1)
				if strings.Compare("1", text) == 0 {
					speaker.Unlock()
					playInstance.Paused = false
				}*/
				readInput(*playInstance, formatRead, done)
				os.Exit(0)
			default:
			}
		}
	}
}
