package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
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

type Service struct {
	queries *gen.Queries
}

func NewService(queries *gen.Queries) *Service {
	return &Service{queries: queries}
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

func (s *Service) Register(ctx context.Context, email, slug, password string) (*gen.User, *gen.Session, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, nil, err
	}
	if len(password) < 8 {
		return nil, nil, fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.queries.CreateUser(ctx, gen.CreateUserParams{
		Email:        email,
		Slug:         slug,
		PasswordHash: hash,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create user: %w", err)
	}

	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}

	return &user, session, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*gen.GetUserByEmailRow, *gen.Session, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid email or password")
	}

	if !CheckPassword(user.PasswordHash, password) {
		return nil, nil, fmt.Errorf("invalid email or password")
	}

	session, err := s.createSession(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}

	return &user, session, nil
}

func (s *Service) ValidateSession(ctx context.Context, sessionID string) (*gen.GetUserByIDRow, error) {
	session, err := s.queries.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}

	user, err := s.queries.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	_ = s.queries.TouchSession(ctx, gen.TouchSessionParams{
		ID:        sessionID,
		ExpiresAt: time.Now().Add(sessionDuration),
	})

	return &user, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	return s.queries.DeleteSession(ctx, sessionID)
}

func (s *Service) createSession(ctx context.Context, userID uuid.UUID) (*gen.Session, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	session, err := s.queries.CreateSession(ctx, gen.CreateSessionParams{
		ID:        token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionDuration),
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &session, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
