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
	"github.com/mediocregopher/radix/v3"

	_ "net/http/pprof"
)

// Prefix is a Markov chain prefix of one or more words.
type Prefix []string

// String returns the Prefix as a string (for use as a map key).
func (p Prefix) String() string {
	return strings.Join(p, " ")
}

// Shift returns a copy of the Prefix with the first word removed and the given
// one appended.
func (p Prefix) Shift(word string) Prefix {
	p2 := make(Prefix, len(p))
	copy(p2, p[1:])
	p2[len(p2)-1] = word
	return p2
}

var p *radix.Pool

func prefixKey(chain string, prefix Prefix) string {
	return fmt.Sprintf("markov:%s:%s", chain, prefix.String())
}

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

	p, err = radix.NewPool("tcp", redisAddr, 10)
	if err != nil {
		log.Fatal(err)
	}

	prefixLen, _ := l.ParamInt("-prefixLen")
	timeout, _ := l.ParamInt("-timeout")
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

		prefix := make(Prefix, prefixLen)
		ts := time.Now().Unix()
		for _, suffix := range suffixes {
			key := prefixKey(r.FormValue("chainName"), prefix)
			if err := p.Do(radix.FlatCmd(nil, "ZADD", key, ts, suffix)); err != nil {
				log.Fatal(err)
			}
			prefix = prefix.Shift(suffix)
		}
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

		// for tracking if a prefix has been used already or not.
		prefixM := map[string]bool{}

		prefix := make(Prefix, prefixLen)
		var words []string
		for {
			key := prefixKey(r.FormValue("chainName"), prefix)
			var suffixes []string
			if err := p.Do(radix.Cmd(&suffixes, "ZRANGE", key, "0", "-1")); err != nil {
				log.Fatal(err)
			} else if len(suffixes) == 0 {
				break
			}

			// try each possible suffix (randomly) trying to find one that
			// generates a prefix which hasn't been used already. If none do
			// then break.
			var next string
			var ok bool
			for _, i := range rand.Perm(len(suffixes)) {
				next = suffixes[i]
				words = append(words, next)
				newPrefix := prefix.Shift(next)
				newPrefixStr := newPrefix.String()
				if prefixM[newPrefixStr] {
					continue
				}

				prefixM[newPrefixStr] = true
				prefix = newPrefix
				ok = true
				break
			}

			if !ok {
				break
			} else if len(next) == 0 {
				continue
			}

			lastChar := next[len(next)-1]

			if lastChar == '!' ||
				lastChar == '?' ||
				lastChar == '.' ||
				(numParts == 1 &&
					(lastChar == ',' ||
						lastChar == ':' ||
						lastChar == ';')) {
				numParts--
			}

			switch lastChar {
			case '!', '?', '.', ',', ':', ';':
				numParts--
			}

			if numParts <= 0 {
				break
			}
		}

		fmt.Fprintln(w, strings.Join(words, " "))
	})

	listenAddr, _ := l.ParamStr("-listenAddr")
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func clydeTheCleaner(timeout int64) {
	tick := time.Tick(30 * time.Second)
	for {
		expire := time.Now().Unix() - (timeout * 3600)
		scanner := radix.NewScanner(p, radix.ScanOpts{
			Command: "SCAN",
			Pattern: "markov:*",
		})
		var key string
		for scanner.Next(&key) {
			if err := p.Do(radix.FlatCmd(nil, "ZREMRANGEBYSCORE", key, "0", expire)); err != nil {
				log.Fatal(err)
			}
		}
		if err := scanner.Close(); err != nil {
			log.Fatal(err)
		}

		<-tick
	}
}
