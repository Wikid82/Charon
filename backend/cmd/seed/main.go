package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

func main() {
	// Connect to database
	db, err := gorm.Open(sqlite.Open("./data/cpm.db"), &gorm.Config{})
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
			UUID:         uuid.NewString(),
			Name:         "Development App",
			Domain:       "app.local.dev",
			TargetScheme: "http",
			TargetHost:   "localhost",
			TargetPort:   3000,
			EnableTLS:    false,
			EnableWS:     true,
			Enabled:      true,
		},
		{
			UUID:         uuid.NewString(),
			Name:         "API Server",
			Domain:       "api.local.dev",
			TargetScheme: "http",
			TargetHost:   "192.168.1.100",
			TargetPort:   8080,
			EnableTLS:    false,
			EnableWS:     false,
			Enabled:      true,
		},
		{
			UUID:         uuid.NewString(),
			Name:         "Docker Registry",
			Domain:       "docker.local.dev",
			TargetScheme: "http",
			TargetHost:   "localhost",
			TargetPort:   5000,
			EnableTLS:    false,
			EnableWS:     false,
			Enabled:      false,
		},
	}

	for _, host := range proxyHosts {
		result := db.Where("domain = ?", host.Domain).FirstOrCreate(&host)
		if result.Error != nil {
			log.Printf("Failed to seed proxy host %s: %v", host.Domain, result.Error)
		} else if result.RowsAffected > 0 {
			fmt.Printf("✓ Created proxy host: %s -> %s://%s:%d\n",
				host.Domain, host.TargetScheme, host.TargetHost, host.TargetPort)
		} else {
			fmt.Printf("  Proxy host already exists: %s\n", host.Domain)
		}
	}

	// Seed Settings
	settings := []models.Setting{
		{
			Key:      "app_name",
			Value:    "Caddy Proxy Manager+",
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
	user := models.User{
		UUID:         uuid.NewString(),
		Email:        "admin@localhost",
		Name:         "Administrator",
		PasswordHash: "$2a$10$example_hashed_password", // This would be properly hashed in production
		Role:         "admin",
		Enabled:      true,
	}
	result := db.Where("email = ?", user.Email).FirstOrCreate(&user)
	if result.Error != nil {
		log.Printf("Failed to seed user: %v", result.Error)
	} else if result.RowsAffected > 0 {
		fmt.Printf("✓ Created default user: %s\n", user.Email)
	} else {
		fmt.Printf("  User already exists: %s\n", user.Email)
	}

	fmt.Println("\n✓ Database seeding completed successfully!")
	fmt.Println("  You can now start the application and see sample data.")
}
