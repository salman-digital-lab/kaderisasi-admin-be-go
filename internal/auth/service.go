package auth

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/api/idtoken"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"slices"
	"strings"
	"time"
)

type UserView struct {
	ID          int32         `json:"id"`
	Email       string        `json:"email"`
	DisplayName *string       `json:"display_name"`
	IsActive    bool          `json:"is_active"`
	Role        *AssignedRole `json:"role"`
}
type LegacyToken struct {
	Type      string `json:"type"`
	Token     string `json:"token"`
	ExpiresIn string `json:"expiresIn"`
}
type Session struct {
	AccessToken           string      `json:"access_token"`
	AccessTokenExpiresIn  int         `json:"access_token_expires_in"`
	User                  UserView    `json:"user"`
	AuthenticationMethods []string    `json:"authentication_methods"`
	Permissions           []string    `json:"permissions"`
	IsSuperAdmin          bool        `json:"is_super_admin"`
	Token                 LegacyToken `json:"token"`
}
type ClientInfo struct {
	UserAgent *string
	IP        *string
}
type GoogleVerifier interface {
	Validate(context.Context, string, string) (*idtoken.Payload, error)
}
type Service struct {
	Pool           *pgxpool.Pool
	Key            string
	GoogleClientID string
	Google         GoogleVerifier
	Now            func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) Authenticate(ctx context.Context, token string) (dbgen.AdminUser, error) {
	claims, err := VerifyAccess(s.Key, token, s.now())
	if err != nil {
		return dbgen.AdminUser{}, domain.Fail(401, "UNAUTHORIZED")
	}
	user, err := dbgen.New(s.Pool).FindAdminByID(ctx, claims.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return user, domain.Fail(401, "UNAUTHORIZED")
	}
	if err != nil {
		return user, err
	}
	if !user.IsActive {
		return user, domain.Fail(403, "USER_INACTIVE")
	}
	return user, nil
}
func (s *Service) Login(ctx context.Context, email, password string, info ClientInfo) (Session, string, error) {
	user, err := dbgen.New(s.Pool).FindAdminByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, "", domain.Fail(404, "USER_NOT_FOUND")
	}
	if err != nil {
		return Session{}, "", err
	}
	if !user.IsActive {
		return Session{}, "", domain.Fail(403, "USER_INACTIVE")
	}
	if user.Password == nil || !VerifyPassword(*user.Password, password) {
		return Session{}, "", domain.Fail(401, "WRONG_PASSWORD")
	}
	return s.Issue(ctx, user, info)
}
func (s *Service) Build(ctx context.Context, user dbgen.AdminUser) (Session, error) {
	token, err := SignAccess(s.Key, user.ID, user.Email, s.now())
	if err != nil {
		return Session{}, err
	}
	providers, err := dbgen.New(s.Pool).AdminIdentityProviders(ctx, user.ID)
	if err != nil {
		return Session{}, err
	}
	methods := []string{}
	if user.Password != nil && *user.Password != "" {
		methods = append(methods, "password")
	}
	for _, p := range providers {
		if !slices.Contains(methods, p) {
			methods = append(methods, p)
		}
	}
	a := ForRole(user.RoleCode, user.IsActive)
	return Session{AccessToken: token, AccessTokenExpiresIn: 900, User: UserView{user.ID, user.Email, user.DisplayName, user.IsActive, a.Role}, AuthenticationMethods: methods, Permissions: a.Permissions, IsSuperAdmin: a.IsSuperAdmin, Token: LegacyToken{"bearer", token, "15m"}}, nil
}
func (s *Service) Issue(ctx context.Context, user dbgen.AdminUser, info ClientInfo) (Session, string, error) {
	value, err := RandomToken()
	if err != nil {
		return Session{}, "", err
	}
	familyText, err := UUID()
	if err != nil {
		return Session{}, "", err
	}
	var family pgtype.UUID
	if err = family.Scan(familyText); err != nil {
		return Session{}, "", err
	}
	_, err = dbgen.New(s.Pool).CreateRefresh(ctx, dbgen.CreateRefreshParams{FamilyID: family, AdminUserID: user.ID, TokenHash: HashToken(value), UserAgent: info.UserAgent, IpAddress: info.IP, ExpiresAt: pgtype.Timestamptz{Time: s.now().Add(RefreshTTL), Valid: true}})
	if err != nil {
		return Session{}, "", err
	}
	session, err := s.Build(ctx, user)
	return session, value, err
}
func (s *Service) Rotate(ctx context.Context, value string, info ClientInfo) (Session, string, error) {
	invalid := domain.Fail(401, "REFRESH_TOKEN_INVALID")
	if value == "" {
		return Session{}, "", invalid
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Session{}, "", err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	current, err := q.FindRefreshForUpdate(ctx, HashToken(value))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, "", invalid
	}
	if err != nil {
		return Session{}, "", err
	}
	if current.RevokedAt.Valid || !current.ExpiresAt.Time.After(s.now()) {
		if err = q.RevokeFamily(ctx, current.FamilyID); err != nil {
			return Session{}, "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return Session{}, "", err
		}
		return Session{}, "", invalid
	}
	user, err := q.FindAdminByID(ctx, current.AdminUserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Session{}, "", err
	}
	if !user.IsActive {
		reason := "account_inactive"
		if err = q.RevokeRefresh(ctx, dbgen.RevokeRefreshParams{TokenHash: current.TokenHash, RevocationReason: &reason}); err != nil {
			return Session{}, "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return Session{}, "", err
		}
		return Session{}, "", invalid
	}
	nextValue, err := RandomToken()
	if err != nil {
		return Session{}, "", err
	}
	next, err := q.CreateRefresh(ctx, dbgen.CreateRefreshParams{FamilyID: current.FamilyID, AdminUserID: user.ID, TokenHash: HashToken(nextValue), ParentTokenID: &current.ID, UserAgent: info.UserAgent, IpAddress: info.IP, ExpiresAt: pgtype.Timestamptz{Time: s.now().Add(RefreshTTL), Valid: true}})
	if err != nil {
		return Session{}, "", err
	}
	if err = q.RotateRefresh(ctx, dbgen.RotateRefreshParams{ID: current.ID, ReplacedByTokenID: &next.ID}); err != nil {
		return Session{}, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, "", err
	}
	session, err := s.Build(ctx, user)
	return session, nextValue, err
}
func (s *Service) Logout(ctx context.Context, value string) error {
	if value == "" {
		return nil
	}
	reason := "logout"
	return dbgen.New(s.Pool).RevokeRefresh(ctx, dbgen.RevokeRefreshParams{TokenHash: HashToken(value), RevocationReason: &reason})
}
func (s *Service) GoogleLogin(ctx context.Context, credential string, info ClientInfo) (Session, string, error) {
	if s.GoogleClientID == "" {
		return Session{}, "", domain.Fail(503, "GOOGLE_LOGIN_NOT_CONFIGURED")
	}
	payload, err := s.Google.Validate(ctx, credential, s.GoogleClientID)
	if err != nil {
		return Session{}, "", domain.Fail(401, "GOOGLE_CREDENTIAL_INVALID")
	}
	email, _ := payload.Claims["email"].(string)
	verified, _ := payload.Claims["email_verified"].(bool)
	email = strings.ToLower(strings.TrimSpace(email))
	if payload.Subject == "" || email == "" || !verified {
		return Session{}, "", domain.Fail(401, "GOOGLE_EMAIL_NOT_VERIFIED")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Session{}, "", err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	identity, err := q.FindGoogleIdentity(ctx, payload.Subject)
	var user dbgen.AdminUser
	if err == nil {
		if err = q.TouchIdentity(ctx, dbgen.TouchIdentityParams{ID: identity.ID, Email: email}); err != nil {
			return Session{}, "", err
		}
		user, err = q.FindAdminByID(ctx, identity.AdminUserID)
	} else if errors.Is(err, pgx.ErrNoRows) {
		user, err = q.FindAdminByEmail(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			name, _ := payload.Claims["name"].(string)
			name = strings.TrimSpace(name)
			if name == "" {
				name = strings.Split(email, "@")[0]
			}
			user, err = q.CreateGoogleAdmin(ctx, dbgen.CreateGoogleAdminParams{Email: email, DisplayName: &name})
		}
		if err == nil {
			err = q.CreateGoogleIdentity(ctx, dbgen.CreateGoogleIdentityParams{AdminUserID: user.ID, ProviderSubject: payload.Subject, Email: email})
		}
	}
	if err != nil {
		return Session{}, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, "", err
	}
	if !user.IsActive {
		return Session{}, "", domain.Fail(403, "USER_INACTIVE")
	}
	return s.Issue(ctx, user, info)
}
