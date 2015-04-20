package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/mediocregopher/lever"
)

func main() {
	l := lever.New("markov", nil)
	l.Add(lever.Param{
		Name:        "-addr",
		Default:     "http://127.0.0.1:8080",
		Description: "Address markov api is listening on",
	})
	l.Parse()

	addr, _ := l.ParamStr("-addr")
	buf := bufio.NewReader(os.Stdin)

	for {
		sent, rerr := buf.ReadString('.')
		if rerr != nil && rerr != io.EOF {
			log.Fatal(rerr)
		}
		req, err := http.NewRequest("POST", addr+"/build", bytes.NewBufferString(sent))
		if err != nil {
			log.Fatal(err)
		}

		if _, err = http.DefaultClient.Do(req); err != nil {
			log.Fatal(err)
		}

		if rerr == io.EOF {
			return
		}
	}
}
