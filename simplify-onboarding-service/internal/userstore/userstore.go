// Package userstore resolves a Zitadel sub to the platform-internal app_users.id
// and reads the user's DocFlow tenant + role context. Ported from docflow-auth so
// the session this service writes is a first-class DocFlow session — letting
// DocFlow (and other products on the shared DB/Redis) pick it up with no rewrite.
//
// Optional: only active when DATABASE_URL points at the shared platform DB.
package userstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Store wraps a pgx pool over the shared platform database.
type Store struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

// New opens a pool to the given DSN (pgbouncer-friendly: simple query protocol).
func New(ctx context.Context, dsn string, log *zap.Logger) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("userstore: parse dsn: %w", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("userstore: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("userstore: ping db: %w", err)
	}
	log.Info("userstore: connected to shared platform database")
	return &Store{pool: pool, log: log}, nil
}

// Close releases the pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Profile carries the optional Zitadel profile fields stored alongside identity.
type Profile struct {
	NickName string
	Phone    string
}

// ResolveOrCreate upserts the Zitadel sub into app_users and returns the internal
// UUID + stored role. This is the same write path docflow-auth uses on login.
func (s *Store) ResolveOrCreate(ctx context.Context, sub, email, firstName, lastName, role string, p Profile) (internalID, storedRole string, err error) {
	if role == "" {
		role = "user"
	}
	safeEmail := ""
	if strings.Contains(email, "@") {
		safeEmail = email
	}
	const q = `
INSERT INTO public.app_users (zitadel_user_id, email, first_name, last_name, nickname, phone, role)
VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), $7)
ON CONFLICT (zitadel_user_id) DO UPDATE
    SET email      = CASE WHEN EXCLUDED.email IS NOT NULL THEN EXCLUDED.email ELSE public.app_users.email END,
        first_name = COALESCE(EXCLUDED.first_name, public.app_users.first_name),
        last_name  = COALESCE(EXCLUDED.last_name,  public.app_users.last_name),
        nickname   = COALESCE(EXCLUDED.nickname,   public.app_users.nickname),
        phone      = COALESCE(EXCLUDED.phone,      public.app_users.phone),
        updated_at = NOW()
RETURNING id, role`
	err = s.pool.QueryRow(ctx, q, sub, safeEmail, firstName, lastName, p.NickName, p.Phone, role).Scan(&internalID, &storedRole)
	if err != nil {
		return "", "", fmt.Errorf("userstore: resolve: %w", err)
	}
	return internalID, storedRole, nil
}

// TenantContext is the DocFlow RBAC context derived for a user.
type TenantContext struct {
	TenantID       string
	RoleTier       int    // cerbos tier, -1 when none
	CustomRoleID   string
	CustomRoleName string
	IsMember       bool
}

// ResolveTenantContext reads the user's active DocFlow tenant + role.
func (s *Store) ResolveTenantContext(ctx context.Context, internalUserID string) TenantContext {
	tc := TenantContext{RoleTier: -1}
	tc.TenantID = s.docflowTenantID(ctx, internalUserID)
	if tc.TenantID == "" {
		return tc
	}
	tc.IsMember = true
	tc.RoleTier = s.roleTier(ctx, internalUserID, tc.TenantID)
	tc.CustomRoleID = s.scalar(ctx,
		`SELECT tm.custom_role_id::text FROM tenant_members tm
		 WHERE tm.user_id::text=$1 AND tm.tenant_id::text=$2 AND tm.is_active AND tm.status='active' AND tm.custom_role_id IS NOT NULL LIMIT 1`,
		internalUserID, tc.TenantID)
	tc.CustomRoleName = s.scalar(ctx,
		`SELECT cr.name FROM tenant_members tm JOIN tenant_custom_roles cr ON cr.id=tm.custom_role_id
		 WHERE tm.user_id::text=$1 AND tm.tenant_id::text=$2 AND tm.is_active AND tm.status='active' LIMIT 1`,
		internalUserID, tc.TenantID)
	return tc
}

func (s *Store) docflowTenantID(ctx context.Context, internalUserID string) string {
	return s.scalar(ctx,
		`SELECT tm.tenant_id::text FROM tenant_members tm
		 JOIN public.tenants t ON t.id = tm.tenant_id
		 WHERE tm.user_id::text=$1 AND tm.is_active AND tm.status='active' AND t.deleted_at IS NULL LIMIT 1`,
		internalUserID)
}

func (s *Store) roleTier(ctx context.Context, internalUserID, tenantID string) int {
	var tier int
	err := s.pool.QueryRow(ctx,
		`SELECT cr.cerbos_tier FROM tenant_members tm JOIN tenant_custom_roles cr ON cr.id=tm.custom_role_id
		 WHERE tm.user_id::text=$1 AND tm.tenant_id::text=$2 AND tm.is_active AND tm.status='active' AND tm.custom_role_id IS NOT NULL LIMIT 1`,
		internalUserID, tenantID).Scan(&tier)
	if err != nil {
		return -1
	}
	return tier
}

func (s *Store) scalar(ctx context.Context, q string, args ...any) string {
	var v string
	_ = s.pool.QueryRow(ctx, q, args...).Scan(&v)
	return v
}
