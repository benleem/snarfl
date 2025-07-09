package scrape

import (
	"github.com/benleem/snarfl/internal/pool"
)

type Scraper struct {
	initSeed string
	outFile  *string
	// cache
	// db
}

func NewScraper(initSeed string, out *string) (*Scraper, error) {
	return &Scraper{
		initSeed,
		out,
	}, nil
}

func (s *Scraper) Crawl() error {
	p, err := pool.NewPool(10000, s.initSeed)
	if err != nil {
		return err
	}
	initSeed := pool.NewSeed(s.initSeed)
	p.AddJob(*initSeed)
	go p.Shutdown()
	p.Writer(*s.outFile)
	return nil
}

// func (s *Scraper) extract() error {
// 	fmt.Printf("outputting to file: %s", *s.outFile)
// 	return nil
// }
