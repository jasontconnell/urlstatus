package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func noRedirect(req *http.Request, via []*http.Request) error {
	return errors.New("no redirect")
}

type UrlLine struct {
	URL    string
	Result string
	Index  int
}

type Result struct {
	Text  string
	Index int
}

type Mode int

const (
	Status Mode = iota
	Redirect
)

func modeHeader(mode Mode) string {
	h := "Status Code, Url"
	if mode == Redirect {
		h = "Result, Url, Location, Expected"
	}
	return h
}

func main() {
	base := flag.String("b", "", "base domain name for when list is relative urls")
	csvFilename := flag.String("c", "", "csv filename")
	modestr := flag.String("m", "status", "processing mode. (status or redirect)")
	output := flag.String("o", "stdout", "output file or stdout")
	batchsize := flag.Int("batch", 15, "batch size")
	flag.Parse()

	start := time.Now()

	mode := Status
	if *modestr == "redirect" {
		mode = Redirect
	}

	if *csvFilename == "" {
		fmt.Println("Usage: urlstatus -c filename.csv -m mode(status or redirect) -o outputfile(optional, defaults to stdout) -batch batchsize(defaults to 15 per batch)")
		flag.PrintDefaults()
		return
	}

	urls, err := readCSV(*csvFilename)
	if err != nil {
		log.Fatalf("couldn't read file %s: %s", *csvFilename, err.Error())
	}

	log.Println("Processing", len(urls), "urls in batches of", *batchsize)
	results := process(*base, urls, mode, *batchsize)

	w := os.Stdout
	if *output != "stdout" {
		f, err := os.OpenFile(*output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
		if err != nil {
			fmt.Println("couldn't open file, writing to stdout instead")
		} else {
			w = f
			defer f.Close()
		}
	}

	fmt.Fprintln(w, modeHeader(mode))
	for _, r := range results {
		fmt.Fprintln(w, r.Text)
	}

	fmt.Println("\n\nFinished.", len(urls), "processed.", time.Since(start))
}

func readCSV(filename string) ([]UrlLine, error) {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	csvreader := csv.NewReader(f)
	lines, err := csvreader.ReadAll()
	if err != nil {
		return nil, err
	}

	urls := []UrlLine{}
	for i, line := range lines {
		u := UrlLine{Index: i, URL: line[0]}
		if len(line) > 1 {
			u.Result = line[1]
		}
		urls = append(urls, u)
	}
	return urls, nil
}

func process(baseUrl string, list []UrlLine, mode Mode, batchsize int) []Result {
	client := &http.Client{
		CheckRedirect: noRedirect,
	}

	rchan := make(chan Result, len(list))

	var wg sync.WaitGroup
	pools := len(list)/batchsize + 1
	wg.Add(pools)

	for i := 0; i < pools; i++ {
		start := i * batchsize
		end := (i + 1) * batchsize
		if end > len(list) {
			end = len(list)
		}
		go func(chunk []UrlLine, ch chan Result) {
			for _, line := range chunk {
				result := processOne(client, baseUrl, line, mode)
				ch <- Result{Text: result, Index: line.Index}
			}
			wg.Done()
		}(list[start:end], rchan)
	}

	start := time.Now()
	go func(ch chan Result) {
		for {
			select {
			case <-time.After(3 * time.Second):
				fmt.Printf("\r\tprocessed: %d. time: %v", len(ch), time.Since(start))
			}
		}
	}(rchan)

	wg.Wait()
	close(rchan)

	var results []Result
	for s := range rchan {
		results = append(results, s)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Index < results[j].Index
	})
	return results
}

func processOne(client *http.Client, baseUrl string, line UrlLine, mode Mode) string {
	var result string
	test := baseUrl + line.URL
	status, redir := getResult(client, test)
	switch mode {
	case Redirect:
		resstr := "SUCCESS"
		if !strings.HasSuffix(redir, line.Result) {
			resstr = "FAILURE"
		}
		result = fmt.Sprintf("%s, %s, %s, %s", resstr, test, redir, line.Result)
	case Status:
		result = fmt.Sprintf("%d, %s", status, test)
	}
	return result
}

func getResult(client *http.Client, url string) (int, string) {
	req, _ := http.NewRequest("GET", url, nil)
	resp, _ := client.Do(req)
	if resp != nil {
		var redir string
		if resp.StatusCode > 300 && resp.StatusCode < 308 {
			redir = resp.Header.Get("Location")
		}
		return resp.StatusCode, redir
	}

	return -1, ""
}
