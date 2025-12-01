package main

import (
	"io"
	"os"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func main() {
	// Connect to database
	// Initialize simple logger to stdout
	mw := io.MultiWriter(os.Stdout)
	logger.Init(false, mw)

	db, err := gorm.Open(sqlite.Open("./data/charon.db"), &gorm.Config{})
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
		if result.Error != nil {
			logger.Log().WithField("server", server.Name).WithError(result.Error).Error("Failed to seed remote server")
		} else if result.RowsAffected > 0 {
			logger.Log().WithField("server", server.Name).Infof("✓ Created remote server: %s (%s:%d)", server.Name, server.Host, server.Port)
		} else {
			logger.Log().WithField("server", server.Name).Info("Remote server already exists")
		}
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
		if result.Error != nil {
			logger.Log().WithField("host", util.SanitizeForLog(host.DomainNames)).WithError(result.Error).Error("Failed to seed proxy host")
		} else if result.RowsAffected > 0 {
			logger.Log().WithField("host", util.SanitizeForLog(host.DomainNames)).Infof("✓ Created proxy host: %s -> %s://%s:%d", host.DomainNames, host.ForwardScheme, host.ForwardHost, host.ForwardPort)
		} else {
			logger.Log().WithField("host", util.SanitizeForLog(host.DomainNames)).Info("Proxy host already exists")
		}
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
		if result.Error != nil {
			logger.Log().WithField("setting", setting.Key).WithError(result.Error).Error("Failed to seed setting")
		} else if result.RowsAffected > 0 {
			logger.Log().WithField("setting", setting.Key).Infof("✓ Created setting: %s = %s", setting.Key, setting.Value)
		} else {
			logger.Log().WithField("setting", setting.Key).Info("Setting already exists")
		}
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
	// Find by email first
	if err := db.Where("email = ?", user.Email).First(&existing).Error; err != nil {
		// Not found -> create
		result := db.Create(&user)
		if result.Error != nil {
			logger.Log().WithError(result.Error).Error("Failed to seed user")
		} else if result.RowsAffected > 0 {
			logger.Log().WithField("user", user.Email).Infof("✓ Created default user: %s", user.Email)
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
	// result handling is done inline above

	logger.Log().Info("\n✓ Database seeding completed successfully!")
	logger.Log().Info("  You can now start the application and see sample data.")
}
