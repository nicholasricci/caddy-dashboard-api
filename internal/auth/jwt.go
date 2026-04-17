package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenUseAccess  = "access"
	tokenUseRefresh = "refresh"
)

var (
	ErrInvalidTokenType = errors.New("invalid token type")
	ErrInvalidToken     = errors.New("invalid token")
)

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	TokenUse string `json:"token_use"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret        []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
	issuer        string
	audience      string
	validMethods  []string
	parserOptions []jwt.ParserOption
}

func NewJWTManager(secret string, accessTTLMinutes, refreshTTLMinutes int, issuer, audience string) *JWTManager {
	return &JWTManager{
		secret:       []byte(secret),
		accessTTL:    time.Duration(accessTTLMinutes) * time.Minute,
		refreshTTL:   time.Duration(refreshTTLMinutes) * time.Minute,
		issuer:       issuer,
		audience:     audience,
		validMethods: []string{jwt.SigningMethodHS256.Alg()},
		parserOptions: []jwt.ParserOption{
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithLeeway(30 * time.Second),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(audience),
		},
	}
}

func (m *JWTManager) GeneratePair(username, role string) (access, refresh string, err error) {
	now := time.Now()
	accessClaims := Claims{
		Username: username,
		Role:     role,
		TokenUse: tokenUseAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
		},
	}
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	access, err = at.SignedString(m.secret)
	if err != nil {
		return "", "", err
	}

	refreshClaims := Claims{
		Username: username,
		Role:     role,
		TokenUse: tokenUseRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
		},
	}
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refresh, err = rt.SignedString(m.secret)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

func (m *JWTManager) ValidateAccessToken(tokenStr string) (*Claims, error) {
	claims, err := m.parseClaims(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenUse != tokenUseAccess {
		return nil, ErrInvalidTokenType
	}
	return claims, nil
}

func (m *JWTManager) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	claims, err := m.parseClaims(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenUse != tokenUseRefresh {
		return nil, ErrInvalidTokenType
	}
	return claims, nil
}

func (m *JWTManager) parseClaims(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	}, m.parserOptions...)
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
