package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/thinktt/yowapi/pkg/db"
	"github.com/thinktt/yowapi/pkg/models"
)

var lichessUrl = "https://lichess.org/api/account"
var jwtKey = os.Getenv("JWT_KEY")

// would be good to run this through a validator since it's coming from outside
// even if lichess is  a trusted source
type LichessAccount struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type TokenRes struct {
	Token  string                 `json:"token"`
	Claims map[string]interface{} `json:"claims"`
}

func GetToken(lichessToken string) (TokenRes, error) {
	req, err := http.NewRequest("GET", lichessUrl, nil)
	if err != nil {
		fmt.Println("error creating lichess request: ", err)
		return TokenRes{}, &ServerError{"error creating lichess request"}
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", lichessToken))

	client := &http.Client{
		Timeout: time.Second * 5,
	}
	res, err := client.Do(req)
	if err != nil {
		errMsg := "error contacting lichess"
		fmt.Println(errMsg, err)
		return TokenRes{}, &ServerError{errMsg}
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		errMsg := fmt.Sprintf("lichess auth failed with: %s", res.Status)
		fmt.Println(errMsg)
		return TokenRes{}, &AuthError{errMsg}
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("error reading lichess response: ", err)
		return TokenRes{}, &ServerError{"error reading lichess response"}
	}

	var account LichessAccount
	err = json.Unmarshal(body, &account)
	if err != nil {
		fmt.Println("error parsing lichess response: ", err)
		return TokenRes{}, &ServerError{"error parsing lichess response"}
	}

	if !isValidUser(account.ID) {
		errMsg := fmt.Sprintf("no authorization found for user %s", account.ID)
		return TokenRes{}, &AuthError{errMsg}
	}
	roles := []string{"mover"}

	adminUser, exists := os.LookupEnv("ADMIN_USER")
	if !exists {
		fmt.Println("environment variable ADMIN_USER is not set")
	}

	if account.ID == adminUser {
		roles = []string{"mover", "admin"}
	}

	claims := jwt.MapClaims{
		"iss":   "yeoldwizard.com",
		"sub":   account.ID,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
		"roles": roles,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(jwtKey))
	if err != nil {
		fmt.Println("error creating token:", err)
		return TokenRes{}, &ServerError{"error creating token"}
	}

	tokenRes := TokenRes{
		Token:  tokenStr,
		Claims: claims,
	}

	return tokenRes, nil
}

func MakeToken(sub string, roles []string) (string, jwt.MapClaims, error) {
	claims := jwt.MapClaims{
		"iss":   "yeoldwizard.com",
		"sub":   sub,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
		"roles": roles,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(jwtKey))
	if err != nil {
		fmt.Println("error creating token:", err)
		return "", claims, fmt.Errorf("error creating token")
	}

	return tokenStr, claims, nil
}

func isValidUser(userID string) bool {
	user, err := db.GetUser(userID)
	if err != nil {
		fmt.Println("error getting user from db:", err)
		return false
	}

	// Check if user was found (empty struct check)
	if (user == models.User{}) {
		fmt.Println("no user found for userID:", userID)
		return false
	}

	// Validate fields
	if !user.HasAcceptedDisclaimer || user.KingCmVersion == "" {
		return false
	}

	return true
}

type ServerError struct {
	Message string
}

func (e *ServerError) Error() string {
	return e.Message
}

type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

type Claims struct {
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

func CheckToken(tokenStr string) (Claims, error) {
	claims := &Claims{}

	// parse the token
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtKey), nil
	})

	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "expired") {
			return Claims{}, &AuthError{"token is expired"}
		}

		if strings.Contains(errMsg, "malformed") {
			return Claims{}, &AuthError{"token is malformed"}
		}

		fmt.Println(err.Error())
		return Claims{}, &AuthError{"could not parse token"}
	}

	if !token.Valid {
		return Claims{}, fmt.Errorf("token is not valid")
	}

	// If we get here, everything worked and we can return the Claims
	return *claims, nil
}
