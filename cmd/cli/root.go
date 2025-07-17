package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/benleem/snarfl/internal/scrape"
)

const OUTPUT_DEFAULT = "./goscrape_data.json"

func exec() {
	outFlag := flag.String("o", OUTPUT_DEFAULT, "file to output data")
	flag.Parse()
	args := flag.Args()
	var scrapeUrl string
	isStdin, err := checkStdin()
	if err != nil {
		errorToExit(err)
	}
	if isStdin {
		scrapeUrl = readStdin()
	} else {
		if len(args) == 0 || len(args) > 1 {
			errorToExit(fmt.Errorf("incorrect format"))
		}
		scrapeUrl = args[0]
	}
	scraper, err := scrape.NewScraper(scrapeUrl, outFlag)
	if err != nil {
		errorToExit(err)
	}
	scraper.Crawl()
}

func checkStdin() (bool, error) {
	fileStat, err := os.Stdin.Stat()
	if err != nil {
		return false, fmt.Errorf("getting stdin stat failed: %v", err)
	}
	if fileStat.Size() == 0 {
		return false, nil
	}
	return true, nil
}

func readStdin() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	text := scanner.Text()
	return text
}

func errorToExit(err error) {
	fmt.Println("---")
	fmt.Printf("ERROR: %s\n", err)
	fmt.Println("---")
	fmt.Println("usage:")
	fmt.Println("goscrape -[options] https://someurl.com")
	// fmt.Println("---")
	os.Exit(1)
}
