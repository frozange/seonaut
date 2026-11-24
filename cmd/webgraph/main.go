// Command webgraph keeps the Common Crawl domain graph index used by the
// popularity section of the audit report.
//
//	webgraph -dir D status
//	webgraph -dir D [-rate 6] update -for 2m
//	webgraph -dir D query <domain>
//
// update looks for a new quarterly release at most once a day, downloads its
// files over one connection at a capped rate (MB/s) until the slot ends, and
// resumes where it stopped on the next run. Once a release is complete it is
// indexed (local work, no download) and its download is deleted. Exit codes
// of update: 0 nothing left to do, 10 call again in the same slot, 12 the
// server asked to wait (do not call again before the next slot), 1 error.
//
// The schedule (outside the hours another job of the server reads Common
// Crawl) is the caller's business: see deploy/webgraph-cron.sh.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stjudewashere/seonaut/internal/webgraph"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: webgraph -dir D status\n       webgraph -dir D [-rate MB/s] update -for 2m\n       webgraph -dir D query <domain>")
	os.Exit(2)
}

func main() {
	dir := flag.String("dir", "", "Data directory.")
	rate := flag.Float64("rate", 6, "Download cap in MB/s.")
	flag.Parse()
	args := flag.Args()
	if *dir == "" || len(args) == 0 {
		usage()
	}
	logger := log.New(os.Stdout, "", log.LstdFlags)
	store := &webgraph.Store{
		Dir:     *dir,
		Fetcher: &webgraph.Fetcher{BytesPerSec: int64(*rate * 1e6), Log: logger.Printf},
		Builder: &webgraph.Builder{Log: logger.Printf},
		Log:     logger.Printf,
	}

	switch args[0] {
	case "status":
		fmt.Print(store.Status())
	case "update":
		fs := flag.NewFlagSet("update", flag.ExitOnError)
		slot := fs.Duration("for", 2*time.Minute, "Download time slot.")
		fs.Parse(args[1:])
		unlock, err := store.Lock()
		if err != nil {
			logger.Print(err)
			os.Exit(12)
		}
		defer unlock()
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		out, err := store.Update(ctx, time.Now().Add(*slot))
		if err != nil {
			logger.Printf("update: %s: %v", out, err)
			unlock()
			os.Exit(1)
		}
		logger.Printf("update: %s", out)
		unlock()
		switch out {
		case webgraph.More, webgraph.Built:
			os.Exit(10)
		case webgraph.Backoff:
			os.Exit(12)
		}
	case "query":
		if len(args) != 2 {
			usage()
		}
		cur, prev, err := store.Open()
		if err != nil {
			log.Fatal(err)
		}
		defer cur.Close()
		defer prev.Close()
		id, ok := cur.Lookup(args[1])
		if !ok {
			fmt.Printf("%s: not in %s\n", args[1], cur.Meta.Release)
			return
		}
		fmt.Printf("%s in %s: rank %d of %d, %d referring domains\n", args[1], cur.Meta.Release, cur.Rank(id), cur.Meta.Nodes, cur.InDegree(id))
		for _, r := range cur.TopReferrers(id, 20) {
			fmt.Printf("  %-40s rank %d\n", r.Domain, r.Rank)
		}
		if prev != nil {
			if pid, ok := prev.Lookup(args[1]); ok {
				fmt.Printf("in %s: %d referring domains\n", prev.Meta.Release, prev.InDegree(pid))
			}
		}
	default:
		usage()
	}
}
