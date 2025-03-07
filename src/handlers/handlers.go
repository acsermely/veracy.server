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

	"github.com/acsermely/veracy.server/src/arweave"
	"github.com/acsermely/veracy.server/src/db"
	"github.com/acsermely/veracy.server/src/distributed"
	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

func Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))

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

	walletId := r.URL.Query().Get("walletId")
	if walletId == "" {
		http.Error(w, "Missing Wallet ID", http.StatusBadRequest)
		return
	}

	var key string
	var challange string
	user, err := db.GetUserKey(walletId)
	if err != nil {
		keyData, err := distributed.GroupUserByAddress(walletId)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		key = string(keyData)
		challange, err = db.InsertUserKey(walletId, key)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if key == "" {
		key = user.Key
	}

	rsaPublicKey, err := parsePublicKeyString(key)
	if err != nil {
		http.Error(w, "Cannot parse Key", http.StatusBadRequest)
		return
	}

	if challange == "" {
		challange, err = db.SetNewChal(walletId)
		if err != nil {
			http.Error(w, "Couldn't generate Challange", http.StatusInternalServerError)
			return
		}
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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(tokenString))
}

func LoginCheckKey(w http.ResponseWriter, r *http.Request) {
	storedUser := r.Context().Value(CONTEXT_USER_OBJECT_KEY).(db.UserKey)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(storedUser.WalletID))
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
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	fullId := r.URL.Query().Get("id")
	if fullId == "" {
		http.Error(w, "Missing image ID", http.StatusBadRequest)
		return
	}
	tx := r.URL.Query().Get("tx")

	parts := strings.Split(fullId, ":")
	wallet, post, idStr := parts[0], parts[1], parts[2]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid image ID", http.StatusBadRequest)
		return
	}

	isPrivate, err := arweave.IsDataPrivate(fullId, tx)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Data check failed", http.StatusBadRequest)
		return
	}
	if isPrivate {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header missing", http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		secret := []byte(os.Getenv("SECRET"))
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

		var userWallet string
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userWallet = claims["user"].(string)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}

		storedUser, err := db.GetUserKey(userWallet)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		if storedUser.WalletID != wallet {
			paid, err := arweave.CheckPayment(storedUser.WalletID, tx, wallet, post)
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			if !paid {
				http.Error(w, "Couldn't find payment", http.StatusPaymentRequired)
				return
			}
		}
	}

	var imageData []byte
	var imageActive bool
	err = db.Database.QueryRow("SELECT data, active FROM images WHERE id = ? AND post = ? AND wallet = ?", id, post, wallet).Scan(&imageData, &imageActive)
	if err != nil {
		if err == sql.ErrNoRows {
			imageData, err = distributed.NeedById(fullId)
			if err != nil {
				http.Error(w, "Image not found", http.StatusNotFound)
				return
			}
		} else {
			http.Error(w, "Failed to fetch image", http.StatusInternalServerError)
			return
		}
	} else if !imageActive {
		http.Error(w, "Disabled image", http.StatusForbidden)
		return
	}

	w.Write(imageData)
}

func AddFeedback(w http.ResponseWriter, r *http.Request) {
	storedUser := r.Context().Value(CONTEXT_USER_OBJECT_KEY).(db.UserKey)

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var newFeedback FeedbackBody
	if err := json.NewDecoder(r.Body).Decode(&newFeedback); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	err := db.AddFeedback(db.Feedback{
		Type:    newFeedback.Type,
		Wallet:  storedUser.WalletID,
		Target:  newFeedback.Target,
		Content: newFeedback.Content,
		Done:    false,
	})

	if err != nil {
		http.Error(w, "Failed to add Feedback", http.StatusInternalServerError)
		return
	}
}

type UserInfo struct {
	WalletID   string `json:"walletId"`
	InboxCount int    `json:"inboxCount"`
	ImageWidth int    `json:"imageWidth"`
	ImageSize  int    `json:"imageSize"`
}

func GetInfo(w http.ResponseWriter, r *http.Request) {
	storedUser := r.Context().Value(CONTEXT_USER_OBJECT_KEY).(db.UserKey)

	inboxCount, err := db.GetInboxCount(storedUser.WalletID)
	if err != nil {
		http.Error(w, "Failed to get inbox count", http.StatusInternalServerError)
		return
	}

	info := UserInfo{
		WalletID:   storedUser.WalletID,
		ImageWidth: 1000,
		ImageSize:  200,
		InboxCount: inboxCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func GetMessages(w http.ResponseWriter, r *http.Request) {
	storedUser := r.Context().Value(CONTEXT_USER_OBJECT_KEY).(db.UserKey)

	messages, err := db.GetInboxMessages(storedUser.WalletID)
	if err != nil {
		http.Error(w, "Failed to get messages", http.StatusInternalServerError)
		return
	}

	response := GetMessagesResponse{
		Messages: messages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func SendMessage(w http.ResponseWriter, r *http.Request) {
	storedUser := r.Context().Value(CONTEXT_USER_OBJECT_KEY).(db.UserKey)

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Check if recipient exists in local keys table
	_, err := db.GetUserKey(req.Recipient)
	if err == nil {
		// Recipient exists locally, add to their inbox
		_, err = db.AddInboxMessage(req.Recipient, storedUser.WalletID, req.Message)
		if err != nil {
			http.Error(w, "Failed to send message", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Recipient not found locally, publish to distributed network
	received, err := distributed.PublishInboxMessage(req.Recipient, storedUser.WalletID, req.Message)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send message to distributed network: %v", err), http.StatusInternalServerError)
		return
	}

	if !received {
		http.Error(w, "Message was not delivered to any recipient", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func MessageSaved(w http.ResponseWriter, r *http.Request) {
	storedUser := r.Context().Value(CONTEXT_USER_OBJECT_KEY).(db.UserKey)

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req MessageSavedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if len(req.MessageIds) == 0 {
		http.Error(w, "No message IDs provided", http.StatusBadRequest)
		return
	}

	err := db.RemoveInboxMessage(storedUser.WalletID, req.MessageIds)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to remove messages: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
