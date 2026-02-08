package repo

import (
	"context"
	"database/sql"
	"time"
)

// Repository — интерфейс для работы с БД
type Repository interface {
	CreateUser(ctx context.Context, login, email, password string) (int, error)
	GetBylogin(ctx context.Context, login string) (int, string, error)
	GetProfileByLogin(ctx context.Context, login string) (*Profile, error)
	UpdateAvatar(ctx context.Context, userID int, avatarURL string) error
	UpdateProfile(ctx context.Context, userID int, login string, description string) (string, error)
	GetProfileByID(ctx context.Context, id int) (*Profile, error)
	SetPremiumUntil(ctx context.Context, userID int, until time.Time) error
	GetPremiumUntil(ctx context.Context, userID int) (*time.Time, error)
	ClearPremium(ctx context.Context, userID int) error
}

type Profile struct {
	Login       string `json:"login"`
	Email       string `json:"email"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
	IsPremium   bool   `json:"is_premium"`
	PremiumUntil *time.Time `json:"premium_until"`
}

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserDB(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) UpdateAvatar(ctx context.Context, userID int, avatarURL string) error {
	query := `UPDATE profile SET avatar_url = $1 WHERE user_id = $2`
	_, err := r.db.ExecContext(ctx, query, avatarURL, userID)
	return err
}

func (r *PostgresUserRepository) UpdateProfile(ctx context.Context, userID int, login string, description string) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if login != "" {
		_, err = tx.ExecContext(ctx, `UPDATE users SET login = $1 WHERE id = $2`, login, userID)
		if err != nil {
			return "", err
		}
	}

	if _, err = tx.ExecContext(ctx, `UPDATE profile SET description = $1 WHERE user_id = $2`, description, userID); err != nil {
		return "", err
	}

	var finalLogin string
	if err = tx.QueryRowContext(ctx, `SELECT login FROM users WHERE id = $1`, userID).Scan(&finalLogin); err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	return finalLogin, nil
}

func (r *PostgresUserRepository) GetProfileByID(ctx context.Context, id int) (*Profile, error) {
	p := &Profile{}
	// SQL: профиль привязан к id пользователя
	query := `
		SELECT u.login, u.email, COALESCE(p.description, ''), COALESCE(p.avatar_url, ''), p.is_premium, p.premium_until
		FROM profile p
		JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1
	`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&p.Login, &p.Email, &p.Description, &p.AvatarURL, &p.IsPremium, &p.PremiumUntil)
	return p, err
}

func (r *PostgresUserRepository) GetProfileByLogin(ctx context.Context, login string) (*Profile, error) {
	p := &Profile{}
	query := `
		SELECT u.login, u.email, COALESCE(p.description, ''), COALESCE(p.avatar_url, ''), p.is_premium, p.premium_until
		FROM profile p
		JOIN users u ON u.id = p.user_id
		WHERE u.login = $1
	`
	err := r.db.QueryRowContext(ctx, query, login).Scan(&p.Login, &p.Email, &p.Description, &p.AvatarURL, &p.IsPremium, &p.PremiumUntil)
	return p, err
}

func (r *PostgresUserRepository) GetBylogin(ctx context.Context, login string) (int, string, error) {
	var id int
	var hash string
	query := "SELECT id, password FROM users WHERE login=$1"
	err := r.db.QueryRowContext(ctx, query, login).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		return 0, "", nil
	}
	return id, hash, err
}

func (r *PostgresUserRepository) CreateUser(
	ctx context.Context,
	login, email, password string,
) (int, error) {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var userID int

	err = tx.QueryRowContext(ctx, `
        INSERT INTO users (login, email, password)
        VALUES ($1, $2, $3)
        RETURNING id
    `, login, email, password).Scan(&userID)
	if err != nil {
		return 0, err
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO profile (user_id)
        VALUES ($1)
    `, userID)
	if err != nil {
		return 0, err
	}

	return userID, tx.Commit()
}

func (r *PostgresUserRepository) SetPremiumUntil(ctx context.Context, userID int, until time.Time) error {
	query := `UPDATE profile SET is_premium = true, premium_until = $1 WHERE user_id = $2`
	_, err := r.db.ExecContext(ctx, query, until, userID)
	return err
}

func (r *PostgresUserRepository) GetPremiumUntil(ctx context.Context, userID int) (*time.Time, error) {
	var t sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT premium_until FROM profile WHERE user_id = $1`, userID).Scan(&t)
	if err != nil {
		return nil, err
	}
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}

func (r *PostgresUserRepository) ClearPremium(ctx context.Context, userID int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE profile SET is_premium = false, premium_until = NULL WHERE user_id = $1`, userID)
	return err
}
