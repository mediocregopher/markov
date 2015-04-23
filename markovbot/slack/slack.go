// Package slack exists because the existing slack library sucks and I just want
// a simply thing to hit the real-time api
package slack

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"golang.org/x/net/websocket"
)

// SlackAPI is the api endpoint for slack
var SlackAPI = "https://slack.com/api/"

var msgIDCh = make(chan int)

func init() {
	go func() {
		i := 0
		for {
			msgIDCh <- i
			i++
		}
	}()
}

// Message is an auxiliary type to allow us to have a message containing sub messages
type Message struct {
	Msg
	SubMessage Msg `json:"message,omitempty"`
}

// Msg contains information about a slack message
type Msg struct {
	Id        int    `json:"id"`
	BotId     string `json:"bot_id,omitempty"`
	UserId    string `json:"user,omitempty"`
	Username  string `json:"username,omitempty"`
	ChannelID string `json:"channel,omitempty"`
	Timestamp string `json:"ts,omitempty"`
	Text      string `json:"text,omitempty"`
	Team      string `json:"team,omitempty"`
	//File      File   `json:"file,omitempty"`
	// Type may come if it's part of a message list
	// e.g.: channel.history
	Type      string `json:"type,omitempty"`
	IsStarred bool   `json:"is_starred,omitempty"`
	// Submessage
	SubType          string `json:"subtype,omitempty"`
	Hidden           bool   `json:"bool,omitempty"`
	DeletedTimestamp string `json:"deleted_ts,omitempty"`
	//Attachments      []Attachment `json:"attachments,omitempty"`
	ReplyTo int  `json:"reply_to,omitempty"`
	Upload  bool `json:"upload,omitempty"`
}

// WS is a websocket connection to slack
type WS struct {
	*websocket.Conn
}

// NewWS returns a websocket connected to the slack api
func NewWS(token string) (*WS, error) {
	u := SlackAPI + "rtm.start"
	values := url.Values{"token": {token}}
	resp, err := http.PostForm(u, values)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{}
	if err = json.Unmarshal(body, &m); err != nil {
		return nil, err
	}

	wsURL := m["url"].(string)
	if wsURL == "" {
		return nil, errors.New("no ws url returned from slack")
	}

	ws, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		return nil, err
	}

	return &WS{ws}, nil
}

// Read reads a single message off the api and returns it
func (w *WS) Read() (*Message, error) {
	var m Message
	if err := websocket.JSON.Receive(w.Conn, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Sends sends the given channel a chat message
func (w *WS) Send(channelID, text string) error {
	m := Msg{
		Id:        <-msgIDCh,
		Type:      "message",
		ChannelID: channelID,
		Text:      text,
	}
	return websocket.JSON.Send(w.Conn, &m)
}

// Ping will ping the server to ensure it's still there. This doesn't wait for
// the pong response, that will come through a call to Read later. It doesn't
// really matter if it never comes in, this is mostly to make sure the
// connection is still open
func (w *WS) Ping() error {
	m := Msg{
		Id:   <-msgIDCh,
		Type: "ping",
	}
	return websocket.JSON.Send(w.Conn, &m)
}
