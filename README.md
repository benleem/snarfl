# snarfl

web crawler, scraper built in golang

TO DO:

- integrate sqlite for persisting storage of collected websites, perchance redis for caching
  - filter and save only user queried data (i.e. elements with certain ids, class names, styles)
- request timeout range (random number between min and max) to avoid rate limiting
- depth control for crawler following discovered links
- custom request headers (User Agent, etc)
- robots.txt compliance
- input validation for flags and args
