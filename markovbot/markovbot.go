package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"

	"github.com/mediocregopher/lever"
	"github.com/nlopes/slack"
)

var addr string

func main() {

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
	l.Parse()

	apiToken, ok := l.ParamStr("-token")
	if !ok {
		log.Fatal("-token is required")
	}

	addr, _ = l.ParamStr("-addr")

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

	for event := range ch {
		switch e := event.Data.(type) {
		case *slack.MessageEvent:
			log.Printf("%s [@%s] %q", e.ChannelId, e.UserId, e.Text)
			url := fmt.Sprintf("%s/build?chainName=%s", addr, e.ChannelId)
			_, err := http.DefaultClient.Post(url, "", bytes.NewBufferString(e.Text))
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
