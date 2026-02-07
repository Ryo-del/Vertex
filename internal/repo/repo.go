package repo

import (
	"context"
	"database/sql"
)

type Repository interface {
	CreateUser(ctx context.Context, login, email, password string) (int, error)
	GetBylogin(ctx context.Context, login string) (int, string, error)
	GetProfileByLogin(ctx context.Context, login string) (*Profile, error)
	UpdateAvatar(ctx context.Context, login string, avatarURL string) error
}

type PostgresUserRepository struct {
	db *sql.DB
}
type Profile struct {
	Login       string `json:"login"`
	Email       string `json:"email"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
	IsPremium   bool   `json:"is_premium"`
}

func NewPostgresUserDB(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}
func (r *PostgresUserRepository) UpdateAvatar(ctx context.Context, login string, avatarURL string) error {
	query := `UPDATE profile SET avatar_url = $1 WHERE login = $2`
	_, err := r.db.ExecContext(ctx, query, avatarURL, login)
	return err
}
func (r *PostgresUserRepository) CreateUser(ctx context.Context, login, email, password string) (int, error) {
	// ⚡️ Атомарность: используем транзакцию, чтобы не плодить мусор в БД
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int
	userQuery := "INSERT INTO users (login, email, password) VALUES ($1, $2, $3) RETURNING id"
	if err := tx.QueryRowContext(ctx, userQuery, login, email, password).Scan(&id); err != nil {
		return 0, err
	}

	profileQuery := "INSERT INTO profile (login, email) VALUES ($1, $2)"
	if _, err := tx.ExecContext(ctx, profileQuery, login, email); err != nil {
		return 0, err
	}

	return id, tx.Commit()
}

func (r *PostgresUserRepository) GetProfileByLogin(ctx context.Context, login string) (*Profile, error) {
	p := &Profile{}
	query := `
		SELECT 
			login, 
			email, 
			COALESCE(description, ''), 
			COALESCE(avatar_url, ''), 
			is_premium 
		FROM profile 
		WHERE login = $1`
	err := r.db.QueryRowContext(ctx, query, login).Scan(
		&p.Login, &p.Email, &p.Description, &p.AvatarURL, &p.IsPremium,
	)
	if err != nil {
		return nil, err
	}
	return p, err
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
