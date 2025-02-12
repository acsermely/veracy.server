package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/acsermely/veracy.server/src/db"
	"github.com/golang-jwt/jwt/v4"
)

func GetAdminChal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))

	adminKeyString := os.Getenv("ADMIN_KEY")

	rsaPublicKey, err := parsePublicKeyString(adminKeyString)
	if err != nil {
		http.Error(w, "Cannot parse Key", http.StatusBadRequest)
		return
	}

	challange, err := db.SetAdminChal()
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

func LoginAdminChal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))

	secret := []byte(os.Getenv("SECRET"))

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var loginCreds LoginAdminBody
	if err := json.NewDecoder(r.Body).Decode(&loginCreds); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	adminChal, err := db.GetAdminChal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if adminChal == "" || loginCreds.Chal != adminChal {
		http.Error(w, "Invalid Challange", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"authorized": true,
		"user":       "admin",
		"exp":        time.Now().Add(JWT_COOKIE_EXPIRATION).Unix(),
	})

	tokenString, err := token.SignedString(secret)

	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	_ = db.ResetAdminChal()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(tokenString))
}

func GetAllImages(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Database.Query("SELECT id, wallet, post, data, active FROM images")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer rows.Close()
	var imageDataList []ImageData
	for rows.Next() {
		var imageData ImageData
		err := rows.Scan(&imageData.Id, &imageData.Wallet, &imageData.Post, &imageData.Data, &imageData.Active)
		if err != nil {
			fmt.Println(err)
			return
		}
		imageDataList = append(imageDataList, imageData)
	}
	if err = rows.Err(); err != nil {
		fmt.Println(err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(imageDataList)
}

func SetImageActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var details SetImageActiveBody
	if err := json.NewDecoder(r.Body).Decode(&details); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	_, err := db.Database.Exec(`UPDATE images SET active = ? WHERE id = ? AND post = ? AND wallet = ?`, details.Active, details.Id, details.Post, details.Wallet)
	if err != nil {
		fmt.Println(err)
		return
	}

	var imageData SetImageActiveBody
	err = db.Database.QueryRow("SELECT id, wallet, post, active FROM images WHERE id = ? AND post = ? AND wallet = ?", details.Id, details.Post, details.Wallet).Scan(&imageData.Id, &imageData.Wallet, &imageData.Post, &imageData.Active)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to fetch image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(imageData)
}

func GetAllFeedback(w http.ResponseWriter, r *http.Request) {
	feedbacks, err := db.GetAllFeedback()
	if err != nil {
		http.Error(w, "Failed get Feedbacks", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feedbacks)
}
