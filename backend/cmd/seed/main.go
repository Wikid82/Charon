package main

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func main() {
	// Connect to database
	db, err := gorm.Open(sqlite.Open("./data/charon.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
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
		log.Fatal("Failed to migrate database:", err)
	}

	fmt.Println("✓ Database migrated successfully")

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
			log.Printf("Failed to seed remote server %s: %v", server.Name, result.Error)
		} else if result.RowsAffected > 0 {
			fmt.Printf("✓ Created remote server: %s (%s:%d)\n", server.Name, server.Host, server.Port)
		} else {
			fmt.Printf("  Remote server already exists: %s\n", server.Name)
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
			log.Printf("Failed to seed proxy host %s: %v", host.DomainNames, result.Error)
		} else if result.RowsAffected > 0 {
			fmt.Printf("✓ Created proxy host: %s -> %s://%s:%d\n",
				host.DomainNames, host.ForwardScheme, host.ForwardHost, host.ForwardPort)
		} else {
			fmt.Printf("  Proxy host already exists: %s\n", host.DomainNames)
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
			log.Printf("Failed to seed setting %s: %v", setting.Key, result.Error)
		} else if result.RowsAffected > 0 {
			fmt.Printf("✓ Created setting: %s = %s\n", setting.Key, setting.Value)
		} else {
			fmt.Printf("  Setting already exists: %s\n", setting.Key)
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
			log.Printf("Failed to hash default admin password: %v", err)
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
			log.Printf("Failed to seed user: %v", result.Error)
		} else if result.RowsAffected > 0 {
			fmt.Printf("✓ Created default user: %s\n", user.Email)
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
					fmt.Printf("✓ Updated existing admin user password for: %s\n", existing.Email)
				} else {
					log.Printf("Failed to update existing admin password: %v", err)
				}
			} else {
				db.Save(&existing)
				fmt.Printf("  User already exists: %s\n", existing.Email)
			}
		} else {
			fmt.Printf("  User already exists: %s\n", existing.Email)
		}
	}
	// result handling is done inline above

	fmt.Println("\n✓ Database seeding completed successfully!")
	fmt.Println("  You can now start the application and see sample data.")
}
