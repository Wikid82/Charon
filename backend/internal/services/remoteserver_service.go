package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// RemoteServerService encapsulates business logic for remote server management.
type RemoteServerService struct {
	db *gorm.DB
}

// NewRemoteServerService creates a new remote server service.
func NewRemoteServerService(db *gorm.DB) *RemoteServerService {
	return &RemoteServerService{db: db}
}

// ValidateUniqueServer ensures no duplicate name+host+port combinations.
func (s *RemoteServerService) ValidateUniqueServer(name, host string, port int, excludeID uint) error {
	var count int64
	query := s.db.Model(&models.RemoteServer{}).Where("name = ? OR (host = ? AND port = ?)", name, host, port)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	if err := query.Count(&count).Error; err != nil {
		return fmt.Errorf("checking server uniqueness: %w", err)
	}

	if count > 0 {
		return errors.New("server with same name or host:port already exists")
	}

	return nil
}

// Create validates and creates a new remote server.
func (s *RemoteServerService) Create(server *models.RemoteServer) error {
	if err := s.ValidateUniqueServer(server.Name, server.Host, server.Port, 0); err != nil {
		return err
	}

	return s.db.Create(server).Error
}

// Update validates and updates an existing remote server.
func (s *RemoteServerService) Update(server *models.RemoteServer) error {
	if err := s.ValidateUniqueServer(server.Name, server.Host, server.Port, server.ID); err != nil {
		return err
	}

	return s.db.Save(server).Error
}

// Delete removes a remote server.
func (s *RemoteServerService) Delete(id uint) error {
	return s.db.Delete(&models.RemoteServer{}, id).Error
}

// GetByID retrieves a remote server by ID.
func (s *RemoteServerService) GetByID(id uint) (*models.RemoteServer, error) {
	var server models.RemoteServer
	if err := s.db.Where("id = ?", id).First(&server).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

// GetByUUID retrieves a remote server by UUID.
func (s *RemoteServerService) GetByUUID(uuidStr string) (*models.RemoteServer, error) {
	var server models.RemoteServer
	if err := s.db.Where("uuid = ?", uuidStr).First(&server).Error; err != nil {
		return nil, err
	}
	return &server, nil
}

// List retrieves all remote servers, optionally filtering by enabled status.
func (s *RemoteServerService) List(enabledOnly bool) ([]models.RemoteServer, error) {
	var servers []models.RemoteServer
	query := s.db

	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}

	if err := query.Order("name ASC").Find(&servers).Error; err != nil {
		return nil, err
	}
	return servers, nil
}

// remoteServerTestTimeout bounds the TCP dial of a remote server connection test.
const remoteServerTestTimeout = 5 * time.Second

// TestDial checks the destination and dials host:port once. It returns the time
// the connection took. A destination the probe policy refuses returns
// ErrDestinationRefused; a dial that does not connect, including one refused at
// connect time, returns ErrConnectionTestFailed.
func (s *RemoteServerService) TestDial(ctx context.Context, host string, port int) (time.Duration, error) {
	if err := CheckProbeTarget(ctx, s.db, host, port); err != nil {
		return 0, err
	}
	start := time.Now()
	conn, err := NewProbeDialer(s.db, remoteServerTestTimeout).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 0, ErrConnectionTestFailed
	}
	if closeErr := conn.Close(); closeErr != nil {
		logger.Log().WithError(closeErr).Warn("failed to close remote server test connection")
	}
	return time.Since(start), nil
}
