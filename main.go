package main

import (
    "fmt"
    "os"
    "bufio"
    "strings"
    "net/http"
    "net/url"
    "errors"
    "flag"
)

var csvfilename string
var mode string

func noRedirect(req *http.Request, via []*http.Request) error {
        return errors.New("Don't redirect!")
}

func init(){
    flag.StringVar(&csvfilename, "c", "", "Please provide the csv filename")
    flag.StringVar(&mode, "m", "status", "Mode for processing. Either status or redirect")
}


func main(){
    client := &http.Client{
        CheckRedirect: noRedirect,
    }

    if csvfilename == "" {
        fmt.Println("Usage: urlstatus -c filename.csv -m mode(status or redirect)", )
    }

    if f, err := os.Open(csvfilename); err == nil {
        scanner := bufio.NewScanner(f)

        for scanner.Scan() {
            var txt = scanner.Text()
            csv := strings.Split(txt, ",")

            switch mode {
            case "redirect":
                if redir,isredir := getRedirect(client, csv[1]); isredir {
                    fmt.Println(csv[0] + "," + redir)
                } else {
                    fmt.Println(csv[0] + "," + csv[1])
                }
                break
            case "status":
                stat := getStatus(client, csv[0])

                fmt.Sprintf("%v, %v", stat, csv[0])
            }

            

        }
    }
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
    srcURL,_ := url.Parse(src)

    req, _ := http.NewRequest("GET", src, nil)

    resp, _ := client.Do(req)
    if resp != nil {
        if resp.StatusCode == 301 || resp.StatusCode == 302  {
            redir := resp.Header["Location"][0]
            destURL,_ := url.Parse(redir)

            if !destURL.IsAbs() {
                return srcURL.Scheme + "://" + srcURL.Hostname() + redir, true
            } else {
                return destURL.String(), true
            }
            
        }
    }

    return "", false
}