package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/mostransport/vsm-trainer/internal/domain"
	"github.com/mostransport/vsm-trainer/internal/repo"
)

var (
	ErrEmailTaken   = errors.New("email already registered")
	ErrInvalidCreds = errors.New("invalid email or password")
	ErrInvalidToken = errors.New("invalid token")
)

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

type AuthService struct {
	store      repo.Store
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewAuthService(store repo.Store, jwtSecret string, accessTTL, refreshTTL time.Duration) *AuthService {
	return &AuthService{
		store:      store,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

type AuthResult struct {
	Player domain.Player `json:"player"`
	Tokens TokenPair     `json:"tokens"`
}

func (s *AuthService) Register(ctx context.Context, email, username, password string) (AuthResult, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}

	player, err := s.store.CreatePlayer(ctx, email, username, string(hash))
	if err != nil {
		if isUniqueViolation(err) {
			return AuthResult{}, ErrEmailTaken
		}
		return AuthResult{}, err
	}

	tokens, err := s.issueTokens(ctx, player.ID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Player: player, Tokens: tokens}, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	player, err := s.store.GetPlayerByEmail(ctx, email)
	if err != nil {
		return AuthResult{}, ErrInvalidCreds
	}
	if bcrypt.CompareHashAndPassword([]byte(player.PasswordHash), []byte(password)) != nil {
		return AuthResult{}, ErrInvalidCreds
	}

	tokens, err := s.issueTokens(ctx, player.ID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Player: player, Tokens: tokens}, nil
}

func (s *AuthService) issueTokens(ctx context.Context, playerID uuid.UUID) (TokenPair, error) {
	access, err := s.signAccess(playerID)
	if err != nil {
		return TokenPair{}, err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return TokenPair{}, err
	}
	refresh := hex.EncodeToString(raw)

	if err := s.store.CreateRefreshToken(ctx, playerID, hashRefreshToken(refresh), time.Now().Add(s.refreshTTL)); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

// Refresh validates a refresh token, revokes it and issues a fresh pair.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (AuthResult, error) {
	hash := hashRefreshToken(refreshToken)
	playerID, err := s.store.ConsumeRefreshToken(ctx, hash)
	if err != nil {
		return AuthResult{}, ErrInvalidToken
	}

	player, err := s.store.GetPlayerByID(ctx, playerID)
	if err != nil {
		return AuthResult{}, err
	}
	tokens, err := s.issueTokens(ctx, playerID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Player: player, Tokens: tokens}, nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type claims struct {
	PlayerID string `json:"player_id"`
	jwt.RegisteredClaims
}

func (s *AuthService) signAccess(playerID uuid.UUID) (string, error) {
	now := time.Now()
	c := claims{
		PlayerID: playerID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   playerID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.jwtSecret)
}

// ParseAccess validates an access token and returns the player id.
func (s *AuthService) ParseAccess(tokenString string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}
	c, ok := token.Claims.(*claims)
	if !ok {
		return uuid.Nil, ErrInvalidToken
	}
	id, err := uuid.Parse(c.PlayerID)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return id, nil
}
