package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mediocregopher/lever"
	"github.com/mediocregopher/markov/markovbot/slack"
	"github.com/mediocregopher/radix.v2/redis"
)

var addr string
var rconn *redis.Client
var interjectWait int

func quietCountKey(channelID string) string {
	return fmt.Sprintf("markovbot:quietCount:%s", channelID)
}

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
	l.Add(lever.Param{
		Name:        "-redis-addr",
		Description: "Address of redis instance to store some minor data in",
		Default:     "127.0.0.1:6379",
	})
	l.Parse()

	apiToken, ok := l.ParamStr("-token")
	if !ok {
		log.Fatal("-token is required")
	}

	addr, _ = l.ParamStr("-addr")

	interjectWait, _ = l.ParamInt("-interject-wait")
	if interjectWait <= 0 {
		log.Fatal("-interject-wait must be a number greater than 0")
	}

	redisAddr, _ := l.ParamStr("-redis-addr")
	log.Printf("connecting to redis at %s", redisAddr)

	var err error
	if rconn, err = redis.Dial("tcp", redisAddr); err != nil {
		log.Fatal(err)
	}

	log.Printf("connecting with %s", apiToken)
	ws, err := slack.NewWS(apiToken)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("connected")

	pingTick := time.Tick(5 * time.Second)

	for {
		ws.SetReadDeadline(time.Now().Add(5 * time.Second))

		select {
		case <-pingTick:
			if err := ws.Ping(); err != nil {
				log.Fatalf("error pinging slack websocket: %s", err)
			}
			if err := rconn.Cmd("PING").Err; err != nil {
				log.Fatalf("error pinging redis connection: %s", err)
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

		m.Text = cleanText(m.Text)

		// If the message is just a link, don't even bother
		if _, err := url.Parse(m.Text); err == nil {
			continue
		}

		log.Printf("%s [@%s] %q", m.ChannelID, m.UserId, m.Text)
		url := fmt.Sprintf("%s/build?chainName=%s", addr, m.ChannelID)
		_, err = http.Post(url, "", bytes.NewBufferString(m.Text))
		if err != nil {
			log.Fatal(err)
		}

		ok, err := shouldInterject(m)
		if err != nil {
			log.Fatal(err)
		} else if !ok {
			continue
		}

		var response string
		for {
			innerRes, err := generate(m.ChannelID)
			if err != nil {
				log.Fatal(err)
			}

			response += innerRes
			if len(response) >= 20 {
				break
			}

			switch response[len(response)-1] {
			case '.', '!', '?':
				response += " "
			default:
				response += ". "
			}
		}

		// Clean outgoing text too, in case there's any chain data left that has
		// the old, unclean links in it
		response = cleanText(response)

		log.Printf("sending %q", response)
		if err = ws.Send(m.ChannelID, response); err != nil {
			log.Fatal(err)
		}

		if err := rconn.Cmd("DEL", quietCountKey(m.ChannelID)).Err; err != nil {
			log.Fatal(err)
		}
	}
}

var linkRegex = regexp.MustCompile("<(https?://.+?)>")

func cleanText(text string) string {
	matches := linkRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		text = strings.Replace(text, match[0], match[1], -1)
	}
	return text
}

func shouldInterject(m *slack.Message) (bool, error) {
	quietCount, err := rconn.Cmd("INCR", quietCountKey(m.ChannelID)).Int()
	if err != nil {
		return false, err
	}

	randN := rand.Intn(interjectWait)
	if quietCount > interjectWait && randN == 0 {
		return true, nil
	}

	text := strings.ToLower(m.Text)
	if strings.HasPrefix(text, "markov") {
		return true, nil
	}

	if strings.HasPrefix(text, "@markov") {
		return true, nil
	}

	return false, nil
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
