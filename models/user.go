package models

import "time"

type User struct {
	ID        int64     `db:"user_id"`
	Username  string    `db:"username"`
	UUID      string    `db:"uuid"`
	CreatedAt time.Time `db:"created_at"`
}

type XrayUser struct {
	Email string `json:"email"`
	ID    string `json:"id"`
	Flow  string `json:"flow"`
}

type XrayConfig struct {
	Log       interface{}   `json:"log"`
	Routing   interface{}   `json:"routing"`
	Inbounds  []interface{} `json:"inbounds"`
	Outbounds []interface{} `json:"outbounds"`
	API       interface{}   `json:"api,omitempty"`
}
