package pool

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"golang.org/x/net/html"
)

type Pool struct {
	host        string
	scheme      string
	workWg      sync.WaitGroup
	jobWg       sync.WaitGroup
	jobs        chan Seed
	Results     chan SeedResult
	mu          sync.Mutex
	crawledUrls []string
}

func NewPool(maxGoRoutines int, initSeed string) (*Pool, error) {
	u, err := url.Parse(initSeed)
	if err != nil {
		return nil, err
	}
	host := u.Hostname()
	scheme := u.Scheme
	p := Pool{
		host:        host,
		scheme:      scheme,
		jobs:        make(chan Seed),
		Results:     make(chan SeedResult),
		crawledUrls: []string{},
	}
	for range maxGoRoutines {
		p.workWg.Add(1)
		go p.worker()
	}
	return &p, nil
}

func (p *Pool) worker() {
	defer p.workWg.Done()
	for j := range p.jobs {
		result := j.Task(p)
		if result.Err != nil && result.Err.Error() == "duplicate" {
			p.jobWg.Done()
			return
		}
		p.mu.Lock()
		p.crawledUrls = append(p.crawledUrls, result.Url)
		p.mu.Unlock()
		p.Results <- result
		p.jobWg.Done()
	}
}

func (p *Pool) AddJob(s Seed) {
	p.jobWg.Add(1)
	p.jobs <- s
}

func (p *Pool) Shutdown() {
	p.jobWg.Wait()
	close(p.jobs)
	p.workWg.Wait()
	close(p.Results)
}

type SeedResult struct {
	Url     string
	Title   string
	Content string
	Err     error
}

type Seed struct {
	url string
}

func NewSeed(url string) *Seed {
	return &Seed{url}
}

func (s *Seed) Task(p *Pool) SeedResult {
	// need to convert crawledUrls to map for better search performance
	if slices.Contains(p.crawledUrls, s.url) {
		return SeedResult{Url: s.url, Title: "", Content: "", Err: fmt.Errorf("duplicate")}
	}
	fmt.Printf("fetching content: %s\n", s.url)
	content, err := s.fetch()
	if err != nil {
		return SeedResult{Url: s.url, Title: "", Content: string(content), Err: err}
	}
	fmt.Printf("parsing: %s\n", s.url)
	title, links, err := s.parse(content)
	if err != nil && err.Error() != "EOF" {
		return SeedResult{Url: s.url, Title: title, Content: string(content), Err: err}
	}
	for _, link := range links {
		validUrl, valid := s.validateUrl(link, p)
		if valid {
			seed := NewSeed(validUrl)
			p.AddJob(*seed)
		}
	}
	return SeedResult{Url: s.url, Title: title, Content: string(content), Err: nil}
}

func (s *Seed) fetch() ([]byte, error) {
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
		return nil, fmt.Errorf("error fetching seed url: %s", resp.Status)
	}
}

func (s *Seed) parse(content []byte) (string, []string, error) {
	z := html.NewTokenizer(bytes.NewReader(content))
	var links []string
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
					return title, links, fmt.Errorf("webpage has no content")
				}
				return title, links, nil
			}
			return title, links, z.Err()
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
				z.Next()
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
							z.Next()
							continue
						}
						links = append(links, a.Val)
					} else {
						z.Next()
						continue
					}
				}
			}
		}
	}
}

func (s *Seed) validateUrl(url string, p *Pool) (string, bool) {
	protocol := fmt.Sprintf("%s://", p.scheme)
	fullHost := fmt.Sprintf("%s%s", protocol, p.host)
	if strings.HasPrefix(url, fullHost) {
		return url, true
	} else if strings.HasPrefix(url, "/") {
		newUrl := fmt.Sprintf("%s%s", fullHost, url)
		return newUrl, true
	}
	return "", false
}
