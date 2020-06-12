package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func noRedirect(req *http.Request, via []*http.Request) error {
	return errors.New("no redirect")
}

type Line struct {
	URL    string
	Result string
}

type Mode int

const (
	Status Mode = iota
	Redirect
)

func main() {
	base := flag.String("b", "", "base domain name for when list is relative urls")
	csvFilename := flag.String("c", "", "csv filename")
	modestr := flag.String("m", "status", "processing mode. (status or redirect)")
	flag.Parse()

	mode := Status
	if *modestr == "redirect" {
		mode = Redirect
	}

	if *csvFilename == "" {
		fmt.Println("Usage: urlstatus -c filename.csv -m mode(status or redirect)")
		flag.PrintDefaults()
		return
	}

	f, err := os.Open(*csvFilename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	csvreader := csv.NewReader(f)
	lines, err := csvreader.ReadAll()
	if err != nil {
		log.Fatalf("reading file %s %v", *csvFilename, err)
	}

	urls := []Line{}
	for _, line := range lines {
		u := Line{URL: line[0], Result: line[1]}
		urls = append(urls, u)
	}

	results := process(*base, urls, mode)

	for _, r := range results {
		fmt.Println(r)
	}
}

func process(baseUrl string, lines []Line, mode Mode) []string {
	client := &http.Client{
		CheckRedirect: noRedirect,
	}

	results := []string{}
	switch mode {
	case Redirect:
		results = append(results, "RESULT, FULL URL, ACTUAL, EXPECTED")
	case Status:
		results = append(results, "RESULT, FULL URL")
	}

	for _, line := range lines {
		test := line.URL
		if baseUrl != "" {
			test = baseUrl + test
		}
		switch mode {
		case Redirect:
			if redir, isredir := getRedirect(client, test); isredir {
				if !strings.HasSuffix(redir, line.Result) {
					results = append(results, fmt.Sprintf("FAILURE, %s, %s, %s", test, redir, line.Result))
					//fmt.Printf("Wrong: For %s expected %s got %s\n", test, line.Result, redir)
				} else {
					results = append(results, fmt.Sprintf("SUCCESS, %s, %s, %s", test, redir, line.Result))
				}
			}
		case Status:
			status := getStatus(client, test)
			results = append(results, fmt.Sprintf("%d, %s", status, test))
		}
	}
	return results
}

func getStatus(client *http.Client, src string) int {
	req, _ := http.NewRequest("GET", src, nil)
	resp, _ := client.Do(req)
	if resp != nil {
		return resp.StatusCode
	}

	return -1
}

func getRedirect(client *http.Client, src string) (string, bool) {
	srcURL, _ := url.Parse(src)

	req, _ := http.NewRequest("GET", src, nil)

	resp, _ := client.Do(req)
	if resp != nil {
		if resp.StatusCode == 301 || resp.StatusCode == 302 {
			redir := resp.Header["Location"][0]
			destURL, _ := url.Parse(redir)

			if !destURL.IsAbs() {
				return srcURL.Scheme + "://" + srcURL.Hostname() + redir, true
			} else {
				return destURL.String(), true
			}

		}
	}

	return "", false
}
