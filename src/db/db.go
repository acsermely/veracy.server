package db

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	createUserTableSQL = `CREATE TABLE IF NOT EXISTS users (
        "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "username" TEXT,
        "password" TEXT
    );`

	createImagesTableSQL = `CREATE TABLE IF NOT EXISTS images (
	id INTEGER PRIMARY KEY,
	wallet TEXT,
	post TEXT,
	data BLOB
	);`

	createKeysTableSQL = `CREATE TABLE IF NOT EXISTS keys (
        "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        "wallet" TEXT,
        "key" TEXT,
		"chal" TEXT
    );`

)

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserKey struct {
	ID       int    `json:"id"`
	WalletID string `json:"wallet"`
	Key      string `json:"key"`
	Chal     string `json:"chal"`
}

var Database *sql.DB

func Create() (*sql.DB, error) {
	database, err := sql.Open("sqlite3", "./users.db")
	if err != nil {
		return nil, err
	}
	Database = database

	_, err = database.Exec(createUserTableSQL)
	if err != nil {
		return nil, err
	}

	_, err = database.Exec(createImagesTableSQL)
	if err != nil {
		return nil, err
	}

	_, err = database.Exec(createKeysTableSQL)
	if err != nil {
		return nil, err
	}

	return database, nil
}

func GetUser(name string) (User, error) {
	selectUserQuery := "SELECT id, username, password FROM users WHERE username = ?"

	var storedUser User
	row := Database.QueryRow(selectUserQuery, name)
	err := row.Scan(&storedUser.ID, &storedUser.Username, &storedUser.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, fmt.Errorf("invalid username or password")
		}
		return User{}, fmt.Errorf("database error")
	}

	return storedUser, nil
}

func GetUserKey(wallet string) (UserKey, error) {
	selectUserQuery := "SELECT id, wallet, key, chal FROM keys WHERE wallet = ?"

	var storedUser UserKey
	row := Database.QueryRow(selectUserQuery, wallet)
	err := row.Scan(&storedUser.ID, &storedUser.WalletID, &storedUser.Key, &storedUser.Chal)
	if err != nil {
		if err == sql.ErrNoRows {
			return UserKey{}, fmt.Errorf("invalid Wallet ID")
		}
		fmt.Println(err)
		return UserKey{}, fmt.Errorf("database error")
	}

	return storedUser, nil
}

func InsertUserKey(wallet string, key string) (string, error) {
	insertUserSQL := `INSERT INTO keys (wallet, key, chal) VALUES (?, ?, ?)`

	stmt, err := Database.Prepare(insertUserSQL)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	newChal := getNewChal()

	_, err = stmt.Exec(wallet, key, newChal)
	if err != nil {
		return "", err
	}

	return newChal, nil
}

func getNewChal() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomNumber := r.Intn(1000000)
	return strconv.Itoa(randomNumber)
}

func SetNewChal(wallet string) (string, error) {
	newChal := getNewChal()

	query := `UPDATE keys SET chal = ? WHERE wallet = ?`
	_, err := Database.Exec(query, newChal, wallet)
	if err != nil {
		return "", err
	}
	return newChal, nil
}

func DeleteChal(wallet string) error {
	query := `UPDATE keys SET chal = ? WHERE wallet = ?`
	_, err := Database.Exec(query, "", wallet)
	if err != nil {
		return err
	}
	return nil
}
