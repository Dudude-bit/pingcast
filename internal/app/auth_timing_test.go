package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/kirillinakin/pingcast/internal/adapter/sysclock"
	"github.com/kirillinakin/pingcast/internal/adapter/sysrand"
	"github.com/kirillinakin/pingcast/internal/app"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/mocks"
)

// TestLogin_TimingParityUnderShort asserts that the Login response time
// does not reveal whether an email is registered. Runs under -run (not
// -bench); skipped in -short mode because bcrypt.DefaultCost is ~100 ms.
func TestLogin_TimingParityUnderShort(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test runs full bcrypt cost")
	}

	users := mocks.NewMockUserRepo(t)
	sessions := mocks.NewMockSessionRepo(t)

	hash, err := app.HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	existingUser := &domain.User{ID: uuid.New(), Email: "present@example.com"}

	users.EXPECT().GetByEmail(mock.Anything, "present@example.com").
		Return(existingUser, hash, nil).Maybe()
	users.EXPECT().GetByEmail(mock.Anything, "absent@example.com").
		Return(nil, "", errors.New("not found")).Maybe()

	svc := app.NewAuthService(users, sessions, sysclock.New(), sysrand.New())

	const samples = 5
	var missingTotal, wrongTotal time.Duration

	// Warm up bcrypt once so the first iteration doesn't skew results.
	_, _, _ = svc.Login(context.Background(), "absent@example.com", "warmup")

	for i := 0; i < samples; i++ {
		start := time.Now()
		_, _, _ = svc.Login(context.Background(), "absent@example.com", "password")
		missingTotal += time.Since(start)

		start = time.Now()
		_, _, _ = svc.Login(context.Background(), "present@example.com", "wrong-password")
		wrongTotal += time.Since(start)
	}

	missingAvg := missingTotal / samples
	wrongAvg := wrongTotal / samples

	ratio := float64(wrongAvg) / float64(missingAvg)
	if ratio > 3.0 || ratio < 0.33 {
		t.Fatalf("timing parity broken: missing=%s wrong=%s ratio=%.2fx (expected within 3x)",
			missingAvg, wrongAvg, ratio)
	}
	t.Logf("timing parity OK: missing=%s wrong=%s ratio=%.2fx", missingAvg, wrongAvg, ratio)
}
