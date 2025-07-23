package database

import (
	"database/sql"
	"log"
	"sync"
	"xray-telegram-bot/models"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
	mu sync.Mutex
}

func New(databasePath string) (*Database, error) {
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	database := &Database{db: db}
	if err := database.createTable(); err != nil {
		return nil, err
	}

	return database, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) createTable() error {
	query := `
    CREATE TABLE IF NOT EXISTS users (
        user_id INTEGER PRIMARY KEY,
        username TEXT,
        uuid TEXT,
        created_at TIMESTAMP
    );`

	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(query)
	return err
}

func (d *Database) GetUser(userID int64) (*models.User, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var user models.User
	err := d.db.QueryRow("SELECT user_id, username, uuid, created_at FROM users WHERE user_id = ?", userID).
		Scan(&user.ID, &user.Username, &user.UUID, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (d *Database) CreateUser(user *models.User) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		"INSERT INTO users (user_id, username, uuid, created_at) VALUES (?, ?, ?, ?)",
		user.ID, user.Username, user.UUID, user.CreatedAt,
	)
	return err
}

func (d *Database) DeleteUser(userID int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM users WHERE user_id = ?", userID)
	return err
}

func (d *Database) GetAllUsers() ([]*models.User, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.db.Query("SELECT user_id, username, uuid, created_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.UUID, &user.CreatedAt); err != nil {
			log.Printf("Error scanning user: %v", err)
			continue
		}
		users = append(users, &user)
	}

	return users, nil
}
