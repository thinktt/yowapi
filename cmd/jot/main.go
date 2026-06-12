package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: jot <username> [comma-separated-roles]")
		return
	}

	username := os.Args[1]
	var roles []string

	if len(os.Args) > 2 {
		roles = strings.Split(os.Args[2], ",")
	} else {
		roles = []string{"user"}
	}

	jwtKey := os.Getenv("JWT_KEY")
	if jwtKey == "" {
		fmt.Println("Error: JWT_KEY environment variable is not set")
		return
	}

	claims := jwt.MapClaims{
		"iss":   "yeoldwizard.com",
		"sub":   username,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
		"roles": roles,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(jwtKey))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println(tokenStr)
}
