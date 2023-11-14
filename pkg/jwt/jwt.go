package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
)

// ExtendJwtExp1Year extend a token exp time to 1 year
func ExtendJwtExp1Year(tokenString string, secret string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})
	if err != nil {
		return "", errors.Wrap(err, "error parse token")
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {

		tenantID, err := extractTenantID(tokenString, secret)
		if err != nil {
			return "", err
		}

		if tenantID != 0 { // tenantID may lose precision, we have to use this trick to make the new claims retain it
			claims["tenantId"] = tenantID
		}

		claims["exp"] = time.Now().AddDate(1, 0, 0).Unix()

		t := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
		ss, err := t.SignedString([]byte(secret))
		if err != nil {
			return "", errors.Wrap(err, "err signing jwt token")
		}

		return ss, nil
	}

	return "", errors.New("the provided token is not valid")
}

// extractTenantID extract possible `tenantId` field, if not found, return 0
func extractTenantID(tokenString string, secret string) (uint64, error) {
	type MyCustomClaims struct {
		TenantID uint64 `json:"tenantId"`
		jwt.RegisteredClaims
	}

	token, err := jwt.ParseWithClaims(tokenString, &MyCustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(secret), nil
	})
	if err != nil {
		return 0, errors.Wrap(err, "error parse token")
	}

	if claims, ok := token.Claims.(*MyCustomClaims); ok && token.Valid {
		return claims.TenantID, nil
	}

	return 0, errors.New("the provided token is not valid")
}
