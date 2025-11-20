package services

import (
	"errors"
	"time"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuthService struct {
	db     *gorm.DB
	config config.Config
}

func NewAuthService(db *gorm.DB, cfg config.Config) *AuthService {
	return &AuthService{db: db, config: cfg}
}

type Claims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func (s *AuthService) Register(email, password, name string) (*models.User, error) {
	var count int64
	s.db.Model(&models.User{}).Count(&count)

	role := "user"
	if count == 0 {
		role = "admin" // First user is admin
	}

	user := &models.User{
		UUID:      uuid.New().String(),
		Email:     email,
		Name:      name,
		Role:      role,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(email, password string) (string, error) {
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return "", errors.New("invalid credentials")
	}

	if !user.Enabled {
		return "", errors.New("account disabled")
	}

	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return "", errors.New("account locked")
	}

	if !user.CheckPassword(password) {
		user.FailedLoginAttempts++
		if user.FailedLoginAttempts >= 5 {
			lockTime := time.Now().Add(15 * time.Minute)
			user.LockedUntil = &lockTime
		}
		s.db.Save(&user)
		return "", errors.New("invalid credentials")
	}

	// Reset failed attempts
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	now := time.Now()
	user.LastLogin = &now
	s.db.Save(&user)

	return s.GenerateToken(&user)
}

func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "cpmp",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return errors.New("user not found")
	}

	if !user.CheckPassword(oldPassword) {
		return errors.New("invalid current password")
	}

	if err := user.SetPassword(newPassword); err != nil {
		return err
	}

	return s.db.Save(&user).Error
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
