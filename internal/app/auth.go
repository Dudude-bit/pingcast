package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 30 * 24 * time.Hour

var (
	slugRegex     = regexp.MustCompile(`^[a-z0-9-]{3,30}$`)
	reservedSlugs = map[string]bool{
		"login": true, "logout": true, "register": true, "api": true,
		"admin": true, "status": true, "health": true, "webhook": true,
		"pricing": true, "docs": true, "app": true, "dashboard": true,
		"settings": true, "billing": true,
	}
)

type AuthService struct {
	users    port.UserRepo
	sessions port.SessionRepo
}

func NewAuthService(users port.UserRepo, sessions port.SessionRepo) *AuthService {
	return &AuthService{users: users, sessions: sessions}
}

func (s *AuthService) Register(ctx context.Context, email, slug, password string) (*domain.User, string, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, "", err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, "", err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	user, err := s.users.Create(ctx, email, slug, hash)
	if err != nil {
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	sessionID, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, sessionID, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	user, hash, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", fmt.Errorf("invalid email or password")
	}

	if !CheckPassword(hash, password) {
		return nil, "", fmt.Errorf("invalid email or password")
	}

	sessionID, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, sessionID, nil
}

func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (*domain.User, error) {
	userID, err := s.sessions.GetUserID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	_ = s.sessions.Touch(ctx, sessionID, time.Now().Add(sessionDuration))

	return user, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	return s.sessions.Delete(ctx, sessionID)
}

func (s *AuthService) UpgradeToPro(ctx context.Context, userID uuid.UUID, customerID, subscriptionID string) error {
	if err := s.users.UpdatePlan(ctx, userID, domain.PlanPro); err != nil {
		return err
	}
	return s.users.UpdateLemonSqueezy(ctx, userID, customerID, subscriptionID)
}

func (s *AuthService) DowngradeToFree(ctx context.Context, userID uuid.UUID) error {
	return s.users.UpdatePlan(ctx, userID, domain.PlanFree)
}

func (s *AuthService) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, _, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return user, nil
}

func (s *AuthService) createSession(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	if err := s.sessions.Create(ctx, token, userID, time.Now().Add(sessionDuration)); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func ValidateSlug(slug string) error {
	if !slugRegex.MatchString(slug) {
		return fmt.Errorf("slug must be 3-30 characters, lowercase alphanumeric and hyphens only")
	}
	if reservedSlugs[slug] {
		return fmt.Errorf("slug %q is reserved", slug)
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
