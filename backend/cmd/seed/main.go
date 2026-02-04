package main

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/models"
)

func logSeedResult(entry *logrus.Entry, result *gorm.DB, errorMessage string, logCreated func(), existsMessage string) {
	switch {
	case result.Error != nil:
		entry.WithError(result.Error).Error(errorMessage)
	case result.RowsAffected > 0:
		logCreated()
	default:
		entry.Info(existsMessage)
	}
}

func main() {
	// Connect to database
	// Initialize simple logger to stdout
	mw := io.MultiWriter(os.Stdout)
	logger.Init(false, mw)

	// Configure GORM logger to ignore "record not found" errors
	// These are expected during seed operations when checking if records exist
	gormLog := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	db, err := gorm.Open(sqlite.Open("./data/charon.db"), &gorm.Config{
		Logger: gormLog,
	})
	if err != nil {
		logger.Log().WithError(err).Fatal("Failed to connect to database")
	}

	// Auto migrate
	if err := db.AutoMigrate(
		&models.User{},
		&models.ProxyHost{},
		&models.CaddyConfig{},
		&models.RemoteServer{},
		&models.SSLCertificate{},
		&models.AccessList{},
		&models.Setting{},
		&models.ImportSession{},
	); err != nil {
		logger.Log().WithError(err).Fatal("Failed to migrate database")
	}

	logger.Log().Info("✓ Database migrated successfully")

	// Seed Remote Servers
	remoteServers := []models.RemoteServer{
		{
			UUID:        uuid.NewString(),
			Name:        "Local Docker Registry",
			Provider:    "docker",
			Host:        "localhost",
			Port:        5000,
			Scheme:      "http",
			Description: "Local Docker container registry",
			Enabled:     true,
			Reachable:   false,
		},
		{
			UUID:        uuid.NewString(),
			Name:        "Development API Server",
			Provider:    "generic",
			Host:        "192.168.1.100",
			Port:        8080,
			Scheme:      "http",
			Description: "Main development API backend",
			Enabled:     true,
			Reachable:   false,
		},
		{
			UUID:        uuid.NewString(),
			Name:        "Staging Web App",
			Provider:    "vm",
			Host:        "staging.internal",
			Port:        3000,
			Scheme:      "http",
			Description: "Staging environment web application",
			Enabled:     true,
			Reachable:   false,
		},
		{
			UUID:        uuid.NewString(),
			Name:        "Database Admin",
			Provider:    "docker",
			Host:        "localhost",
			Port:        8081,
			Scheme:      "http",
			Description: "PhpMyAdmin or similar DB management tool",
			Enabled:     false,
			Reachable:   false,
		},
	}

	for _, server := range remoteServers {
		result := db.Where("host = ? AND port = ?", server.Host, server.Port).FirstOrCreate(&server)
		logEntry := logger.Log().WithField("server", server.Name)
		logSeedResult(
			logEntry,
			result,
			"Failed to seed remote server",
			func() {
				logEntry.Infof("✓ Created remote server: %s (%s:%d)", server.Name, server.Host, server.Port)
			},
			"Remote server already exists",
		)
	}

	// Seed Proxy Hosts
	proxyHosts := []models.ProxyHost{
		{
			UUID:             uuid.NewString(),
			Name:             "Development App",
			DomainNames:      "app.local.dev",
			ForwardScheme:    "http",
			ForwardHost:      "localhost",
			ForwardPort:      3000,
			SSLForced:        false,
			WebsocketSupport: true,
			HSTSEnabled:      false,
			BlockExploits:    true,
			Enabled:          true,
		},
		{
			UUID:             uuid.NewString(),
			Name:             "API Server",
			DomainNames:      "api.local.dev",
			ForwardScheme:    "http",
			ForwardHost:      "192.168.1.100",
			ForwardPort:      8080,
			SSLForced:        false,
			WebsocketSupport: false,
			HSTSEnabled:      false,
			BlockExploits:    true,
			Enabled:          true,
		},
		{
			UUID:             uuid.NewString(),
			Name:             "Docker Registry",
			DomainNames:      "docker.local.dev",
			ForwardScheme:    "http",
			ForwardHost:      "localhost",
			ForwardPort:      5000,
			SSLForced:        false,
			WebsocketSupport: false,
			HSTSEnabled:      false,
			BlockExploits:    true,
			Enabled:          false,
		},
	}

	for _, host := range proxyHosts {
		result := db.Where("domain_names = ?", host.DomainNames).FirstOrCreate(&host)
		logEntry := logger.Log().WithField("host", util.SanitizeForLog(host.DomainNames))
		logSeedResult(
			logEntry,
			result,
			"Failed to seed proxy host",
			func() {
				logEntry.Infof("✓ Created proxy host: %s -> %s://%s:%d", host.DomainNames, host.ForwardScheme, host.ForwardHost, host.ForwardPort)
			},
			"Proxy host already exists",
		)
	}

	// Seed Settings
	settings := []models.Setting{
		{
			Key:      "app_name",
			Value:    "Charon",
			Type:     "string",
			Category: "general",
		},
		{
			Key:      "default_scheme",
			Value:    "http",
			Type:     "string",
			Category: "general",
		},
		{
			Key:      "enable_ssl_by_default",
			Value:    "false",
			Type:     "bool",
			Category: "security",
		},
	}

	for _, setting := range settings {
		result := db.Where("key = ?", setting.Key).FirstOrCreate(&setting)
		logEntry := logger.Log().WithField("setting", setting.Key)
		logSeedResult(
			logEntry,
			result,
			"Failed to seed setting",
			func() {
				logEntry.Infof("✓ Created setting: %s = %s", setting.Key, setting.Value)
			},
			"Setting already exists",
		)
	}

	// Seed default admin user (for future authentication)
	defaultAdminEmail := os.Getenv("CHARON_DEFAULT_ADMIN_EMAIL")
	if defaultAdminEmail == "" {
		defaultAdminEmail = "admin@localhost"
	}
	defaultAdminPassword := os.Getenv("CHARON_DEFAULT_ADMIN_PASSWORD")
	// If a default password is not specified, leave the hashed placeholder (non-loginable)
	forceAdmin := os.Getenv("CHARON_FORCE_DEFAULT_ADMIN") == "1"

	user := models.User{
		UUID:    uuid.NewString(),
		Email:   defaultAdminEmail,
		Name:    "Administrator",
		Role:    "admin",
		Enabled: true,
	}

	// If a default password provided, use SetPassword to generate a proper bcrypt hash
	if defaultAdminPassword != "" {
		if err := user.SetPassword(defaultAdminPassword); err != nil {
			logger.Log().WithError(err).Error("Failed to hash default admin password")
		}
	} else {
		// Keep previous behavior: using example hashed password (not valid)
		user.PasswordHash = "$2a$10$example_hashed_password"
	}

	var existing models.User
	// Find by email first - use Take instead of First to avoid GORM's "record not found" log
	result := db.Where("email = ?", user.Email).Take(&existing)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Not found -> create new user
			createResult := db.Create(&user)
			if createResult.Error != nil {
				logger.Log().WithError(createResult.Error).Error("Failed to seed user")
			} else if createResult.RowsAffected > 0 {
				logger.Log().WithField("user", user.Email).Infof("✓ Created default user: %s", user.Email)
			}
		} else {
			// Unexpected error
			logger.Log().WithError(result.Error).Error("Failed to query for existing user")
		}
	} else {
		// Found existing user - optionally update if forced
		if forceAdmin {
			existing.Email = user.Email
			existing.Name = user.Name
			existing.Role = user.Role
			existing.Enabled = user.Enabled
			if defaultAdminPassword != "" {
				if err := existing.SetPassword(defaultAdminPassword); err == nil {
					db.Save(&existing)
					logger.Log().WithField("user", existing.Email).Infof("✓ Updated existing admin user password for: %s", existing.Email)
				} else {
					logger.Log().WithError(err).Error("Failed to update existing admin password")
				}
			} else {
				db.Save(&existing)
				logger.Log().WithField("user", existing.Email).Info("User already exists")
			}
		} else {
			logger.Log().WithField("user", existing.Email).Info("User already exists")
		}
	}

	logger.Log().Info("\n✓ Database seeding completed successfully!")
	logger.Log().Info("  You can now start the application and see sample data.")
}
