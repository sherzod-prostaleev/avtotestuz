package admin

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/auth"
)

var (
	ErrInvalidCreds = errors.New("invalid email or password")
	ErrDisabled     = errors.New("admin account disabled")
)

// Service handles staff login/session lifecycle.
type Service struct {
	Store  Store
	Secret []byte
}

// TokenPair is returned on login/refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// MeResponse is GET /admin/v1/me.
type MeResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
}

func (s Service) Login(ctx context.Context, email, password, ua string, ip *net.IP) (TokenPair, MeResponse, error) {
	u, err := s.Store.GetUserByEmail(ctx, email)
	if err != nil {
		if IsNoRows(err) {
			return TokenPair{}, MeResponse{}, ErrInvalidCreds
		}
		return TokenPair{}, MeResponse{}, err
	}
	if u.Status != "active" {
		return TokenPair{}, MeResponse{}, ErrDisabled
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		return TokenPair{}, MeResponse{}, ErrInvalidCreds
	}
	pair, err := s.issuePair(ctx, u, ua, ip)
	if err != nil {
		return TokenPair{}, MeResponse{}, err
	}
	me, err := s.meFromUser(ctx, u)
	return pair, me, err
}

func (s Service) Refresh(ctx context.Context, refreshRaw, ua string, ip *net.IP) (TokenPair, error) {
	hash := HashToken(refreshRaw)
	sess, err := s.Store.FindSessionByRefreshHash(ctx, hash)
	if err != nil {
		return TokenPair{}, ErrInvalidCreds
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = s.Store.RevokeSession(ctx, sess.ID)
		return TokenPair{}, ErrInvalidCreds
	}
	u, err := s.Store.GetUserByID(ctx, sess.UserID)
	if err != nil || u.Status != "active" {
		return TokenPair{}, ErrDisabled
	}
	newRaw := NewRefreshToken()
	newHash := HashToken(newRaw)
	expires := time.Now().Add(refreshTTL)
	if err := s.Store.RotateSession(ctx, sess.ID, newHash, expires); err != nil {
		return TokenPair{}, err
	}
	access, err := IssueAccess(s.Secret, u.ID, u.Email, accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	_ = ua
	_ = ip
	return TokenPair{
		AccessToken:  access,
		RefreshToken: newRaw,
		ExpiresIn:    int(accessTTL.Seconds()),
	}, nil
}

func (s Service) Logout(ctx context.Context, refreshRaw string) error {
	if strings.TrimSpace(refreshRaw) == "" {
		return nil
	}
	sess, err := s.Store.FindSessionByRefreshHash(ctx, HashToken(refreshRaw))
	if err != nil {
		return nil
	}
	return s.Store.RevokeSession(ctx, sess.ID)
}

func (s Service) Me(ctx context.Context, id uuid.UUID) (MeResponse, error) {
	u, err := s.Store.GetUserByID(ctx, id)
	if err != nil {
		return MeResponse{}, err
	}
	return s.meFromUser(ctx, u)
}

func (s Service) meFromUser(ctx context.Context, u User) (MeResponse, error) {
	roles, err := s.Store.ListRoles(ctx, u.ID)
	if err != nil {
		return MeResponse{}, err
	}
	perms, err := s.Store.ListPermissions(ctx, u.ID)
	if err != nil {
		return MeResponse{}, err
	}
	if roles == nil {
		roles = []string{}
	}
	if perms == nil {
		perms = []string{}
	}
	return MeResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Roles:       roles,
		Permissions: perms,
	}, nil
}

func (s Service) issuePair(ctx context.Context, u User, ua string, ip *net.IP) (TokenPair, error) {
	raw := NewRefreshToken()
	hash := HashToken(raw)
	expires := time.Now().Add(refreshTTL)
	if _, err := s.Store.CreateSession(ctx, u.ID, hash, ua, ip, expires); err != nil {
		return TokenPair{}, err
	}
	access, err := IssueAccess(s.Secret, u.ID, u.Email, accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(accessTTL.Seconds()),
	}, nil
}
