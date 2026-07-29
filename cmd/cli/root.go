package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/benleem/snarfl/internal/crawler"
)

const (
	OUTPUT_DEFAULT      = "./snarfl-data.json"
	DEPTH_DEFAULT       = 0
	HEADER_DEFAULT      = ""
	MIN_TIMEOUT_DEFAULT = 0.0
	MAX_TIMEOUT_DEFAULT = 3.0
)

func exec() {
	outFlag := flag.String("o", OUTPUT_DEFAULT, "file to output data")
	depthFlag := flag.Int("d", DEPTH_DEFAULT, "depth of links from init seed the crawler will follow")
	headerFlag := flag.String("h", HEADER_DEFAULT, "header information used in requests")
	minTimeFlag := flag.Float64("min", MIN_TIMEOUT_DEFAULT, "min time for timeout before next request")
	maxTimeFlag := flag.Float64("max", MAX_TIMEOUT_DEFAULT, "max time for timeout before next request")
	flag.Parse()
	args := flag.Args()
	var crawlUrl string
	isStdin, err := checkStdin()
	if err != nil {
		errorToExit(err)
	}
	if isStdin {
		fmt.Printf("output file: %s, depth: %v, header: %s, min timeout: %v, max timeout: %v\n", *outFlag, *depthFlag, *headerFlag, *minTimeFlag, *maxTimeFlag)
		crawlUrl = readStdin()
	} else {
		fmt.Printf("output file: %s, depth: %v, header: %s, min timeout: %v, max timeout: %v\n", *outFlag, *depthFlag, *headerFlag, *minTimeFlag, *maxTimeFlag)
		if len(args) == 0 || len(args) > 1 {
			errorToExit(fmt.Errorf("incorrect format"))
		}
		crawlUrl = args[0]
	}
	pool, err := crawler.NewCrawlerPool(10000, crawlUrl, *outFlag, *depthFlag, *headerFlag, *minTimeFlag, *maxTimeFlag)
	if err != nil {
		errorToExit(err)
	}
	pool.Crawl()
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
