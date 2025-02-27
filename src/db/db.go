package db

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	createImagesTableSQL = `CREATE TABLE IF NOT EXISTS images (
		id INTEGER PRIMARY KEY,
		wallet TEXT,
		post TEXT,
		data BLOB,
		active BOOLEAN
	);`

	createKeysTableSQL = `CREATE TABLE IF NOT EXISTS keys (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        wallet TEXT,
        key TEXT,
		chal TEXT
    );`

	createFeedbackTableSQL = `CREATE TABLE IF NOT EXISTS feedback (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        type TEXT,
		wallet TEXT,
		target TEXT,
		content TEXT,
		done BOOLEAN DEFAULT FALSE
    );`

	createAdminTableSQL = `CREATE TABLE IF NOT EXISTS admin (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        role TEXT,
		chal TEXT
    );`

	createInboxTableSQL = `CREATE TABLE IF NOT EXISTS inbox (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user TEXT NOT NULL,
        sender TEXT NOT NULL,
        message TEXT NOT NULL,
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
    );`

	checkImagesColumnsSQL = `PRAGMA table_info(images);`

	initAdminTableSQL = `INSERT OR REPLACE INTO admin (
		id,
		role,
		chal
	) VALUES (
	 	(SELECT id FROM admin WHERE role = 'admin'),
		'admin',
		NULL
	);`
)

type UserKey struct {
	ID       int    `json:"id"`
	WalletID string `json:"wallet"`
	Key      string `json:"key"`
	Chal     string `json:"chal"`
}

type Feedback struct {
	Type    string
	Wallet  string
	Target  string
	Content string
	Done    bool
}

type InboxMessage struct {
	User      string    `json:"user"`
	Sender    string    `json:"sender"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

var Database *sql.DB

func upgrade(database *sql.DB) (*sql.DB, error) {
	query := checkImagesColumnsSQL
	rows, err := database.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnExists := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt_value interface{}
		err = rows.Scan(&cid, &name, &ctype, &notnull, &dflt_value, &pk)
		if err != nil {
			return nil, err
		}
		if name == "active" {
			columnExists = true
			break
		}
	}
	if !columnExists {
		alterQuery := `ALTER TABLE images ADD COLUMN active BOOLEAN DEFAULT TRUE;`
		_, err = database.Exec(alterQuery)
		if err != nil {
			return nil, err
		}
	}
	return database, nil
}

func Create() (*sql.DB, error) {
	database, err := sql.Open("sqlite3", "./users.db")
	if err != nil {
		return nil, err
	}
	Database = database

	_, err = database.Exec(createImagesTableSQL)
	if err != nil {
		return nil, err
	}

	_, err = database.Exec(createKeysTableSQL)
	if err != nil {
		return nil, err
	}

	_, err = database.Exec(createAdminTableSQL)
	if err != nil {
		return nil, err
	}

	_, err = database.Exec(createFeedbackTableSQL)
	if err != nil {
		return nil, err
	}

	_, err = database.Exec(createInboxTableSQL)
	if err != nil {
		return nil, err
	}

	database, err = upgrade(database)
	if err != nil {
		return nil, err
	}

	_, err = database.Exec(initAdminTableSQL)
	if err != nil {
		return nil, err
	}

	return database, nil
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

	newChal := generateChal()

	_, err = stmt.Exec(wallet, key, newChal)
	if err != nil {
		return "", err
	}

	return newChal, nil
}

func generateChal() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomNumber := r.Intn(1000000)
	return strconv.Itoa(randomNumber)
}

func SetNewChal(wallet string) (string, error) {
	newChal := generateChal()

	query := `UPDATE keys SET chal = ? WHERE wallet = ?`
	_, err := Database.Exec(query, newChal, wallet)
	if err != nil {
		return "", err
	}
	return newChal, nil
}

func SetAdminChal() (string, error) {
	newChal := generateChal()

	query := `UPDATE admin SET chal = ? WHERE role = "admin"`
	_, err := Database.Exec(query, newChal)
	if err != nil {
		return "", err
	}
	return newChal, nil
}

func GetAdminChal() (string, error) {
	selectUserQuery := `SELECT chal FROM admin WHERE role = "admin"`

	var chal string
	row := Database.QueryRow(selectUserQuery)
	err := row.Scan(&chal)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("invalid Wallet ID")
		}
		fmt.Println(err)
		return "", fmt.Errorf("database error")
	}

	return chal, nil
}

func ResetAdminChal() error {
	query := `UPDATE admin SET chal = "" WHERE role="admin"`
	_, err := Database.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

func DeleteChal(wallet string) error {
	query := `UPDATE keys SET chal = ? WHERE wallet = ?`
	_, err := Database.Exec(query, "", wallet)
	if err != nil {
		return err
	}
	return nil
}

func AddFeedback(feedback Feedback) error {
	query := `INSERT INTO feedback (type, wallet, target, content, done) VALUES (?, ?, ?, ?, ?)`
	_, err := Database.Exec(query, feedback.Type, feedback.Wallet, feedback.Target, feedback.Content, feedback.Done)
	return err
}

func GetAllFeedback() ([]Feedback, error) {
	query := `SELECT type, wallet, target, content, done FROM feedback`
	rows, err := Database.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feedbacks []Feedback
	for rows.Next() {
		var feedback Feedback
		err := rows.Scan(&feedback.Type, &feedback.Wallet, &feedback.Target, &feedback.Content, &feedback.Done)
		if err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

func GetFeedbackByID(id int) (Feedback, error) {
	query := `SELECT type, wallet, target, content, done FROM feedback WHERE id = ?`
	row := Database.QueryRow(query, id)

	var feedback Feedback
	err := row.Scan(&feedback.Type, &feedback.Wallet, &feedback.Target, &feedback.Content, &feedback.Done)
	return feedback, err
}

func GetFeedbackByWallet(wallet string) ([]Feedback, error) {
	query := `SELECT type, wallet, target, content, done FROM feedback WHERE wallet = ?`
	rows, err := Database.Query(query, wallet)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feedbacks []Feedback
	for rows.Next() {
		var feedback Feedback
		err := rows.Scan(&feedback.Type, &feedback.Wallet, &feedback.Target, &feedback.Content, &feedback.Done)
		if err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

func UpdateFeedbackDone(id int, done bool) error {
	query := `UPDATE feedback SET done = ? WHERE id = ?`
	_, err := Database.Exec(query, done, id)
	return err
}

func AddInboxMessage(user, sender, message string) error {
	query := `INSERT INTO inbox (user, sender, message) VALUES (?, ?, ?)`
	_, err := Database.Exec(query, user, sender, message)
	if err != nil {
		return fmt.Errorf("failed to add inbox message: %w", err)
	}
	return nil
}

func RemoveInboxMessage(user string, timestamps []time.Time) error {
	if len(timestamps) == 0 {
		return fmt.Errorf("no timestamps provided")
	}

	// Create the placeholder string for the timestamps
	placeholders := make([]string, len(timestamps))
	args := make([]interface{}, len(timestamps)+1)
	args[0] = user

	for i := range timestamps {
		placeholders[i] = "?"
		args[i+1] = timestamps[i]
	}

	query := fmt.Sprintf(
		`DELETE FROM inbox WHERE user = ? AND timestamp IN (%s)`,
		strings.Join(placeholders, ","),
	)

	result, err := Database.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to remove inbox messages: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no messages found for user %s with the provided timestamps", user)
	}

	return nil
}

func GetInboxMessages(user string) ([]InboxMessage, error) {
	query := `SELECT user, sender, message, timestamp FROM inbox WHERE user = ? ORDER BY timestamp DESC`
	rows, err := Database.Query(query, user)
	if err != nil {
		return nil, fmt.Errorf("failed to query inbox messages: %w", err)
	}
	defer rows.Close()

	var messages []InboxMessage
	for rows.Next() {
		var msg InboxMessage
		err := rows.Scan(&msg.User, &msg.Sender, &msg.Message, &msg.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inbox message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating inbox rows: %w", err)
	}

	return messages, nil
}

func GetInboxCount(user string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM inbox WHERE user = ?`
	err := Database.QueryRow(query, user).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get inbox count: %w", err)
	}
	return count, nil
}
