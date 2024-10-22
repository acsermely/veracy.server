package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"gitlab.com/acsermely/permit-v0/server/src/arweave"
	"gitlab.com/acsermely/permit-v0/server/src/db"
	"gitlab.com/acsermely/permit-v0/server/src/distributed"
	"golang.org/x/crypto/bcrypt"
)

type ProtectedResponse struct {
	User string `json:"user"`
	Hash string `json:"hash"`
}

type LoginKeyBody struct {
	WalletID string `json:"wallet"`
	Chal     string `json:"challange"`
}

const (
	JWT_COOKIE_EXPIRATION = 24 * time.Hour
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	secret := []byte(os.Getenv("SECRET"))

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var creds db.User
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	storedUser, err := db.GetUser(creds.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(creds.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"authorized": true,
		"user":       creds.Username,
		"exp":        time.Now().Add(JWT_COOKIE_EXPIRATION).Unix(),
	})

	tokenString, err := token.SignedString(secret)

	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Expires:  time.Now().Add(JWT_COOKIE_EXPIRATION).UTC(),
	})
}

func ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	storedUser := r.Context().Value(CONTEXT_USER_OBJECT_KEY).(db.User)
	response := ProtectedResponse{
		User: storedUser.Username,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "false")

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var user db.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	_, err := db.GetUser(user.Username)
	if err == nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	insertUserSQL := `INSERT INTO users (username, password) VALUES (?, ?)`

	stmt, err := db.Database.Prepare(insertUserSQL)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(user.Username, hashedPassword)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "User registered successfully")
}

func LoginCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	secret := []byte(os.Getenv("SECRET"))
	cookie, err := r.Cookie("token")
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Missing Cookie"))
		return
	}
	tokenString := cookie.Value

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
		return
	}
	var userName string
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userName = claims["user"].(string)
	} else {
		fmt.Println("invalid token")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%v", userName)
}

func Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	postId := r.FormValue("id")

	walletId := r.FormValue("walletId")

	imageData := r.FormValue("image")

	result, err := db.Database.Exec(`INSERT INTO images (wallet, post, data) VALUES (?, ?, ?)`, walletId, postId, []byte(imageData))
	if err != nil {
		http.Error(w, "Failed to store image", http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "Failed to retrieve image ID", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%d", id)
}

func Image(w http.ResponseWriter, r *http.Request) {
	fullId := r.URL.Query().Get("id")
	if fullId == "" {
		http.Error(w, "Missing image ID", http.StatusBadRequest)
		return
	}
	tx := r.URL.Query().Get("tx")

	parts := strings.Split(fullId, ":")
	wallet, post, idStr := parts[0], parts[1], parts[2]

	if tx != "" {
		paid, err := arweave.CheckPayment(wallet, tx)
		if err != nil {
			http.Error(w, "Failed to check payment", http.StatusUnauthorized)
			return
		}
		if !paid {
			http.Error(w, "Couldn't find payment", http.StatusUnauthorized)
			return
		}
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	var imageData []byte
	err = db.Database.QueryRow("SELECT data FROM images WHERE id = ? AND post = ? AND wallet = ?", id, post, wallet).Scan(&imageData)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Print("Distributed: ")
			imageData, err = distributed.NeedById(fullId)
			if err != nil {
				http.Error(w, "Image not found", http.StatusNotFound)
				fmt.Println(err)
				return
			}
			fmt.Println("OK")
		} else {
			http.Error(w, "Failed to fetch image", http.StatusInternalServerError)
			return
		}
	}

	w.Write(imageData)
}

// Handle keys for login

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
		Name:  "token",
		Value: tokenString,
		// Secure:   true,
		HttpOnly: true,
		// SameSite: http.SameSiteNoneMode,
		Expires: time.Now().Add(JWT_COOKIE_EXPIRATION).UTC(),
	})
}