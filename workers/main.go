// Command workers is a chromedp example demonstrating how to run many jobs at
// the same time in one browser with a pool of goroutines. The program starts
// the browser once, and each worker opens one tab for each job, with a new
// context of the same browser. The jobs read a local page that answers after a
// different delay for each job. The workers send the jobs on a channel, and the
// program collects the results in the order of the jobs. Each job has a time
// limit. A derived context ends the job when the limit passes, and it does not
// stop the browser or the other jobs. One job is slower than the limit on
// purpose. At the end, the program prints the total time next to the sum of
// the times of the jobs, which shows the speedup of the pool. It starts a local
// server and needs no internet. Use -v to print the protocol messages and
// -visible to show the browser window and leave it open.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// job is one page to read. The server waits for Delay before it answers.
type job struct {
	ID    int
	Delay time.Duration
}

// result is what a worker found for one job.
type result struct {
	Title   string
	Value   string
	Elapsed time.Duration
	Err     error
}

func main() {
	workers := flag.Int("workers", 4, "number of workers, and so of tabs that work at the same time")
	limit := flag.Duration("timeout", 2*time.Second, "time limit of one job")
	verbose := flag.Bool("v", false, "print the protocol messages")
	visible := flag.Bool("visible", false, "show the browser window and leave it open")
	flag.Parse()

	// start the server
	srv := httptest.NewServer(newMux())
	defer srv.Close()

	// create context
	var opts []chromedp.ContextOption
	if *verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	if *visible {
		opts = append(opts, chromedp.WithVisibleWindow(), remote.WithKeepOpen())
	}
	ctx, cancel := chromedp.NewContext(context.Background(), opts...)
	defer cancel()
	if *visible {
		defer func() {
			wsURL, dir := chromedp.KeptOpen(ctx)
			fmt.Fprintf(os.Stderr, "browser kept open at %s with profile directory %s\n", wsURL, dir)
		}()
	}

	if err := run(ctx, srv.URL, *workers, *limit, !*visible); err != nil {
		log.Fatal(err)
	}
}

// run reads the pages with a pool of workers, and prints the results. When
// shutdown is true, it waits until the browser has exited.
func run(ctx context.Context, host string, workers int, limit time.Duration, shutdown bool) error {
	// Start the browser before the workers do. An empty Do starts it. A
	// worker that starts it owns the browser, and the time limit of that
	// worker will stop the browser for all.
	if err := chromedp.Do(ctx); err != nil {
		return fmt.Errorf("starting the browser: %w", err)
	}

	// The delays are in milliseconds. The last job is slower than the time
	// limit of the default flags.
	delays := []int{400, 700, 300, 600, 500, 800, 200, 5000}
	jobs := make([]job, len(delays))
	for i, d := range delays {
		jobs[i] = job{ID: i + 1, Delay: time.Duration(d) * time.Millisecond}
	}

	// Each worker takes jobs from the channel until the channel is closed
	// and empty. A result goes to the slot of its job, so every slot has one
	// writer, and the results are in the order of the jobs.
	queue := make(chan job)
	results := make([]result, len(jobs))
	start := time.Now()
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for j := range queue {
				results[j.ID-1] = read(ctx, host, j, limit)
			}
		})
	}
	for _, j := range jobs {
		queue <- j
	}
	close(queue)
	wg.Wait()
	total := time.Since(start)

	var sum, delaySum time.Duration
	var failed int
	for i, r := range results {
		sum += r.Elapsed
		delaySum += jobs[i].Delay
		if r.Err != nil {
			failed++
			if errors.Is(r.Err, context.DeadlineExceeded) {
				fmt.Printf("job %d, delay %s: timed out after %s\n", jobs[i].ID, jobs[i].Delay, r.Elapsed.Round(100*time.Millisecond))
			} else {
				fmt.Printf("job %d, delay %s: error: %v\n", jobs[i].ID, jobs[i].Delay, r.Err)
			}
			continue
		}
		fmt.Printf("job %d, delay %s: title %q, value %s, took %s\n", jobs[i].ID, jobs[i].Delay, r.Title, r.Value, r.Elapsed.Round(100*time.Millisecond))
	}
	fmt.Printf("%d jobs, %d workers, %d finished, %d failed\n", len(jobs), workers, len(jobs)-failed, failed)
	fmt.Printf("sum of the delays: %s, sum of the times of the jobs: %s\n", delaySum, sum.Round(100*time.Millisecond))
	fmt.Printf("total time: %s, the speedup is about %.1f times\n", total.Round(100*time.Millisecond), float64(sum)/float64(total))

	// Cancel waits until the browser has exited, so no process stays behind.
	if shutdown {
		if err := chromedp.Cancel(ctx); err != nil {
			return fmt.Errorf("closing the browser: %w", err)
		}
	}
	return nil
}

// read opens a tab for the job, and reads its page. The time limit ends the
// job, and the deferred cancel closes the tab in every case.
func read(browser context.Context, host string, j job, limit time.Duration) (res result) {
	start := time.Now()
	defer func() { res.Elapsed = time.Since(start) }()

	// A new context of the context of the browser is a new tab. The deadline
	// belongs to a context that is derived from the tab, so it ends the work
	// of this job. The browser has its own context, and it does not see the
	// deadline.
	tab, closeTab := chromedp.NewContext(browser)
	defer closeTab()
	jobCtx, cancel := context.WithTimeout(tab, limit)
	defer cancel()

	url := host + "/page?id=" + strconv.Itoa(j.ID) + "&delay=" + strconv.Itoa(int(j.Delay.Milliseconds()))
	if res.Err = chromedp.Do(jobCtx, chromedp.Navigate(url)); res.Err != nil {
		return res
	}
	if res.Title, res.Err = chromedp.Run(jobCtx, chromedp.Title()); res.Err != nil {
		return res
	}
	res.Value, res.Err = chromedp.Run(jobCtx, chromedp.Text(chromedp.ID("value")))
	return res
}

// newMux returns the handlers of the test server. The page /page waits for
// delay milliseconds. Its title names the job, and its value is the square of
// the id.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))
		delay, _ := strconv.Atoi(r.URL.Query().Get("delay"))
		select {
		case <-time.After(time.Duration(delay) * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		fmt.Fprintf(w, `<html><head><title>job %d</title></head><body><span id="value">%d</span></body></html>`, id, id*id)
	})
	return mux
}
