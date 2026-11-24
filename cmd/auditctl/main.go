// Command auditctl queues a free audit from the server itself, through the same
// validation and limits as the /audit form, without the browser and its human
// check. It is the owner's route for tests and requests made on someone's
// behalf: it needs the instance config (database credentials), so only the
// account that runs SEOnaut can use it.
//
//	auditctl -c config [-fresh] request <site URL> <e-mail>
//	auditctl -c config status <token or slug>
//	auditctl -c config requery <token or slug>
//	auditctl -c config popularity <token or slug> [competitor ...]
//	auditctl -c config popularity -missing
//
// requery checks the target searches of a finished report against Google
// Suggest again, with no crawl and no AI call. popularity reads the sites
// linking to the audited site in the latest Common Crawl graph again, and
// compares it with the competitor sites given (none: those kept, if any);
// -missing does it for every site whose latest report lacks it (run by the
// graph job once a quarter is indexed).
//
// -fresh crawls again even when a report of the same site is recent enough to
// be reused (the form always reuses it).
//
// The running server picks the queued audit up within 30 seconds.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/stjudewashere/seonaut/internal/audit"
	"github.com/stjudewashere/seonaut/internal/config"
	"github.com/stjudewashere/seonaut/internal/repository"
	"github.com/stjudewashere/seonaut/internal/services"
	"github.com/stjudewashere/seonaut/internal/webgraph"
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: auditctl -c config [-fresh] request <site URL> <e-mail>\n       auditctl -c config status <token or slug>\n       auditctl -c config requery <token or slug>\n       auditctl -c config popularity <token or slug> [competitor ...]\n       auditctl -c config popularity -missing")
	os.Exit(2)
}

func main() {
	var configFile string
	var fresh bool
	flag.StringVar(&configFile, "c", "config", "Configuration file.")
	flag.BoolVar(&fresh, "fresh", false, "Crawl again instead of reusing a recent report.")
	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		usage()
	}

	cfg, err := config.NewConfig(configFile)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if !cfg.Audit.Enabled {
		log.Fatal("audit.enabled is false in this config")
	}
	db, err := repository.SqlConnect(cfg.DB)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	svc := &audit.Service{
		Store: &audit.Store{DB: db},
		Cfg: audit.Config{
			BaseURL:     cfg.HTTPServer.URL,
			ReportURL:   cfg.Audit.ReportURL,
			DefaultLang: cfg.Audit.Language,
			DailyCap:    cfg.Audit.DailyCap,
			PerEmailDay: cfg.Audit.PerEmailPerDay,
			ReuseFor:    time.Duration(cfg.Audit.ReuseHours) * time.Hour,
		},
	}
	if fresh {
		svc.Cfg.ReuseFor = 0
	}
	if m, err := services.NewMailer(cfg.Mail); err == nil {
		svc.Mailer = m
	}

	switch args[0] {
	case "request":
		if len(args) != 3 {
			usage()
		}
		a, err := svc.Request(args[1], args[2], "cli")
		if err != nil {
			log.Fatalf("request refused: %v", err)
		}
		if a.Status == audit.StatusDone {
			// A recent report of the same site was reused; its link is mailed.
			time.Sleep(2 * time.Second)
		}
		fmt.Printf("audit %d %s\nstatus: %s\nreport: %s\n", a.ID, a.URL, a.Status, svc.ReportURL(a))
	case "status":
		a, err := svc.Store.ByKey(args[1])
		if errors.Is(err, audit.ErrNotFound) {
			log.Fatal("no audit with this token")
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("audit %d %s\nstatus: %s %s\nupdated: %s\n", a.ID, a.URL, a.Status, a.Error, a.Updated.Format(time.RFC3339))
		if a.Result != nil && a.Result.AIError != "" {
			fmt.Printf("AI error: %s\n", a.Result.AIError)
		}
	case "requery":
		a, err := svc.Store.ByKey(args[1])
		if err != nil {
			log.Fatalf("no audit with this token or name: %v", err)
		}
		svc.Suggest = &audit.Suggester{Pause: 400 * time.Millisecond}
		if !cfg.Audit.Suggest {
			svc.Suggest = nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := svc.Requery(ctx, a); err != nil {
			log.Fatalf("requery: %v", err)
		}
		fmt.Printf("audit %d %s\n", a.ID, a.URL)
		for _, q := range a.Result.AI.TargetQueries {
			fmt.Printf("kept    %-10s %s\n", q.Google, q.Query)
		}
		for _, q := range a.Result.AI.DroppedQueries {
			fmt.Printf("dropped %-10s %s %v\n", q.Google, q.Query, q.GoogleRelated)
		}
	case "popularity":
		if cfg.Audit.WebGraphDir != "" {
			svc.Graph = &webgraph.Store{Dir: cfg.Audit.WebGraphDir}
		}
		if args[1] == "-missing" {
			n, err := svc.FillPopularity()
			if err != nil {
				log.Fatalf("popularity: %v", err)
			}
			fmt.Printf("popularity measured for %d report(s)\n", n)
			return
		}
		a, err := svc.Store.ByKey(args[1])
		if err != nil {
			log.Fatalf("no audit with this token or name: %v", err)
		}
		var competitors []string
		if len(args) > 2 {
			competitors = args[2:]
		}
		if err := svc.RefreshPopularity(a, competitors); err != nil {
			log.Fatalf("popularity: %v", err)
		}
		p := a.Result.Popularity
		fmt.Printf("audit %d %s, graph %s (previous: %s)\n", a.ID, a.URL, p.Release, p.Previous)
		for _, d := range append([]audit.DomainPop{p.Site}, p.Competitors...) {
			if !d.Found {
				fmt.Printf("%-30s not in the sample yet\n", d.Domain)
				continue
			}
			fmt.Printf("%-30s %d referring domains, rank %d\n", d.Domain, d.Referring, d.Rank)
		}
		if p.Compared {
			fmt.Printf("gained %d, lost %d\n", p.GainedCount, p.LostCount)
		}
		if p.GapMin > 0 {
			fmt.Printf("sites linking to competitors and not to the site: %d\n", p.GapCount)
		}
	default:
		usage()
	}
}
