package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	*sql.DB
}

type User struct {
	ID         int
	Username   string
	Role       string
	SecretData string
	Notes      string
	CreatedAt  time.Time
}

type UserWithHash struct {
	User
	PasswordHash string
}

type Order struct {
	ID           int
	UserID       int
	Product      string
	Amount       float64
	Status       string
	PrivateNotes string
	CreatedAt    time.Time
}

func Connect(url string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("open error: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping error: %w", err)
	}
	return &DB{sqlDB}, nil
}

func (db *DB) GetUserByUsername(username string) (*UserWithHash, error) {
	u := &UserWithHash{}
	// COALESCE gör så om värdet är tex NULL hos notes so converteras det till en tom string
	err := db.QueryRow(`
		SELECT id, username,password_hash,role,
			COALESCE(secret_data,''),
			COALESCE(notes,''),			
			created_at
		FROM users
		WHERE username = $1`, username).Scan(
		&u.User.ID,
		&u.User.Username,
		&u.PasswordHash,
		&u.User.Role,
		&u.User.SecretData,
		&u.User.Notes,
		&u.User.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user error: %w", err)
	}
	return u, nil

}
func (db *DB) GetOrdersByUserID(userID int) ([]Order, error) {
	rows, err := db.Query(`SELECT id, user_id, product, amount, status, COALESCE(private_notes,''), created_at FROM orders WHERE user_id=$1`, userID) // ta alla rows i hela tabellen som matchar på user id
	if err != nil {
		return nil, fmt.Errorf("order query error: %w", err)
	}
	defer rows.Close()
	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Product, &o.Amount, &o.Status, &o.PrivateNotes, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan order error: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil { // dubbel kolla om db connectinon stängdes mitt i queryeen dp första err kollar bara om det går att öppna en connection till db
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return orders, nil

}
func (db *DB) Migrate() error {
	schema := `
    CREATE TABLE IF NOT EXISTS users (
        id            SERIAL PRIMARY KEY,
        username      TEXT UNIQUE NOT NULL,
        password_hash TEXT NOT NULL,
        role          TEXT NOT NULL DEFAULT 'user',
        secret_data   JSONB,
        notes         TEXT,
        created_at    TIMESTAMPTZ DEFAULT NOW()
    );

    CREATE TABLE IF NOT EXISTS orders (
        id            SERIAL PRIMARY KEY,
        user_id       INT REFERENCES users(id),
        product       TEXT NOT NULL,
        amount        FLOAT NOT NULL,
        status        TEXT NOT NULL DEFAULT 'pending',
        private_notes TEXT,
        created_at    TIMESTAMPTZ DEFAULT NOW()
    );`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate error: %w", err)
	}
	return nil
}
