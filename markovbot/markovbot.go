package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/mediocregopher/lever"
	"github.com/nlopes/slack"
)

var addr string

func main() {

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
	api := slack.New(apiToken)

	log.Print("connecting to RTM")
	apiWS, err := api.StartRTM("", "http://localhost/")
	if err != nil {
		log.Fatal(err)
	}

	ch := make(chan slack.SlackEvent)
	go func() {
		apiWS.HandleIncomingEvents(ch)
	}()

	quietCount := map[string]int{}

	for event := range ch {
		switch e := event.Data.(type) {
		case *slack.MessageEvent:
			log.Printf("%s [@%s] %q", e.ChannelId, e.UserId, e.Text)
			url := fmt.Sprintf("%s/build?chainName=%s", addr, e.ChannelId)
			_, err := http.Post(url, "", bytes.NewBufferString(e.Text))
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("quiet count is %d/%d", quietCount[e.ChannelId], interjectWait)
			randN := rand.Intn(interjectWait)
			log.Printf("randN is %d", randN)
			if (quietCount[e.ChannelId] >= interjectWait && randN == 0) ||
				strings.HasPrefix(strings.ToLower(e.Text), "markov") {
				responseText, err := generate(e.ChannelId)
				if err != nil {
					log.Fatal(err)
				}
				response := apiWS.NewOutgoingMessage(responseText, e.ChannelId)
				if err := apiWS.SendMessage(response); err != nil {
					log.Fatal(err)
				}
				quietCount[e.ChannelId] = 0
			} else {
				quietCount[e.ChannelId]++
			}
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
