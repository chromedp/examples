// Command distrust is a chromedp example demonstrating how to check that a
// browser rejects the certificates of a compromised certificate authority. It
// starts the image headless-shell in a container with Podman, opens the test
// page of the eMudhra incident of October 2026 in that browser, opens every
// test host that the page lists, and reports what the browser did with each
// one. It reads https://emudhra.we-have-a-problem.com/ and the test hosts of
// that page, and it needs Podman and the internet. A host that connects is a
// problem, because the browser still trusts a compromised intermediate
// certificate. A host that the browser rejects only because of Certificate
// Transparency is a problem too, because the browser checks the chain first.
// The program exits with the status 1 when any host is a problem. Use -image
// to name another image, -podman to name the command that runs the container,
// and -url to read another test page with the same layout. The program has no
// flag -visible, because the browser runs in a container. Use -v to print the
// protocol messages. Use -visible-on-terminal to draw the page in the terminal
// with terminal graphics.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/chromedp/termcast"
)

func main() {
	verbose := flag.Bool("v", false, "print the protocol messages")
	image := flag.String("image", "docker.io/chromedp/headless-shell:latest", "image of the browser")
	podman := flag.String("podman", "podman", "command that runs the container, such as podman, docker or nerdctl")
	pull := flag.String("pull", "newer", "pull policy of the image: always, missing, never or newer (docker has no newer)")
	urlstr := flag.String("url", "https://emudhra.we-have-a-problem.com/", "test page that lists the hosts")
	timeout := flag.Duration("timeout", 20*time.Second, "time limit for one host")
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	flag.Parse()
	ok, err := run(context.Background(), &tc, *verbose, *image, *podman, *pull, *urlstr, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	if !ok {
		os.Exit(1)
	}
}

// run starts the browser, checks every host of the test page, and reports
// whether the browser rejected all of them.
func run(ctx context.Context, tc *termcast.Flags, verbose bool, image, podman, pull, urlstr string, timeout time.Duration) (bool, error) {
	// start the browser in a container. The container publishes the port of
	// the protocol on a free port of this host, for this host only
	port, err := freePort()
	if err != nil {
		return false, err
	}
	id, err := startContainer(ctx, podman, pull, image, port)
	if err != nil {
		return false, err
	}
	defer stopContainer(podman, id)

	// the browser needs a moment to start. Its version says that it is ready
	version, err := waitForBrowser(ctx, port)
	if err != nil {
		return false, err
	}

	// connect to the browser, as the program remote does
	allocCtx, cancel := remote.NewAllocator(ctx, fmt.Sprintf("http://127.0.0.1:%d/", port))
	defer cancel()
	var opts []chromedp.ContextOption
	if verbose {
		opts = append(opts, chromedp.WithDebugf(log.Printf))
	}
	ctx, cancel = chromedp.NewContext(allocCtx, opts...)
	defer cancel()

	// draw the page in the terminal when the user asks for it. The stream
	// connects to the browser, so it starts before the first navigation
	s, err := tc.Start(ctx, verbose)
	if err != nil {
		return false, err
	}
	defer s.Stop()
	out := io.Writer(os.Stdout)
	if s != nil {
		out = s.LogWriter()
	}
	fmt.Fprintf(out, "browser: %s (%s)\n", version, image)

	// open the test page and read the hosts of its table. A script of the
	// page builds the table, so wait for the first link of a row
	pctx, pcancel := context.WithTimeout(ctx, timeout)
	defer pcancel()
	err = chromedp.Do(pctx,
		chromedp.Navigate(urlstr),
		chromedp.WaitVisible(`#results tbody tr a`),
	)
	if err != nil {
		return false, fmt.Errorf("opening %s: %w", urlstr, err)
	}
	hosts, err := chromedp.Run(pctx, chromedp.Evaluate[[]string](`[...document.querySelectorAll("#results tbody a[href]")].map(a => a.href)`))
	if err != nil {
		return false, fmt.Errorf("reading the hosts of %s: %w", urlstr, err)
	}
	if len(hosts) == 0 {
		return false, fmt.Errorf("the page %s lists no host", urlstr)
	}

	// open every host and sort the results. Chrome checks the chain of the
	// certificate before Certificate Transparency, so a host that fails only
	// the transparency check still has a trusted chain
	var rejected, transparency, connected, unreached int
	for _, host := range hosts {
		result := check(ctx, timeout, host)
		switch result.kind {
		case kindRejected:
			rejected++
		case kindTransparency:
			transparency++
		case kindConnected:
			connected++
		default:
			unreached++
		}
		fmt.Fprintf(out, "%-52s %s\n", strings.TrimSuffix(strings.TrimPrefix(host, "https://"), "/"), result)
	}
	fmt.Fprintf(out, "\n%d hosts: %d rejected, %d rejected by transparency only, %d connected, %d not reached\n",
		len(hosts), rejected, transparency, connected, unreached)
	switch {
	case connected > 0 || transparency > 0:
		fmt.Fprintln(out, "the browser still trusts a compromised intermediate certificate")
	case unreached > 0:
		fmt.Fprintln(out, "a host was not reached, so the result is not complete")
	default:
		fmt.Fprintln(out, "the browser rejects every compromised intermediate certificate")
	}
	return connected == 0 && transparency == 0 && unreached == 0, nil
}

// kind is what the browser did with a host.
type kind int

const (
	// kindRejected is a host that the browser rejected because of its chain.
	kindRejected kind = iota
	// kindTransparency is a host that the browser rejected only because the
	// certificate was not logged to Certificate Transparency.
	kindTransparency
	// kindConnected is a host that the browser connected to.
	kindConnected
	// kindUnreached is a host that the browser did not reach, for example
	// because of the network.
	kindUnreached
)

// result is the result of one host.
type result struct {
	kind   kind
	detail string
}

// String formats the result for the report.
func (r result) String() string {
	switch r.kind {
	case kindRejected:
		return "rejected: " + r.detail
	case kindTransparency:
		return "rejected by transparency only: " + r.detail
	case kindConnected:
		return "connected, that is bad"
	}
	return "not reached: " + r.detail
}

// check opens the host in the browser and reads what the browser did. The
// browser shows its own error page for a certificate that it rejects, and the
// navigation returns the name of the error, such as net::ERR_CERT_REVOKED.
func check(ctx context.Context, timeout time.Duration, host string) result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := chromedp.Do(ctx, chromedp.Navigate(host))
	if err == nil {
		return result{kind: kindConnected}
	}
	name := netError(err)
	switch {
	case name == "net::ERR_CERTIFICATE_TRANSPARENCY_REQUIRED":
		return result{kind: kindTransparency, detail: name}
	case strings.HasPrefix(name, "net::ERR_CERT_") || name == "net::ERR_SSL_PINNED_KEY_NOT_IN_CERT_CHAIN":
		return result{kind: kindRejected, detail: name}
	case errors.Is(err, context.DeadlineExceeded):
		return result{kind: kindUnreached, detail: "time limit"}
	case name != "":
		return result{kind: kindUnreached, detail: name}
	}
	return result{kind: kindUnreached, detail: err.Error()}
}

// netError returns the name of the network error in the error of a navigation,
// such as net::ERR_CERT_AUTHORITY_INVALID, or an empty string.
func netError(err error) string {
	s := err.Error()
	i := strings.Index(s, "net::")
	if i < 0 {
		return ""
	}
	name := s[i:]
	if j := strings.IndexAny(name, " \n"); j >= 0 {
		name = name[:j]
	}
	return name
}

// freePort returns a free TCP port of this host.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("finding a free port: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// startContainer starts the image in a container that publishes the port 9222
// of the browser on the port of this host. It returns the id of the container.
func startContainer(ctx context.Context, podman, pull, image string, port int) (string, error) {
	cmd := exec.CommandContext(ctx, podman, "run", "--detach", "--rm", "--pull="+pull,
		"--publish", fmt.Sprintf("127.0.0.1:%d:9222", port), image)
	cmd.Stderr = os.Stderr
	buf, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("starting the container of %s with %s: %w", image, podman, err)
	}
	// the last line of the output is the id of the container. The lines
	// before it report the pull
	lines := strings.Fields(string(buf))
	if len(lines) == 0 {
		return "", fmt.Errorf("starting the container of %s: %s printed no id", image, podman)
	}
	return lines[len(lines)-1], nil
}

// stopContainer removes the container. The container stops when it is
// removed. It uses its own context, so that it runs when the context of the
// program has ended.
func stopContainer(podman, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, podman, "rm", "--force", id)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: removing the container %s: %v\n", id, err)
	}
}

// waitForBrowser asks the browser for its version until it answers, for 30
// seconds at most. It returns the version.
func waitForBrowser(ctx context.Context, port int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	urlstr := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	for {
		version, err := browserVersion(ctx, urlstr)
		if err == nil {
			return version, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("waiting for the browser at %s: %w", urlstr, err)
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// browserVersion reads the version of the browser from its endpoint.
func browserVersion(ctx context.Context, urlstr string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlstr, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %s", res.Status)
	}
	var v struct {
		Browser string `json:"Browser"`
	}
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		return "", fmt.Errorf("decoding the version: %w", err)
	}
	return v.Browser, nil
}
