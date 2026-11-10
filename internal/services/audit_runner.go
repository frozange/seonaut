package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stjudewashere/seonaut/internal/models"
	"github.com/stjudewashere/seonaut/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

// AuditCrawlRunner lets the audit package crawl a site with the regular crawler.
// Audit projects belong to a dedicated robot account that nobody can sign in to.
type AuditCrawlRunner struct {
	Projects  *ProjectService
	Crawler   *CrawlerService
	Broker    *Broker
	Users     *repository.UserRepository
	ProjRepo  *repository.ProjectRepository
	CrawlRepo *repository.CrawlRepository
	BotEmail  string
	UserAgent string
	MaxPages  int // pages per audit crawl, 0 = the instance limit
}

// botUser returns the robot account, creating it on first use with a random
// password nobody knows.
func (r *AuditCrawlRunner) botUser() (*models.User, error) {
	if u, err := r.Users.FindUserByEmail(r.BotEmail); err == nil {
		return u, nil
	}
	secret := make([]byte, 32)
	rand.Read(secret)
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(secret)), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return r.Users.UserSignup(r.BotEmail, string(hash), "en", LightTheme)
}

// Crawl creates a project for the site, crawls it and waits for the issues.
func (r *AuditCrawlRunner) Crawl(ctx context.Context, siteURL string) (int64, int64, error) {
	bot, err := r.botUser()
	if err != nil {
		return 0, 0, fmt.Errorf("robot account: %w", err)
	}
	p := models.Project{
		URL:          siteURL,
		CrawlSitemap: true,
		UserAgent:    r.UserAgent,
	}
	if err := r.Projects.SaveProject(&p, bot.Id); err != nil {
		return 0, 0, err
	}

	// SaveProject does not return the id: take the newest project of the robot
	// with this URL.
	var found *models.Project
	for _, x := range r.ProjRepo.FindProjectsByUser(bot.Id) {
		x := x
		if strings.TrimSpace(x.URL) == siteURL && (found == nil || x.Id > found.Id) {
			found = &x
		}
	}
	if found == nil {
		return 0, 0, errors.New("project not found after creation")
	}
	p = *found

	done := make(chan struct{}, 1)
	sub := r.Broker.NewSubscriber(fmt.Sprintf("crawl-%d", p.Id), func(m *models.Message) error {
		if m.Name == "CrawlEnd" {
			select {
			case done <- struct{}{}:
			default:
			}
		}
		return nil
	})
	defer r.Broker.Unsubscribe(sub)

	r.Crawler.SetPageLimit(p.Id, r.MaxPages)
	defer r.Crawler.SetPageLimit(p.Id, 0)

	// The instance may already run its maximum of crawls: wait for a free slot.
	for {
		err := r.Crawler.StartCrawler(p, models.BasicAuth{})
		if err == nil {
			break
		}
		if !errors.Is(err, ErrTooManyCrawls) {
			return p.Id, 0, err
		}
		select {
		case <-ctx.Done():
			return p.Id, 0, fmt.Errorf("no crawl slot became free: %w", ctx.Err())
		case <-time.After(30 * time.Second):
		}
	}

	select {
	case <-done:
	case <-ctx.Done():
		r.Crawler.StopCrawler(p)
		return p.Id, 0, fmt.Errorf("crawl did not finish in time: %w", ctx.Err())
	}

	crawl := r.CrawlRepo.GetLastCrawl(&p)
	if crawl.Id == 0 {
		return p.Id, 0, errors.New("crawl not found")
	}
	return p.Id, crawl.Id, nil
}

// Cleanup deletes the audit project and its crawl data once the report holds
// everything it needs.
// LeftoverProjects lists the robot's projects.
func (r *AuditCrawlRunner) LeftoverProjects() []int64 {
	bot, err := r.Users.FindUserByEmail(r.BotEmail)
	if err != nil {
		return nil
	}
	var ids []int64
	for _, p := range r.ProjRepo.FindProjectsByUser(bot.Id) {
		ids = append(ids, p.Id)
	}
	return ids
}

func (r *AuditCrawlRunner) Cleanup(projectID int64) {
	bot, err := r.botUser()
	if err != nil {
		return
	}
	p, err := r.Projects.FindProject(int(projectID), bot.Id)
	if err != nil {
		return
	}
	r.Projects.DeleteProject(&p)
}
