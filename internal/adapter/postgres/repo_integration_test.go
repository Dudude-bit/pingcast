//go:build integration

package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kirillinakin/pingcast/internal/adapter/postgres"
	"github.com/kirillinakin/pingcast/internal/crypto"
	"github.com/kirillinakin/pingcast/internal/domain"
	"github.com/kirillinakin/pingcast/internal/port"
	"github.com/kirillinakin/pingcast/internal/sqlc/gen"
)

// createTestUser inserts a unique user directly via SQL and returns the user ID.
func createTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	uid := uuid.New()
	slug := fmt.Sprintf("user-%s", uid.String()[:8])
	email := fmt.Sprintf("%s@test.local", slug)

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, slug, password_hash) VALUES ($1, $2, $3, $4)`,
		uid, email, slug, "hash_placeholder",
	)
	require.NoError(t, err, "create test user")
	return uid
}

// generateEncryptionKey produces a random base64-encoded 32-byte key for tests.
func generateEncryptionKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(key)
}

// testCipher creates a real Encryptor for integration tests.
func testCipher(t *testing.T) port.Cipher {
	t.Helper()
	enc, err := crypto.NewEncryptor(1, map[byte]string{1: generateEncryptionKey(t)})
	require.NoError(t, err)
	return enc
}

// ---------------------------------------------------------------------------
// MonitorRepo
// ---------------------------------------------------------------------------

func TestMonitorRepo_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	q := gen.New(pool)
	repo := postgres.NewMonitorRepo(pool, q, testCipher(t))

	userID := createTestUser(t, pool)

	// --- Create ---
	monitor := &domain.Monitor{
		UserID:             userID,
		Name:               "Test HTTP Monitor",
		Type:               domain.MonitorHTTP,
		CheckConfig:        json.RawMessage(`{"url":"https://example.com","method":"GET"}`),
		IntervalSeconds:    60,
		AlertAfterFailures: 3,
		IsPaused:           false,
		IsPublic:           true,
	}

	created, err := repo.Create(ctx, monitor)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, monitor.Name, created.Name)
	assert.Equal(t, domain.MonitorHTTP, created.Type)
	assert.Equal(t, domain.StatusUnknown, created.CurrentStatus)
	assert.False(t, created.CreatedAt.IsZero())

	// --- GetByID ---
	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, created.Name, fetched.Name)
	assert.Equal(t, created.IntervalSeconds, fetched.IntervalSeconds)

	// --- Update ---
	fetched.Name = "Updated Monitor"
	fetched.IntervalSeconds = 120
	fetched.IsPublic = false
	err = repo.Update(ctx, fetched)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, fetched.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Monitor", updated.Name)
	assert.Equal(t, 120, updated.IntervalSeconds)
	assert.False(t, updated.IsPublic)

	// --- Delete (soft delete) ---
	err = repo.Delete(ctx, created.ID, userID)
	require.NoError(t, err)

	// GetByID should return ErrNotFound after soft delete (query filters
	// deleted_at IS NULL; the repo wraps pgx.ErrNoRows as domain.ErrNotFound).
	_, err = repo.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMonitorRepo_SoftDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	q := gen.New(pool)
	repo := postgres.NewMonitorRepo(pool, q, testCipher(t))

	userID := createTestUser(t, pool)

	created, err := repo.Create(ctx, &domain.Monitor{
		UserID:             userID,
		Name:               "Soft Delete Test",
		Type:               domain.MonitorHTTP,
		CheckConfig:        json.RawMessage(`{"url":"https://example.com"}`),
		IntervalSeconds:    300,
		AlertAfterFailures: 3,
	})
	require.NoError(t, err)

	// Delete the monitor (soft delete)
	err = repo.Delete(ctx, created.ID, userID)
	require.NoError(t, err)

	// ListByUserID should not return the deleted monitor
	monitors, err := repo.ListByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, monitors)

	// Verify the row still exists in DB with deleted_at set
	var deletedAt *time.Time
	err = pool.QueryRow(ctx,
		`SELECT deleted_at FROM monitors WHERE id = $1`, created.ID,
	).Scan(&deletedAt)
	require.NoError(t, err)
	require.NotNil(t, deletedAt, "deleted_at should be set")
	assert.False(t, deletedAt.IsZero(), "deleted_at should not be zero")
}

// ---------------------------------------------------------------------------
// ChannelRepo
// ---------------------------------------------------------------------------

func TestChannelRepo_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	q := gen.New(pool)
	repo := postgres.NewChannelRepo(pool, q, testCipher(t))

	userID := createTestUser(t, pool)

	// --- Create ---
	channel := &domain.NotificationChannel{
		UserID: userID,
		Name:   "Test Telegram Channel",
		Type:   domain.ChannelTelegram,
		Config: json.RawMessage(`{"chat_id":"123456"}`),
	}

	created, err := repo.Create(ctx, channel)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, channel.Name, created.Name)
	assert.Equal(t, domain.ChannelTelegram, created.Type)
	assert.True(t, created.IsEnabled)

	// --- GetByID ---
	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, created.Name, fetched.Name)

	// --- Update ---
	fetched.Name = "Updated Channel"
	fetched.IsEnabled = false
	fetched.Config = json.RawMessage(`{"chat_id":"789012"}`)
	err = repo.Update(ctx, fetched)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Channel", updated.Name)
	assert.False(t, updated.IsEnabled)

	// --- Delete (soft delete) ---
	err = repo.Delete(ctx, created.ID, userID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestChannelRepo_BindUnbind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	q := gen.New(pool)
	monitorRepo := postgres.NewMonitorRepo(pool, q, testCipher(t))
	channelRepo := postgres.NewChannelRepo(pool, q, testCipher(t))

	userID := createTestUser(t, pool)

	// Create a monitor
	monitor, err := monitorRepo.Create(ctx, &domain.Monitor{
		UserID:             userID,
		Name:               "Bind Test Monitor",
		Type:               domain.MonitorHTTP,
		CheckConfig:        json.RawMessage(`{"url":"https://example.com"}`),
		IntervalSeconds:    60,
		AlertAfterFailures: 3,
	})
	require.NoError(t, err)

	// Create a channel
	channel, err := channelRepo.Create(ctx, &domain.NotificationChannel{
		UserID: userID,
		Name:   "Bind Test Channel",
		Type:   domain.ChannelWebhook,
		Config: json.RawMessage(`{"url":"https://hooks.example.com/notify"}`),
	})
	require.NoError(t, err)

	// Bind channel to monitor
	err = channelRepo.BindToMonitor(ctx, monitor.ID, channel.ID)
	require.NoError(t, err)

	// ListForMonitor should return the bound channel
	channels, err := channelRepo.ListForMonitor(ctx, monitor.ID)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, channel.ID, channels[0].ID)

	// Unbind
	err = channelRepo.UnbindFromMonitor(ctx, monitor.ID, channel.ID)
	require.NoError(t, err)

	// ListForMonitor should return empty
	channels, err = channelRepo.ListForMonitor(ctx, monitor.ID)
	require.NoError(t, err)
	assert.Empty(t, channels)
}

// ---------------------------------------------------------------------------
// APIKeyRepo
// ---------------------------------------------------------------------------

func TestAPIKeyRepo_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	q := gen.New(pool)
	repo := postgres.NewAPIKeyRepo(q)

	userID := createTestUser(t, pool)

	// --- Create ---
	keyHash := fmt.Sprintf("sha256_%s", uuid.New().String())
	apiKey := &domain.APIKey{
		UserID:  userID,
		KeyHash: keyHash,
		Name:    "Test API Key",
		Scopes:  []string{"monitors:read", "monitors:write"},
	}

	created, err := repo.Create(ctx, apiKey)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, apiKey.Name, created.Name)
	assert.Equal(t, keyHash, created.KeyHash)
	assert.Nil(t, created.LastUsedAt)

	// --- GetByHash ---
	fetched, err := repo.GetByHash(ctx, keyHash)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, created.Name, fetched.Name)

	// --- ListByUser ---
	keys, err := repo.ListByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, created.ID, keys[0].ID)

	// --- Touch (verify last_used_at updated) ---
	err = repo.Touch(ctx, created.ID)
	require.NoError(t, err)

	touched, err := repo.GetByHash(ctx, keyHash)
	require.NoError(t, err)
	require.NotNil(t, touched.LastUsedAt, "last_used_at should be set after Touch")
	assert.WithinDuration(t, time.Now(), *touched.LastUsedAt, 5*time.Second)

	// --- Delete ---
	err = repo.Delete(ctx, created.ID, userID)
	require.NoError(t, err)

	_, err = repo.GetByHash(ctx, keyHash)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

// ---------------------------------------------------------------------------
// UptimeRepo
// ---------------------------------------------------------------------------

func TestUptimeRepo_RecordAndQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	q := gen.New(pool)
	uptimeRepo := postgres.NewUptimeRepo(q)

	userID := createTestUser(t, pool)

	// Create monitors directly via SQL (UptimeRepo needs only monitor IDs, and
	// monitor_uptime_hourly has a FK to monitors).
	monitorRepo := postgres.NewMonitorRepo(pool, q, testCipher(t))

	m1, err := monitorRepo.Create(ctx, &domain.Monitor{
		UserID:             userID,
		Name:               "Uptime Test 1",
		Type:               domain.MonitorHTTP,
		CheckConfig:        json.RawMessage(`{"url":"https://example.com"}`),
		IntervalSeconds:    60,
		AlertAfterFailures: 3,
	})
	require.NoError(t, err)

	m2, err := monitorRepo.Create(ctx, &domain.Monitor{
		UserID:             userID,
		Name:               "Uptime Test 2",
		Type:               domain.MonitorHTTP,
		CheckConfig:        json.RawMessage(`{"url":"https://example.org"}`),
		IntervalSeconds:    60,
		AlertAfterFailures: 3,
	})
	require.NoError(t, err)

	now := time.Now()

	// Record checks for m1: 7 success, 3 failure = 70% uptime
	for i := 0; i < 7; i++ {
		rcErr := uptimeRepo.RecordCheck(ctx, m1.ID, now, true)
		require.NoError(t, rcErr)
	}
	for i := 0; i < 3; i++ {
		rcErr := uptimeRepo.RecordCheck(ctx, m1.ID, now, false)
		require.NoError(t, rcErr)
	}

	// Record checks for m2: 10 success, 0 failure = 100% uptime
	for i := 0; i < 10; i++ {
		rcErr := uptimeRepo.RecordCheck(ctx, m2.ID, now, true)
		require.NoError(t, rcErr)
	}

	since := now.Add(-1 * time.Hour)

	// --- GetUptime for m1 ---
	uptime1, err := uptimeRepo.GetUptime(ctx, m1.ID, since)
	require.NoError(t, err)
	assert.InDelta(t, 70.0, uptime1, 0.1, "m1 uptime should be ~70%%")

	// --- GetUptime for m2 ---
	uptime2, err := uptimeRepo.GetUptime(ctx, m2.ID, since)
	require.NoError(t, err)
	assert.InDelta(t, 100.0, uptime2, 0.1, "m2 uptime should be 100%%")

	// --- GetUptimeBatch ---
	batch, err := uptimeRepo.GetUptimeBatch(ctx, []uuid.UUID{m1.ID, m2.ID}, since)
	require.NoError(t, err)
	require.Len(t, batch, 2)
	assert.InDelta(t, 70.0, batch[m1.ID], 0.1)
	assert.InDelta(t, 100.0, batch[m2.ID], 0.1)
}

// ---------------------------------------------------------------------------
// Encryption roundtrip
// ---------------------------------------------------------------------------

func TestEncryption_Roundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	q := gen.New(pool)

	keyBase64 := generateEncryptionKey(t)
	enc, err := crypto.NewEncryptor(1, map[byte]string{1: keyBase64})
	require.NoError(t, err)

	repo := postgres.NewMonitorRepo(pool, q, enc)

	userID := createTestUser(t, pool)

	sensitiveConfig := json.RawMessage(`{"url":"https://secret.example.com","headers":{"Authorization":"Bearer super-secret-token"}}`)

	// Create monitor with sensitive config
	created, err := repo.Create(ctx, &domain.Monitor{
		UserID:             userID,
		Name:               "Encrypted Monitor",
		Type:               domain.MonitorHTTP,
		CheckConfig:        sensitiveConfig,
		IntervalSeconds:    60,
		AlertAfterFailures: 3,
	})
	require.NoError(t, err)

	// Read it back via repo (should be decrypted)
	fetched, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)

	var expectedCfg, fetchedCfg map[string]interface{}
	require.NoError(t, json.Unmarshal(sensitiveConfig, &expectedCfg))
	require.NoError(t, json.Unmarshal(fetched.CheckConfig, &fetchedCfg))
	assert.Equal(t, expectedCfg, fetchedCfg, "decrypted config should match original")

	// Verify raw DB value is encrypted (not plaintext)
	var rawConfig []byte
	err = pool.QueryRow(ctx,
		`SELECT check_config FROM monitors WHERE id = $1`, created.ID,
	).Scan(&rawConfig)
	require.NoError(t, err)

	// The encrypted value is stored as a JSON string (base64-encoded ciphertext)
	assert.NotContains(t, string(rawConfig), "super-secret-token",
		"raw DB value should NOT contain plaintext secret")
	assert.NotContains(t, string(rawConfig), "Bearer",
		"raw DB value should NOT contain plaintext Bearer token")
}

func TestUserRepo_Create_DuplicateEmailReturnsErrUserExists(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := SetupTestDB(t)
	defer cleanup()

	q := gen.New(pool)
	repo := postgres.NewUserRepo(q)

	uniq := uuid.New().String()[:8]
	email := fmt.Sprintf("dup-%s@example.com", uniq)

	_, err := repo.Create(ctx, email, fmt.Sprintf("slug1-%s", uniq), "hash1")
	require.NoError(t, err, "first create")

	_, err = repo.Create(ctx, email, fmt.Sprintf("slug2-%s", uniq), "hash2")
	require.Error(t, err, "duplicate email should error")
	require.ErrorIs(t, err, domain.ErrUserExists,
		"expected domain.ErrUserExists, got %v", err)
}
