// Package jwt 双令牌（access / refresh）签发与校验。
package jwt

import (
	"errors"
	"time"

	gjwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TypeAccess  = "access"
	TypeRefresh = "refresh"
)

var (
	ErrExpired = errors.New("token expired")
	ErrInvalid = errors.New("token invalid")
)

// Manager 负责令牌签发与解析，支持新旧密钥并存（便于密钥轮换）。
type Manager struct {
	secret     []byte
	secretPrev []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

func New(secret, secretPrev string, accessTTL, refreshTTL time.Duration, issuer string) *Manager {
	m := &Manager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     issuer,
	}
	if secretPrev != "" {
		m.secretPrev = []byte(secretPrev)
	}
	return m
}

// Claims 自定义声明：仅放 uid / tenant / jti，权限与角色不放（变更后实时生效）。
type Claims struct {
	UserID    uint64 `json:"uid"`
	TenantID  uint64 `json:"tid"`
	TokenType string `json:"typ"`
	gjwt.RegisteredClaims
}

// Token 签发结果。
type Token struct {
	Value     string    `json:"token"`
	JTI       string    `json:"jti"`
	ExpiresAt time.Time `json:"expiresAt"`
	TTL       time.Duration
}

func (m *Manager) ttlOf(typ string) time.Duration {
	if typ == TypeRefresh {
		return m.refreshTTL
	}
	return m.accessTTL
}

// Generate 签发令牌。
func (m *Manager) Generate(userID, tenantID uint64, typ string) (*Token, error) {
	now := time.Now()
	ttl := m.ttlOf(typ)
	exp := now.Add(ttl)
	jti := uuid.NewString()

	claims := Claims{
		UserID:    userID,
		TenantID:  tenantID,
		TokenType: typ,
		RegisteredClaims: gjwt.RegisteredClaims{
			ID:        jti,
			Issuer:    m.issuer,
			Subject:   typ,
			IssuedAt:  gjwt.NewNumericDate(now),
			NotBefore: gjwt.NewNumericDate(now.Add(-time.Second)),
			ExpiresAt: gjwt.NewNumericDate(exp),
		},
	}
	signed, err := gjwt.NewWithClaims(gjwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return nil, err
	}
	return &Token{Value: signed, JTI: jti, ExpiresAt: exp, TTL: ttl}, nil
}

// RefreshTTL 暴露刷新令牌有效期。
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// Parse 解析并校验令牌（签名 + 过期 + 类型）。
func (m *Manager) Parse(tokenStr, expectType string) (*Claims, error) {
	parse := func(secret []byte) (*Claims, error) {
		claims := &Claims{}
		_, err := gjwt.ParseWithClaims(tokenStr, claims, func(t *gjwt.Token) (any, error) {
			if _, ok := t.Method.(*gjwt.SigningMethodHMAC); !ok {
				return nil, ErrInvalid
			}
			return secret, nil
		}, gjwt.WithIssuer(m.issuer))
		if err != nil {
			if errors.Is(err, gjwt.ErrTokenExpired) {
				return nil, ErrExpired
			}
			return nil, ErrInvalid
		}
		return claims, nil
	}

	claims, err := parse(m.secret)
	if err != nil && m.secretPrev != nil && errors.Is(err, ErrInvalid) {
		claims, err = parse(m.secretPrev) // 密钥轮换期：兼容旧密钥
	}
	if err != nil {
		return nil, err
	}
	if expectType != "" && claims.TokenType != expectType {
		return nil, ErrInvalid
	}
	if claims.UserID == 0 || claims.ID == "" {
		return nil, ErrInvalid
	}
	return claims, nil
}
