package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	//func GenerateFromPassword(password []byte, cost int) ([]byte, error)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func CheckPasswordHash(password, hash string) error {
	//func CompareHashAndPassword(hashedPassword, password []byte) error
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userID uuid.UUID, tokenSecret string) (string, error) {
	//func NewWithClaims(method SigningMethod, claims Claims, opts ...TokenOption) *Token
	//Access Tokens expire in 1 hour automatically now.
	expiresIn := time.Hour

	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresIn)),
		Subject:   userID.String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(tokenSecret))

	//fmt.Printf("MakeJWT tokenString is: %v\n", tokenString)
	return tokenString, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	//fmt.Printf("ValidateJWT tokenString: %v\n", tokenString)
	//fmt.Printf("ValidateJWT tokenSecret: %v\n", tokenSecret)

	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(tokenSecret), nil
		})
	if err != nil {
		return uuid.Nil, errors.New("invalid token")
	}
	userIDString, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, errors.New("cannot get subject")
	}
	userIDUUID, err := uuid.Parse(userIDString)
	if err != nil {
		return uuid.Nil, errors.New("cannot parse userID")
	}
	return userIDUUID, nil
}

func GetAPIKey(headers http.Header) (string, error) {
	authValue := headers.Get("Authorization")
	if authValue == "" {
		//Did not find the header
		return "", errors.New("no authoriztaion found")
	}

	const apikeyPrefix = "ApiKey"
	if !strings.HasPrefix(authValue, apikeyPrefix) {
		return "", errors.New("authorization header format must be ApiKey {key}")
	}

	//fmt.Printf("GetBearerToken authValue: %v\n", authValue)
	return strings.TrimSpace(strings.TrimPrefix(authValue, "ApiKey")), nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authValue := headers.Get("Authorization")
	if authValue == "" {
		//Did not find the header
		return "", errors.New("no authoriztaion found")
	}

	const bearerPrefix = "Bearer"
	if !strings.HasPrefix(authValue, bearerPrefix) {
		return "", errors.New("authorization header format must be Bearer {token}")
	}

	//fmt.Printf("GetBearerToken authValue: %v\n", authValue)
	return strings.TrimSpace(strings.TrimPrefix(authValue, "Bearer")), nil
}

func MakeRefreshToken() (string, error) {
	randoBytes := make([]byte, 32)
	_, err := rand.Read(randoBytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(randoBytes), nil
}

//TEST FUNCTIONS

func TestJWTGood(userID uuid.UUID) {
	tokenSecret := "ThisIsATokenSecret"
	//dur := 60 * time.Minute - No longer used

	fmt.Println("Starting MakeJWT")
	tokenString, err := MakeJWT(userID, tokenSecret)
	fmt.Printf("tokenString: %v, err: %v\n", tokenString, err)
	fmt.Println("Starting ValidateJWT")
	validateUserID, err := ValidateJWT(tokenString, tokenSecret)
	fmt.Printf("UserID from ValidateJWT: %v, err: %v\n", validateUserID, err)
	fmt.Printf("userID passed: %v, userID returned: %v\n", userID, validateUserID)
}

func TestJWTBad(userID uuid.UUID) {
	tokenSecret := "ThisIsATokenSecret"
	//dur := 60 * time.Minute - No longer used

	fmt.Println("Starting MakeJWT")
	tokenString, err := MakeJWT(userID, tokenSecret)
	fmt.Printf("tokenString: %v, err: %v\n", tokenString, err)
	fmt.Println("Starting ValidateJWT")
	tokenString = tokenString + "bad"
	validateUserID, err := ValidateJWT(tokenString, tokenSecret)
	fmt.Printf("UserID from ValidateJWT: %v, err: %v\n", validateUserID, err)
	fmt.Printf("userID passed: %v, userID returned: %v\n", userID, validateUserID)
}
