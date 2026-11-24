// Command epcg-advance-authorisation-export-obligation tracks the export
// obligations attached to Indian EPCG and Advance Authorisation licences: what
// is owed, what has been counted toward it, what is still short, how long is
// left, and what becomes payable in duty and interest if the obligation is not
// discharged.
//
// It serves a small web interface on the machine it runs on and keeps every
// licence in one JSON file beside the binary. It makes no outbound network
// calls of any kind.
//
// What it is not: it is not the Directorate General of Foreign Trade and it is
// not Customs. It has no connection to the DGFT portal or to ICEGATE, it files
// nothing, and it cannot obtain an Export Obligation Discharge Certificate. It
// does arithmetic on figures an exporter types in from their own authorisation,
// bills of entry and shipping bills, using multiples and periods that the
// Foreign Trade Policy fixes and amends often — every one of which is shown
// with its source and can be changed.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// webUI is the interface, built into the binary so the tool is one file.
//
//go:embed web
var webUI embed.FS

const (
	defaultAddr = "127.0.0.1:8080"
	defaultData = "./epcg-advance-authorisation-export-obligation-data.json"

	// maxJSONBody caps what a request may send, so a mistaken paste cannot
	// exhaust memory. A licence with a thousand shipping bills on it is a few
	// hundred kilobytes.
	maxJSONBody = 4 << 20 // 4 MiB
)

func main() {
	log.SetFlags(0)

	addr := flag.String("addr", defaultAddr, "address to serve the interface on")
	data := flag.String("data", defaultData, "file to keep your licences in")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "Export obligation and EODC tracker for EPCG and Advance Authorisation licences.")
		fmt.Fprintln(out, "Serves a local web interface and keeps your licences in one file.")
		fmt.Fprintln(out, "\nUsage:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if err := run(*addr, *data); err != nil {
		log.Fatalf("epcg-advance-authorisation-export-obligation: %v", err)
	}
}

// run opens the licence file, serves the interface and waits for an interrupt.
func run(addr, dataPath string) error {
	st, err := openStore(dataPath)
	if err != nil {
		return err
	}

	assets, err := fs.Sub(webUI, "web")
	if err != nil {
		return fmt.Errorf("reading the built-in interface: %w", err)
	}

	mux := http.NewServeMux()
	(&api{store: st}).routes(mux)
	mux.Handle("GET /", http.FileServerFS(assets))

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot serve on %s: %w", addr, err)
	}

	srv := &http.Server{
		Handler:           guard(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	absData, absErr := filepath.Abs(dataPath)
	if absErr != nil {
		absData = dataPath
	}
	log.Printf("Export obligation tracker — EPCG and Advance Authorisation")
	log.Printf("  open      http://%s", listener.Addr())
	log.Printf("  licences  %s", absData)
	log.Printf("  stop      Ctrl-C")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()

	select {
	case err := <-served:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Printf("stopping")
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}

// guard wraps the whole interface in the few protections a local web tool
// needs: it refuses a request a page on another site has sent to it, and tells
// the browser not to guess at content types.
func guard(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")

		if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
			http.Error(w, "this request came from another site and was refused", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// sameOrigin reports whether an Origin header belongs to the host this server
// was reached on.
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}
