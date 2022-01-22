package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/a-h/character"
	"github.com/a-h/debounce"
	"github.com/stianeikeland/go-rpio/v4"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/host"
)

type AutoGenerated struct {
	Device               Device  `json:"device"`
	ShuffleState         bool    `json:"shuffle_state"`
	RepeatState          string  `json:"repeat_state"`
	Timestamp            int64   `json:"timestamp"`
	Context              Context `json:"context"`
	ProgressMs           float64 `json:"progress_ms"`
	Item                 Item    `json:"item"`
	CurrentlyPlayingType string  `json:"currently_playing_type"`
	Actions              Actions `json:"actions"`
	IsPlaying            bool    `json:"is_playing"`
}
type Device struct {
	ID               string `json:"id"`
	IsActive         bool   `json:"is_active"`
	IsPrivateSession bool   `json:"is_private_session"`
	IsRestricted     bool   `json:"is_restricted"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	VolumePercent    int    `json:"volume_percent"`
}
type ExternalUrls struct {
	Spotify string `json:"spotify"`
}
type Context struct {
	ExternalUrls ExternalUrls `json:"external_urls"`
	Href         string       `json:"href"`
	Type         string       `json:"type"`
	URI          string       `json:"uri"`
}
type Artists struct {
	ExternalUrls ExternalUrls `json:"external_urls"`
	Href         string       `json:"href"`
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Type         string       `json:"type"`
	URI          string       `json:"uri"`
}
type Images struct {
	Height int    `json:"height"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
}
type Album struct {
	AlbumType            string       `json:"album_type"`
	Artists              []Artists    `json:"artists"`
	AvailableMarkets     []string     `json:"available_markets"`
	ExternalUrls         ExternalUrls `json:"external_urls"`
	Href                 string       `json:"href"`
	ID                   string       `json:"id"`
	Images               []Images     `json:"images"`
	Name                 string       `json:"name"`
	ReleaseDate          string       `json:"release_date"`
	ReleaseDatePrecision string       `json:"release_date_precision"`
	TotalTracks          int          `json:"total_tracks"`
	Type                 string       `json:"type"`
	URI                  string       `json:"uri"`
}
type ExternalIds struct {
	Isrc string `json:"isrc"`
}
type Item struct {
	Album            Album        `json:"album"`
	Artists          []Artists    `json:"artists"`
	AvailableMarkets []string     `json:"available_markets"`
	DiscNumber       int          `json:"disc_number"`
	DurationMs       float64      `json:"duration_ms"`
	Explicit         bool         `json:"explicit"`
	ExternalIds      ExternalIds  `json:"external_ids"`
	ExternalUrls     ExternalUrls `json:"external_urls"`
	Href             string       `json:"href"`
	ID               string       `json:"id"`
	IsLocal          bool         `json:"is_local"`
	Name             string       `json:"name"`
	Popularity       int          `json:"popularity"`
	PreviewURL       string       `json:"preview_url"`
	TrackNumber      int          `json:"track_number"`
	Type             string       `json:"type"`
	URI              string       `json:"uri"`
}
type Disallows struct {
	Resuming     bool `json:"resuming"`
	SkippingPrev bool `json:"skipping_prev"`
}
type Actions struct {
	Disallows Disallows `json:"disallows"`
}

func main() {
	// Use the periph library.
	_, err := host.Init()
	if err != nil {
		fmt.Printf("err: %v\n", err)
		os.Exit(1)
	}

	// Open up first i2c channel.
	// You'll need to enable i2c for your Raspberry Pi in
	// https://www.raspberrypi.org/documentation/configuration/raspi-config.md
	bus, err := i2creg.Open("")
	if err != nil {
		fmt.Printf("err: %v\n", err)
		os.Exit(1)
	}

	// The default address for the i2c backpack is 0x27.
	dev := &i2c.Dev{
		Bus:  bus,
		Addr: 0x27,
	}

	// Create a 2 line display.
	d := character.NewDisplay(dev, false)
	// Update the display every second.
	ticker := time.NewTicker(1 * time.Second)
	quit := make(chan struct{})
	offset := 0

	// Init GPIOs
	err = rpio.Open()
	if err != nil {
		log.Fatalln("Could not open GPIOs")
	}
	normallyClosed := false
	// https://www.quinapalus.com/hd44780udg.html
	chars := [8][8]byte{
		{0x8, 0xc, 0xe, 0xf, 0xe, 0xc, 0x8, 0x0}, // play
		{0x0, 0xa, 0xa, 0xa, 0xa, 0xa, 0x0, 0x0}, //pause
		{0x0, 0x0, 0xa, 0xa, 0xa, 0xa, 0x0, 0x0},
		{0xe, 0x1b, 0x11, 0x11, 0x11, 0x1f, 0x1f},
		{0xe, 0x1b, 0x11, 0x11, 0x1f, 0x1f, 0x1f},
		{0xe, 0x1b, 0x11, 0x1f, 0x1f, 0x1f, 0x1f},
		{0xe, 0x1b, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f},
		{0xe, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f, 0x1f},
	}
	d.LoadCustomChars(chars)

	// Play/Pause
	onClickPlayPause := func() {
		fmt.Println("Clicked Play/Pause.")
		playpause()
	}
	swPlayPause := debounce.Button(onClickPlayPause, normallyClosed)

	gpioPlayPause := rpio.Pin(23)
	rpio.PinMode(gpioPlayPause, rpio.Input)
	// Forward
	onClickForward := func() {
		log.Println("Clicked Forward.")
		forward()
	}
	swForward := debounce.Button(onClickForward, normallyClosed)

	gpioForward := rpio.Pin(24)
	rpio.PinMode(gpioForward, rpio.Input)

	// Backward

	onClickBackward := func() {
		log.Println("Clicked Backward.")
		back()
	}
	swBackward := debounce.Button(onClickBackward, normallyClosed)

	gpioBackward := rpio.Pin(25)
	rpio.PinMode(gpioBackward, rpio.Input)

	// Heart

	onClickHeart := func() {
		log.Println("Clicked Heart.")
		heart()
	}
	swHeart := debounce.Button(onClickHeart, normallyClosed)

	gpioHeart := rpio.Pin(16)
	rpio.PinMode(gpioHeart, rpio.Input)

	// Check GPIOs
	getgpio := func() {
		for {
			time.Sleep(3 * time.Millisecond)
			swPlayPause.SetState(rpio.ReadPin(gpioPlayPause) == rpio.High)
			swForward.SetState(rpio.ReadPin(gpioForward) == rpio.High)
			swBackward.SetState(rpio.ReadPin(gpioBackward) == rpio.High)
			swHeart.SetState(rpio.ReadPin(gpioHeart) == rpio.High)
		}
	}
	go getgpio()

	for {
		select {
		case <-ticker.C:
			status, err := exec.Command("spotify", "status", "--raw").Output()
			details := AutoGenerated{}
			reader := strings.NewReader(string(status))
			decode := json.NewDecoder(reader)
			err = decode.Decode(&details)

			if offset >= len(details.Item.Name)-13 {
				offset = 0
			}

			if err != nil {
				d.Goto(0, 0)
				d.Print("Cannot connect, ")
				d.Goto(1, 0)
				d.Print("Try play a song.")
			} else {
				d.Goto(0, 0)
				cut := substring(details.Item.Name, offset, 14)
				d.Print(pad(cut, 14))
				offset++
				progressbar(int((details.ProgressMs/details.Item.DurationMs)*100), d, !details.IsPlaying)
				d.Goto(0, 14)
				d.WriteData(0b01111100)
				if !details.IsPlaying {
					d.WriteData(0b00000001)
				} else {
					d.WriteData(0b00000000)
				}
			}
		case <-quit:
			ticker.Stop()
			return
		}
	}
}
func forward() {
	_, err := exec.Command("spotify", "next").Output()
	if err != nil {
		log.Println("Could not play next song.")
	}
}
func back() {
	_, err := exec.Command("spotify", "previous").Output()
	if err != nil {
		log.Println("Could not play previous song.")
	}

}
func playpause() {
	_, err := exec.Command("spotify", "toggle").Output()
	if err != nil {
		log.Println("Could not play / pause song.")
	}
}
func heart() {
	_, err := exec.Command("spotify", "save", "--track", ".", "-y").Output()
	if err != nil {
		log.Println("Could not heart / unheart song.")
	}
}

func pad(s string, length int) string {
	return fmt.Sprintf("%-*s", length, s)
}

func substring(s string, from, length int) string {
	if from > len(s) {
		return ""
	}
	to := from + length
	if to > len(s) {
		to = len(s)
	}
	return s[from:to]
}

func progressbar(prog int, d *character.Display, paused bool) {
	d.Goto(1, 0)
	if paused {
		s := ""
		for i := 0; i < 17; i++ {
			s = s + " "
		}
		d.Print(s)
		time.Sleep(480 * time.Millisecond)
	} else {
		for i := 0; i < 17; i++ {
			d.Print(" ")
			time.Sleep(30 * time.Millisecond)
		}
	}
	d.Goto(1, 0)
	if paused {
		for i := 0; i < (prog / 6); i++ {
			d.WriteData(0b11111111)
		}
		time.Sleep(480 * time.Millisecond)
	} else {
		for i := 0; i < (prog / 6); i++ {
			d.WriteData(0b11111111)
			time.Sleep(30 * time.Millisecond)
		}

	}

}
