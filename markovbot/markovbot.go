package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mediocregopher/lever"
	"github.com/mediocregopher/markov/markovbot/slack"
)

var addr string

func main() {
	log.SetFlags(log.Lshortfile)
	rand.Seed(time.Now().UTC().UnixNano())

	l := lever.New("markovbot", nil)
	l.Add(lever.Param{
		Name:        "-token",
		Description: "API token for the slack bot",
	})
	l.Add(lever.Param{
		Name:        "-addr",
		Description: "Address the markov api is listening on",
		Default:     "http://127.0.0.1:8080",
	})
	l.Add(lever.Param{
		Name:        "-interject-wait",
		Description: "Minimum number of messages to wait before randomly interjecting in a channel",
		Default:     "100",
	})
	l.Parse()

	apiToken, ok := l.ParamStr("-token")
	if !ok {
		log.Fatal("-token is required")
	}

	addr, _ = l.ParamStr("-addr")

	interjectWait, _ := l.ParamInt("-interject-wait")
	if interjectWait <= 0 {
		log.Fatal("-interject-wait must be a number greater than 0")
	}

	log.Printf("connecting with %s", apiToken)
	ws, err := slack.NewWS(apiToken)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("connected")

	quietCount := map[string]int{}
	pingTick := time.Tick(5 * time.Second)

	for {
		ws.SetReadDeadline(time.Now().Add(5 * time.Second))

		select {
		case <-pingTick:
			if err := ws.Ping(); err != nil {
				log.Fatal(err)
			}
		default:
		}

		m, err := ws.Read()
		if nerr, ok := err.(*net.OpError); ok && nerr.Timeout() {
			continue
		} else if err != nil {
			log.Fatal(err)
		}

		if m.Type != "message" {
			continue
		}

		log.Printf("%s [@%s] %q", m.ChannelID, m.UserId, m.Text)
		url := fmt.Sprintf("%s/build?chainName=%s", addr, m.ChannelID)
		_, err = http.Post(url, "", bytes.NewBufferString(m.Text))
		if err != nil {
			log.Fatal(err)
		}

		randN := rand.Intn(interjectWait)
		if (quietCount[m.ChannelID] >= interjectWait && randN == 0) ||
			strings.HasPrefix(strings.ToLower(m.Text), "markov") {
			response, err := generate(m.ChannelID)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("sendng %q", response)
			if err = ws.Send(m.ChannelID, response); err != nil {
				log.Fatal(err)
			}
			quietCount[m.ChannelID] = 0
		} else {
			quietCount[m.ChannelID]++
		}
	}
}

func generate(channelID string) (string, error) {
	url := fmt.Sprintf("%s/generate?numParts=1&chainName=%s", addr, channelID)
	r, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	all, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return string(all), nil
}
