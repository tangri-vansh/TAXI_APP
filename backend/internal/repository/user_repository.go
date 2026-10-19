package repository

import (
	"context"
	"database/sql"

	"safe-ride/internal/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) CreateUser(
	ctx context.Context,
	user *model.User,
) error {

	query := `
		INSERT INTO users (name, phone, password_hash)
		VALUES ($1, $2, $3)
		RETURNING user_id
	`

	return r.db.QueryRowContext(
		ctx,
		query,
		user.Name,
		user.Phone,
		user.PasswordHash,
	).Scan(&user.UserID)
}