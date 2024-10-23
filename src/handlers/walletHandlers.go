package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"gitlab.com/acsermely/permit-v0/server/src/db"
)

func RegisterKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "false")

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var user UserKeyBody
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	rsaPublicKey, err := parsePublicKeyString(user.Key)
	if err != nil {
		http.Error(w, "Cannot parse Key", http.StatusBadRequest)
		return
	}

	_, err = db.GetUserKey(user.WalletID)
	if err == nil {
		http.Error(w, "Wallet already registered", http.StatusConflict)
		return
	}

	challange, err := db.InsertUserKey(user.WalletID, user.Key)
	if err != nil {
		http.Error(w, "Couldn't register user", http.StatusInternalServerError)
		return
	}

	encryptedText, err := encryptWithPublicKey(rsaPublicKey, challange)
	if err != nil {
		http.Error(w, "Failed to create Challange", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(encryptedText)
}

func encryptWithPublicKey(rsaPublicKey *rsa.PublicKey, plaintext string) ([]byte, error) {
	rng := rand.Reader
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rng, rsaPublicKey, []byte(plaintext), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}

	return ciphertext, nil
}

func parsePublicKeyString(key string) (*rsa.PublicKey, error) {
	publicJWK, err := jwk.ParseKey([]byte(key))
	if err != nil {
		return nil, err
	}

	var rsaPublicKey rsa.PublicKey
	if err := publicJWK.Raw(&rsaPublicKey); err != nil {
		return nil, err
	}
	return &rsaPublicKey, nil
}

func GetLoginChal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	walletId := r.URL.Query().Get("walletId")
	if walletId == "" {
		http.Error(w, "Missing Wallet ID", http.StatusBadRequest)
		return
	}

	user, err := db.GetUserKey(walletId)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rsaPublicKey, err := parsePublicKeyString(user.Key)
	if err != nil {
		http.Error(w, "Cannot parse Key", http.StatusBadRequest)
		return
	}

	challange, err := db.SetNewChal(walletId)
	if err != nil {
		http.Error(w, "Couldn't generate Challange", http.StatusInternalServerError)
		return
	}

	encryptedText, err := encryptWithPublicKey(rsaPublicKey, challange)
	if err != nil {
		http.Error(w, "Failed to create Challange", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(encryptedText)
}

func LoginWhitChal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	secret := []byte(os.Getenv("SECRET"))

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var loginCreds LoginKeyBody
	if err := json.NewDecoder(r.Body).Decode(&loginCreds); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	storedKey, err := db.GetUserKey(loginCreds.WalletID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if storedKey.Chal == "" || loginCreds.Chal != storedKey.Chal {
		http.Error(w, "Invalid Challange", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"authorized": true,
		"user":       loginCreds.WalletID,
		"exp":        time.Now().Add(JWT_COOKIE_EXPIRATION).Unix(),
	})

	tokenString, err := token.SignedString(secret)

	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	_ = db.DeleteChal(loginCreds.WalletID)

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Expires:  time.Now().Add(JWT_COOKIE_EXPIRATION).UTC(),
	})
}
