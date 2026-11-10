package services

import (
	"context"
	"database/sql"
	"encoding/base64"
	"log"
	"os"
	"strings"
	"time"

	"github.com/stjudewashere/seonaut/internal/audit"
	"github.com/stjudewashere/seonaut/internal/config"
	"github.com/stjudewashere/seonaut/internal/issues/multipage"
	"github.com/stjudewashere/seonaut/internal/issues/page"
	"github.com/stjudewashere/seonaut/internal/netguard"
	"github.com/stjudewashere/seonaut/internal/repository"
	"github.com/stjudewashere/seonaut/internal/security"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Container struct {
	Config             *config.Config
	PubSubBroker       *Broker
	IssueService       *IssueService
	ReportService      *ReportService
	ReportManager      *ReportManager
	UserService        *UserService
	DashboardService   *DashboardService
	ProjectService     *ProjectService
	ProjectViewService *ProjectViewService
	ExportService      *Exporter
	CrawlerService     *CrawlerService
	Translator         *Translator
	Renderer           *Renderer
	CookieSession      *CookieSession
	ArchiveService     *ArchiveService
	ReplayService      *ReplayService

	// Protections of a public instance.
	Turnstile          *security.Turnstile
	ClientIP           *security.ClientIPResolver
	SignupLimiter      *security.RateLimiter
	SigninIPLimiter    *security.RateLimiter
	SigninEmailLimiter *security.RateLimiter
	CrawlLimiter       *security.RateLimiter
	NetGuard           *netguard.Guard
	Mailer             Mailer
	AuditService       *audit.Service
	AuditIPLimiter     *security.RateLimiter

	db                   *sql.DB
	issueRepository      *repository.IssueRepository
	pageReportRepository *repository.PageReportRepository
	userRepository       *repository.UserRepository
	projectRepository    *repository.ProjectRepository
	exportRepository     *repository.ExportRepository
	crawlRepository      *repository.CrawlRepository
	dashboardRepository  *repository.DashboardRepository
}

func NewContainer(configFile string) *Container {
	c := &Container{}
	c.InitConfig(configFile)
	c.InitDB()
	c.InitArchiveService()
	c.InitRepositories()
	c.InitPubSubBroker()
	c.InitIssueService()
	c.InitReportService()
	c.InitReportManager()
	c.InitTranslator()
	c.InitUserService()
	c.InitSecurity()
	c.InitDashboardService()
	c.InitProjectService()
	c.InitProjectViewService()
	c.InitExportService()
	c.InitCrawlerService()
	c.InitRenderer()
	c.InitCookieSession()
	c.InitReplayService()
	c.InitAuditService()

	return c
}

// Load config file using the parameters in configFile.
func (c *Container) InitConfig(configFile string) {
	config, err := config.NewConfig(configFile)
	if err != nil {
		log.Fatalf("Error loading config: %v\n", err)
	}

	c.Config = config
}

// Create the sql database connection and run migrations.
func (c *Container) InitDB() {
	db, err := repository.SqlConnect(c.Config.DB)
	if err != nil {
		log.Fatalf("Error creating new database connection: %v", err)
	}

	driver, err := mysql.WithInstance(db, &mysql.Config{})
	if err != nil {
		log.Fatalf("Error creating mysql driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "mysql", driver)
	if err != nil {
		log.Fatalf("Error with mysql migrations: %v", err)
	}

	m.Up()

	c.db = db
}

// Create the data repositories.
func (c *Container) InitRepositories() {
	c.issueRepository = &repository.IssueRepository{DB: c.db}
	c.pageReportRepository = &repository.PageReportRepository{DB: c.db}
	c.userRepository = &repository.UserRepository{DB: c.db}
	c.projectRepository = &repository.ProjectRepository{DB: c.db}
	c.exportRepository = &repository.ExportRepository{DB: c.db}
	c.crawlRepository = &repository.CrawlRepository{DB: c.db}
	c.dashboardRepository = &repository.DashboardRepository{DB: c.db}

	// Clean up unfinished crawls.
	c.crawlRepository.DeleteUnfinishedCrawls()
}

// Create the PubSub broker.
func (c *Container) InitPubSubBroker() {
	c.PubSubBroker = NewPubSubBroker()
}

// Create the issue service.
func (c *Container) InitIssueService() {
	c.IssueService = NewIssueService(c.issueRepository)
}

// Create the report service.
func (c *Container) InitReportService() {
	repository := &struct {
		*repository.PageReportRepository
		*repository.IssueRepository
	}{
		c.pageReportRepository,
		c.issueRepository,
	}

	c.ReportService = NewReportService(repository)
}

// Create the report manager and add all the available reporters.
func (c *Container) InitReportManager() {
	c.ReportManager = NewReportManager(c.issueRepository)
	for _, r := range page.GetAllReporters() {
		c.ReportManager.AddPageReporter(r)
	}

	// Create the sql multipage reporters and add them all to the reporterManager.
	sqlReporters := multipage.NewSqlReporter(c.db)
	for _, r := range sqlReporters.GetAllReporters() {
		c.ReportManager.AddMultipageReporter(r)
	}
}

// Create the user service.
func (c *Container) InitUserService() {
	c.UserService = NewUserService(c.userRepository, c.Translator)
}

// Create the Project service.
func (c *Container) InitProjectService() {
	repository := &struct {
		*repository.ProjectRepository
		*repository.CrawlRepository
	}{
		c.projectRepository,
		c.crawlRepository,
	}

	c.ProjectService = NewProjectService(repository, c.ArchiveService)

	// UserService DeleteHooks are called when a user is deleted.
	// Add a DeleteHook so it deletes all user projects and crawl
	// data when a user is deleted.
	c.UserService.AddDeleteHook(c.ProjectService.DeleteAllUserProjects)
}

// Create the ProjectView service.
func (c *Container) InitProjectViewService() {
	repository := &struct {
		*repository.ProjectRepository
		*repository.CrawlRepository
	}{
		c.projectRepository,
		c.crawlRepository,
	}

	c.ProjectViewService = NewProjectViewService(repository)
}

// Create the Export service.
func (c *Container) InitExportService() {
	c.ExportService = NewExporter(c.exportRepository, c.Translator)
}

// Create Crawler service.
func (c *Container) InitCrawlerService() {
	crawlerServices := CrawlerServicesContainer{
		Broker:           c.PubSubBroker,
		ReportManager:    c.ReportManager,
		CrawlerHandler:   NewCrawlerHandler(c.pageReportRepository, c.PubSubBroker, c.ReportManager),
		ArchiveService:   c.ArchiveService,
		Config:           c.Config.Crawler,
		MaxConcurrent:    c.Config.Limits.MaxConcurrentCrawls,
		DenyIgnoreRobots: !c.Config.Limits.AllowIgnoreRobots,
	}
	repository := &struct {
		*repository.CrawlRepository
		*repository.IssueRepository
	}{
		c.crawlRepository,
		c.issueRepository,
	}

	c.CrawlerService = NewCrawlerService(repository, crawlerServices)
}

// Create the dashboCallbackBuilderard service.
func (c *Container) InitDashboardService() {
	c.DashboardService = NewDashboardService(c.dashboardRepository)
}

// Create The translator.
func (c *Container) InitTranslator() {
	var err error
	c.Translator, err = NewTranslator("translations", c.Config.UIConfig.Language)
	if err != nil {
		log.Fatal(err)
	}
}

// Create html renderer.
func (c *Container) InitRenderer() {
	renderer, err := NewRenderer(&RendererConfig{
		TemplatesFolder:      "web/templates",
		AllowCustomUserAgent: c.Config.Limits.AllowCustomUserAgent,
		AllowIgnoreRobots:    c.Config.Limits.AllowIgnoreRobots,
		TurnstileSiteKey:     c.Turnstile.SiteKey,
	}, c.Translator)
	if err != nil {
		log.Fatal(err)
	}

	c.Renderer = renderer
}

// Create cookie session handler
func (c *Container) InitCookieSession() {
	authKey, err1 := base64.StdEncoding.DecodeString(c.Config.HTTPServer.SessionAuthKey)
	encKey, err2 := base64.StdEncoding.DecodeString(c.Config.HTTPServer.SessionEncryptionKey)
	if err1 != nil || err2 != nil || len(authKey) != 64 || len(encKey) != 32 {
		if c.Config.HTTPServer.SessionAuthKey != "" || c.Config.HTTPServer.SessionEncryptionKey != "" {
			log.Fatal("session keys must be base64: 64 bytes for session_auth_key, 32 for session_encryption_key")
		}
		authKey, encKey = nil, nil
	}

	secure := strings.HasPrefix(c.Config.HTTPServer.URL, "https://")
	c.CookieSession = NewCookieSession(c.userRepository, authKey, encKey, secure)
}

// InitSecurity sets up client IP resolution, rate limits, Turnstile, the network
// guard used to validate project URLs and, when a mail relay is configured,
// e-mail verification of new accounts.
func (c *Container) InitSecurity() {
	var err error
	c.ClientIP, err = security.NewClientIPResolver(c.Config.HTTPServer.TrustedProxies, c.Config.HTTPServer.ClientIPHeader)
	if err != nil {
		log.Fatalf("invalid trusted_proxies: %v", err)
	}

	c.NetGuard, err = netguard.New(c.Config.Crawler.DenyAddresses, c.Config.Crawler.AllowedPorts)
	if err != nil {
		log.Fatal(err)
	}

	l := c.Config.Limits
	c.SignupLimiter = security.NewRateLimiter(l.SignupsPerIPPerDay, 24*time.Hour)
	c.SigninIPLimiter = security.NewRateLimiter(l.SigninAttemptsPerIPPerHour, time.Hour)
	c.SigninEmailLimiter = security.NewRateLimiter(l.SigninAttemptsPerEmailHour, time.Hour)
	c.CrawlLimiter = security.NewRateLimiter(l.CrawlsPerUserPerDay, 24*time.Hour)

	c.Turnstile = security.NewTurnstile(c.Config.Turnstile.SiteKey, c.Config.Turnstile.SecretKey).WithQABypass(c.Config.Turnstile.QABypass)

	if c.Config.Mail.Host == "" && c.Config.Mail.Sendmail == "" {
		return
	}
	mailer, err := NewMailer(c.Config.Mail)
	if err != nil {
		log.Fatal(err)
	}
	c.Mailer = mailer
	mailLimiter := security.NewRateLimiter(l.VerificationMailsPerDay, 24*time.Hour)
	c.UserService.EnableEmailVerification(c.userRepository, mailer, c.Config.HTTPServer.URL, mailLimiter)

	go func() {
		for {
			c.UserService.PurgeUnverified()
			time.Sleep(time.Hour)
		}
	}()
}

// Init the WACZ archiver service.
func (c *Container) InitArchiveService() {
	c.ArchiveService = NewArchiveService("archive")
}

// Init the WACZ archive replay service.
func (c *Container) InitReplayService() {
	c.ReplayService = NewReplayService()
}

// InitAuditService sets up the free on-demand audit when it is enabled.
func (c *Container) InitAuditService() {
	ac := c.Config.Audit
	if !ac.Enabled {
		return
	}

	httpClient, err := newCrawlerHTTPClient(c.Config.Crawler)
	if err != nil {
		log.Fatal(err)
	}

	crawlLimit := CrawlLimit
	if c.Config.Crawler.CrawlLimit > 0 {
		crawlLimit = c.Config.Crawler.CrawlLimit
	}
	if ac.MaxPages > 0 && ac.MaxPages < crawlLimit {
		crawlLimit = ac.MaxPages
	}

	var interpreter *audit.Interpreter
	if len(ac.AICommand) > 0 {
		interpreter = &audit.Interpreter{
			AI:       &audit.GatewayCompleter{Command: ac.AICommand, Model: ac.AIModel, Effort: ac.AIEffort},
			Language: ac.LanguageName,
			Banned:   loadLines("translations/audit_banned_phrases." + ac.Language + ".txt"),
		}
	}

	c.AuditIPLimiter = security.NewRateLimiter(ac.PerIPPerDay, 24*time.Hour)
	c.AuditService = &audit.Service{
		Store:  &audit.Store{DB: c.db},
		Reader: &audit.CrawlReader{DB: c.db},
		Runner: &AuditCrawlRunner{
			Projects:  c.ProjectService,
			Crawler:   c.CrawlerService,
			Broker:    c.PubSubBroker,
			Users:     c.userRepository,
			ProjRepo:  c.projectRepository,
			CrawlRepo: c.crawlRepository,
			BotEmail:  ac.BotEmail,
			UserAgent: c.Config.Crawler.Agent,
			MaxPages:  crawlLimit,
		},
		Interpreter: interpreter,
		Mailer:      c.Mailer,
		HTTP:        httpClient,
		Trans:       c.Translator.Trans,
		Cfg: audit.Config{
			BaseURL:      c.Config.HTTPServer.URL,
			NotifyEmail:  ac.NotifyEmail,
			DefaultLang:  ac.Language,
			LanguageName: ac.LanguageName,
			UserAgent:    c.Config.Crawler.Agent,
			CrawlLimit:   crawlLimit,
			KeyPages:     ac.KeyPages,
			DailyCap:     ac.DailyCap,
			PerEmailDay:  ac.PerEmailPerDay,
			ReuseFor:     time.Duration(ac.ReuseHours) * time.Hour,
			CrawlTimeout: time.Duration(ac.CrawlTimeoutMinutes) * time.Minute,
			KeepProjects: ac.KeepProjects,
		},
	}
	c.AuditService.Start(context.Background())
}

// loadLines reads a text file into its non-empty lines; a missing file gives none.
func loadLines(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
