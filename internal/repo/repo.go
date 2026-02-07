package repo

import (
	"context"
	"database/sql"
)

type Repository interface {
	CreateUser(ctx context.Context, login, email, password string) (int, error)
	GetBylogin(ctx context.Context, login string) (int, string, error)
}

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserDB(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) CreateUser(ctx context.Context, login, email, password string) (int, error) {
	var id int
	query := "INSERT INTO users (login, email, password) VALUES ($1, $2, $3) RETURNING id"
	err := r.db.QueryRowContext(ctx, query, login, email, password).Scan(&id)
	return id, err
}

func (r *PostgresUserRepository) GetBylogin(ctx context.Context, login string) (int, string, error) {
	var id int
	var hash string

	query := "SELECT id, password FROM users WHERE login=$1"

	err := r.db.QueryRowContext(ctx, query, login).Scan(&id, &hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", nil
		}
		return 0, "", err
	}
	return id, hash, nil
}
