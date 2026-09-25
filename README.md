# urlcategorizationdatabase-go

Categorise domains and URLs from Go. This module sends a hostname or URL to the live classifier and returns its content categories, so batch jobs and services can label the web addresses they handle. It complements the [URL database download](https://www.urlcategorizationdatabase.com/pricing.php): the file covers known domains offline, and this client handles everything new.

```bash
go get github.com/explainableaixai/urlcategorizationdatabase-go
```

## Hello, categories

```go
c := urlcategorizationdatabase.New(os.Getenv("AQ_API_KEY"))
res, err := c.Classify(context.Background(), "reuters.com")
if err != nil {
	log.Fatal(err)
}
b, _ := json.MarshalIndent(res, "", "  ")
fmt.Println(string(b))
```

`Classify` posts the value, your key and `data_type=url` as a form, then decodes the JSON reply into a `Result` (`map[string]any`). The client does not reshape the fields, so the reference in the online API documentation applies as written.

## A realistic job: enriching a table of domains

The common case is not one domain but a list. You have a million rows of referrer hosts or company websites, and you want a category column. Three rules make this cheap and reliable.

**Deduplicate first.** Real lists repeat heavily. Normalise by lower-casing and stripping `www.`, then build a set. The unique count is often a fraction of the rows.

**Skip what you already know.** Look each host up in the licensed file or your previous results first. Send only misses to `Classify`.

**Bound concurrency.** A small, fixed number of workers is kinder to the service and simpler to reason about than unbounded goroutines:

```go
func enrich(ctx context.Context, c *urlcategorizationdatabase.Client, hosts []string) map[string]urlcategorizationdatabase.Result {
	const workers = 4
	jobs := make(chan string)
	var mu sync.Mutex
	out := make(map[string]urlcategorizationdatabase.Result, len(hosts))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for h := range jobs {
				r, err := c.Classify(ctx, h)
				if err != nil {
					continue // log it in real code
				}
				mu.Lock()
				out[h] = r
				mu.Unlock()
			}
		}()
	}
	for _, h := range hosts {
		jobs <- h
	}
	close(jobs)
	wg.Wait()
	return out
}
```

Four workers is a sensible start. Lower it if you see HTTP 429.

## Storing results

Keep the raw JSON next to your own extracted columns. A Postgres `jsonb` column or a JSON Lines file works well:

```go
enc := json.NewEncoder(f)
for host, r := range results {
	enc.Encode(map[string]any{"host": host, "at": time.Now().UTC(), "result": r})
}
```

Storing the full answer means you can pull out a new field later without paying for the calls again. The timestamp tells you when to refresh, and a few months is a reasonable age for most sites.

## Errors in batch context

In a job that runs for an hour, some calls will fail. Decide per status what happens:

```go
var apiErr *urlcategorizationdatabase.APIError
if errors.As(err, &apiErr) {
	switch {
	case apiErr.Status == 429:
		time.Sleep(time.Minute) // then requeue
	case apiErr.Status == 401 || apiErr.Status == 403:
		return err // stop: key or quota problem
	default:
		// record and move on
	}
}
```

Timeouts and cancellations arrive as normal errors from the context or transport. An empty key or empty value is rejected before sending.

## Tuning the client

`Client` exposes `APIKey`, `BaseURL` and `HTTPClient`. The default `HTTPClient` has a 30 second timeout, which suits pages that are slow to load. For a batch with a hard deadline, pass a context with `context.WithTimeout` per call instead of shortening the global timeout. Response bodies are capped at 2 MiB.

## Choosing the input form

- **Registered domain** (`example.com`): one label for the whole site. Right for most analytics and filtering.
- **Full hostname** (`blog.example.com`): use it where subdomains are separate sites, as on hosting platforms.
- **Full URL**: use it when the page matters more than the site, such as individual articles on a large publisher.

Pick one form per job and stick to it, or your cache keys will not line up.

## What people build with it

- Sales and marketing teams tag lead lists by industry before routing them.
- Data teams add a category dimension to web analytics or clickstream tables.
- Security teams label outbound traffic for reporting, then hand blocking decisions to [cloud-based web filtering](https://www.webfilteringdatabase.com/web-filtering-database.php), which are built for allow and deny rules.
- Ad platforms pre-classify inventory to support contextual targeting.

For content labels on individual article pages at publish time, [website content classification](https://www.websitecategorizationapi.com) is the more natural fit.

## AI traffic is a special case

General categories place AI products under software or technology, which is too broad for AI policy work. To add risk context, run the hosts through an [AI risk assessment tool](https://www.aitoolsblocklist.com) backed by the dedicated AI register. To separate [sanctioned AI tools](https://www.shadowaitools.com/ai-policy-profiles.php) from unsanctioned ones across the company, the log-scanning service builds that view from existing records.

## Testing

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	fmt.Fprintf(w, `{"query":%q,"categories":[]}`, r.Form.Get("query"))
}))
defer srv.Close()
c := urlcategorizationdatabase.New("k")
c.BaseURL = srv.URL
```

Note that the client appends the endpoint path to `BaseURL`, so a stub server should accept any path.

## Refreshing old labels

Websites change. A domain that hosted a parked page last year may run a shop today. Give every stored result a timestamp and, once a month, requeue entries older than your chosen age, oldest first, through the same worker pool. When a new release of the licensed file arrives, delete learned entries that the file now covers, so the file stays the single reference and your table only holds the gaps.

## Ports to other ecosystems

Rust users can depend on [the urlcategorizationdatabase crate](https://crates.io/crates/urlcategorizationdatabase). PHP projects install [the Composer package](https://packagist.org/packages/urlcategorizationdatabase/urlcategorizationdatabase). JavaScript has [its npm module](https://www.npmjs.com/package/urlcategorizationdatabase), and Flutter apps have [a Dart client](https://pub.dev/packages/urlcategorizationdatabase).

## License

MIT. The IAB Tech Lab Content Taxonomy is referenced for compatibility only.
