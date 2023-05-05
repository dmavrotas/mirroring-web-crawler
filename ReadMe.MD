### wget mirror crawler

This is a simple bash script to crawl a website and download all the files in it. It is useful for downloading a website for offline viewing.

#### Usage

    go run wget-mirror <url> <destination>

#### Example

    go run wget-mirror https://developer.mozilla.org/en-US/docs/Web/HTML destination

### Tech 

Cobra is used for the CLI. `PuerkitoBio/goquery` is used for searching inside the HTML pages. Testify is used for testing and assertions.

### Installation

Make sure you have go 1.20 installed. Then just do :
    
        go get -d ./...

