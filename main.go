package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mediocregopher/lever"
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/util"
)

// Prefix is a Markov chain prefix of one or more words.
type Prefix []string

// String returns the Prefix as a string (for use as a map key).
func (p Prefix) String() string {
	return strings.Join(p, " ")
}

// Shift removes the first word from the Prefix and appends the given word.
func (p Prefix) Shift(word string) {
	copy(p, p[1:])
	p[len(p)-1] = word
}

var p *pool.Pool

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	log.SetFlags(log.Llongfile)

	l := lever.New("markov", nil)
	l.Add(lever.Param{
		Name:        "-prefixLen",
		Default:     "2",
		Description: "Prefix length for the markov chain algorithm",
	})
	l.Add(lever.Param{
		Name:        "-listenAddr",
		Default:     ":8080",
		Description: "Address to listen for calls to the http interface on",
	})
	l.Add(lever.Param{
		Name:        "-redisAddr",
		Default:     "127.0.0.1:6379",
		Description: "Address for an instance of redis",
	})
	l.Add(lever.Param{
		Name:        "-timeout",
		Default:     "720",
		Description: "Hours a suffix is allowed to stay untouched before it is cleaned up",
	})
	l.Parse()

	redisAddr, _ := l.ParamStr("-redisAddr")
	var err error
	p, err = pool.New("tcp", redisAddr, 10)
	if err != nil {
		log.Fatal(err)
	}

	prefixLen, _ := l.ParamInt("-prefixLen")
	timeout, _ := l.ParamInt("-timeout")
	go bobTheBuilder(prefixLen)
	go clydeTheCleaner(int64(timeout))

	http.HandleFunc("/build", func(w http.ResponseWriter, r *http.Request) {
		var suffixes []string
		for {
			var s string
			if _, err := fmt.Fscan(r.Body, &s); err != nil {
				break
			}
			suffixes = append(suffixes, strings.TrimSpace(s))
		}
		buildCh <- suffixes
	})

	http.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {
		numPartsStr := r.FormValue("numParts")
		if numPartsStr == "" {
			http.Error(w, "numParts argument must be specified", 400)
			return
		}

		numParts, err := strconv.Atoi(numPartsStr)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid value of numParts: %s", err), 400)
			return
		}

		prefix := make(Prefix, prefixLen)
		var words []string
		for {
			key := fmt.Sprintf("markov:%s", prefix.String())
			suffixes, err := p.Cmd("ZRANGE", key, 0, -1).List()
			if err != nil {
				log.Fatal(err)
			}
			if len(suffixes) == 0 {
				break
			}
			i := rand.Intn(len(suffixes))
			next := suffixes[i]
			words = append(words, next)
			prefix.Shift(next)

			if len(next) == 0 {
				continue
			}

			lastChar := next[len(next)-1]
			switch lastChar {
			case '!', '?', '.', ',', ':', ';':
				numParts--
			}

			if numParts == 0 {
				break
			}
		}

		fmt.Fprintln(w, strings.Join(words, " "))
	})

	listenAddr, _ := l.ParamStr("-listenAddr")
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

var buildCh = make(chan []string)

func bobTheBuilder(prefixLen int) {
	for suffixes := range buildCh {
		prefix := make(Prefix, prefixLen)
		ts := time.Now().UTC().Unix()
		for _, suffix := range suffixes {
			key := fmt.Sprintf("markov:%s", prefix.String())
			if err := p.Cmd("ZADD", key, ts, suffix).Err; err != nil {
				log.Fatal(err)
			}
			prefix.Shift(suffix)
		}
	}
}

func clydeTheCleaner(timeout int64) {
	tick := time.Tick(30 * time.Second)
	for {
		ch := make(chan string)
		go func() {
			err := util.Scan(p, ch, "SCAN", "", "markov:*")
			if err != nil {
				log.Fatal(err)
			}
		}()
		expire := time.Now().UTC().Unix() - (timeout * 3600)
		for key := range ch {
			if err2 := p.Cmd("ZREMRANGEBYSCORE", key, 0, expire).Err; err2 != nil {
				log.Fatal(err2)
			}
		}

		<-tick
	}
}
