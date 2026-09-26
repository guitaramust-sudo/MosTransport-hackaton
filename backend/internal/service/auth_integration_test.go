package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo/postgres"
)

func TestPublicRegistrationCannotGrantAdminRole(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	store, err := postgres.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	email := fmt.Sprintf("admin-%s@example.invalid", uuid.NewString())
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err == nil {
			_, _ = conn.Exec(context.Background(), `DELETE FROM players WHERE email = $1`, email)
			_ = conn.Close(context.Background())
		}
	})
	const password = "example-secret-1234"
	auth := NewAuthService(store, "test-secret", time.Hour, time.Hour)
	registered, err := auth.Register(ctx, email, "admin", password)
	if err != nil || registered.Player.Role != domain.RoleUser {
		t.Fatalf("public registration granted admin: %+v, %v", registered.Player, err)
	}
	if _, role, err := auth.ParseAccess(registered.Tokens.AccessToken); err != nil || role != domain.RoleUser {
		t.Fatalf("public token role: %q, %v", role, err)
	}
	if _, err := store.BootstrapAdmin(ctx, email, "wrong-password"); err == nil {
		t.Fatal("existing account promoted without its password")
	}
	admin, err := store.BootstrapAdmin(ctx, email, password)
	if err != nil || admin.Role != domain.RoleAdmin {
		t.Fatalf("trusted bootstrap: %+v, %v", admin, err)
	}
	if _, _, err := auth.ParseAccess(registered.Tokens.AccessToken); err == nil {
		t.Fatal("old user token remained valid after role change")
	}
	login, err := auth.Login(ctx, email, password)
	if err != nil || login.Player.Role != domain.RoleAdmin {
		t.Fatalf("admin login: %+v, %v", login.Player, err)
	}
	if _, role, err := auth.ParseAccess(login.Tokens.AccessToken); err != nil || role != domain.RoleAdmin {
		t.Fatalf("admin token role: %q, %v", role, err)
	}
}
