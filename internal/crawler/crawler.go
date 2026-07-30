package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type CrawlerPool struct {
	// settings determined by user
	initSeed   *Seed
	outFile    string
	depth      int
	header     string
	minTimeout float64
	maxTimeout float64
	// derived from initSeed
	host   string
	scheme string
	// pool
	workWg      sync.WaitGroup
	jobWg       sync.WaitGroup
	jobs        chan *Seed
	Results     chan SeedResult
	mu          sync.Mutex
	crawledUrls map[string]struct{}
}

func NewCrawlerPool(maxGoRoutines int, initSeedString string, outFile string, depth int, header string, minTimeout float64, maxTimeout float64) (*CrawlerPool, error) {
	u, err := url.Parse(initSeedString)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	scheme := u.Scheme
	p := CrawlerPool{
		initSeed:    NewSeed(initSeedString, initSeedString, 0),
		outFile:     outFile,
		depth:       depth,
		header:      header,
		minTimeout:  minTimeout,
		maxTimeout:  maxTimeout,
		host:        host,
		scheme:      scheme,
		jobs:        make(chan *Seed),
		Results:     make(chan SeedResult),
		crawledUrls: map[string]struct{}{},
	}
	for range maxGoRoutines {
		p.workWg.Add(1)
		go p.worker()
	}
	return &p, nil
}

func (p *CrawlerPool) Crawl() {
	p.AddJob(p.initSeed)
	go p.Shutdown()
	p.Writer(p.outFile)
}

func (p *CrawlerPool) AddJob(s *Seed) {
	if s.depth > p.depth {
		return
	}
	p.mu.Lock()
	_, crawled := p.crawledUrls[s.url]
	if crawled {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	p.jobWg.Add(1)
	p.jobs <- s
}

func (p *CrawlerPool) Writer(filename string) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Println("error opening file: ", err)
		return
	}
	defer file.Close()
	file.WriteString("[")
	for seed := range p.Results {
		seedOut := &SeedJson{Url: seed.Url, Title: seed.Title, Content: seed.Content}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false) // Disable HTML escaping
		err := enc.Encode(seedOut)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("writing to file: ", filename)
		if _, err := file.WriteString(fmt.Sprintf("%s,", buf.String())); err != nil {
			fmt.Println("error writing to file:", err)
		}
	}
	file.WriteString("]")
}

func (p *CrawlerPool) Shutdown() {
	p.jobWg.Wait()
	close(p.jobs)
	p.workWg.Wait()
	close(p.Results)
}

func (p *CrawlerPool) worker() {
	defer p.workWg.Done()
	for j := range p.jobs {
		result := j.Task(p)
		if result.Err != nil {
			fmt.Println(result.Err.Error())
			p.jobWg.Done()
			continue
		}
		p.Results <- result
		p.jobWg.Done()
	}
}

type SeedJson struct {
	Url     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type SeedResult struct {
	Url     string
	Title   string
	Content string
	Err     error
}

type Seed struct {
	url   string
	depth int
}

func NewSeed(initSeed string, url string, previousDepth int) *Seed {
	if url != initSeed {
		return &Seed{url, previousDepth + 1}
	}
	return &Seed{url, previousDepth}
}

func (s *Seed) Task(p *CrawlerPool) SeedResult {
	fmt.Println("current depth: ", s.depth)
	p.mu.Lock()
	p.crawledUrls[s.url] = struct{}{}
	p.mu.Unlock()
	fmt.Println("fetching: ", s.url)
	content, err := s.fetch(p.initSeed.url, p.minTimeout, p.maxTimeout)
	if err != nil {
		return SeedResult{Url: s.url, Title: "", Content: string(content), Err: err}
	}
	fmt.Println("parsing: ", s.url)
	title, err := s.parse(content, p)
	if err != nil && err.Error() != "EOF" {
		return SeedResult{Url: s.url, Title: title, Content: string(content), Err: err}
	}
	return SeedResult{Url: s.url, Title: title, Content: string(content), Err: nil}
}

func (s *Seed) fetch(initSeedString string, minTimeout float64, maxTimeout float64) ([]byte, error) {
	if s.url != initSeedString {
		// need test to make sure raandom number is in intended range
		timeout := rand.Float64()*(maxTimeout-minTimeout) + minTimeout
		fmt.Println(timeout)
		time.Sleep(time.Duration(timeout))
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	// custom header support, we need dat!
	// req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// for _, header := range req.Header {
	// 	fmt.Println(header)
	// }
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return bodyBytes, nil
	default:
		return nil, fmt.Errorf("error fetching %s: %s", s.url, resp.Status)
	}
}

func (s *Seed) parse(content []byte, p *CrawlerPool) (string, error) {
	z := html.NewTokenizer(bytes.NewReader(content))
	var title string
	body := false
	depth := 0
	for {
		tt := z.Next()
		token := z.Token()
		switch tt {
		case html.ErrorToken:
			if z.Err().Error() == "EOF" {
				if !body {
					return title, fmt.Errorf("webpage has no content")
				}
				return title, nil
			}
			return title, z.Err()
		case html.TextToken:
			text := token.Data
			// need more checks if wanting to parse out text other than title
			if depth > 0 {
				title = text
			}
		case html.StartTagToken, html.EndTagToken:
			tag := token.Data
			attr := token.Attr
			if tag != "title" && tag != "body" && tag != "a" {
				continue
			}
			if tag == "title" {
				if tt == html.StartTagToken {
					depth++
				} else {
					depth--
				}
			}
			if tag == "body" {
				body = true
			}
			if tag == "a" {
				for _, a := range attr {
					if a.Key == "href" {
						if len(a.Val) == 0 || strings.HasPrefix(a.Val, "#") {
							continue
						}
						validUrl, valid := s.validateUrl(a.Val, p)
						if valid {
							seed := NewSeed(p.initSeed.url, validUrl, s.depth)
							p.AddJob(seed)
						}
					} else {
						continue
					}
				}
			}
		}
	}
}

func (s *Seed) validateUrl(url string, p *CrawlerPool) (string, bool) {
	protocol := fmt.Sprintf("%s://", p.scheme)
	fullHost := fmt.Sprintf("%s%s", protocol, p.host)
	if strings.HasPrefix(url, fullHost) {
		return url, true
	} else if strings.HasPrefix(url, "/") {
		newUrl := fmt.Sprintf("%s%s", fullHost, url)
		return newUrl, true
	} else if !strings.HasPrefix(url, protocol) && strings.HasSuffix(url, ".php") {
		newUrl := fmt.Sprintf("%s/%s", fullHost, url)
		return newUrl, true
	}
	return "", false
}
