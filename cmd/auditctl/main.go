// Command auditctl queues a free audit from the server itself, through the same
// validation and limits as the /audit form, without the browser and its human
// check. It is the owner's route for tests and requests made on someone's
// behalf: it needs the instance config (database credentials), so only the
// account that runs SEOnaut can use it.
//
//	auditctl -c config request <site URL> <e-mail>
//	auditctl -c config status <token>
//
// The running server picks the queued audit up within 30 seconds.
package main

import (
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
)

func usage() {
	fmt.Fprintln(os.Stderr, "usage: auditctl -c config request <site URL> <e-mail>\n       auditctl -c config status <token>")
	os.Exit(2)
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "c", "config", "Configuration file.")
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
			DefaultLang: cfg.Audit.Language,
			DailyCap:    cfg.Audit.DailyCap,
			PerEmailDay: cfg.Audit.PerEmailPerDay,
			ReuseFor:    time.Duration(cfg.Audit.ReuseHours) * time.Hour,
		},
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
		a, err := svc.Store.ByToken(args[1])
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
	default:
		usage()
	}
}
