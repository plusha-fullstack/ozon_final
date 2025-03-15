// internal/pkg/repository/postgresql/user_repo.go
package postgresql

import (
	"context"
	"errors"
	"fmt"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/db"
	"gitlab.ozon.dev/pupkingeorgij/homework/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

type UserRepo struct {
	db db.DB
}

type User struct {
	ID       string
	Username string
	Password string
}

func NewUserRepo(db db.DB) storage.UserRepository {
	return &UserRepo{db: db}
}

func (r *UserRepo) CreateUser(ctx context.Context, username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx,
		"INSERT INTO users (username, password) VALUES ($1, $2)",
		username, string(hashedPassword))
	return err
}

func (r *UserRepo) CreateUserTx(ctx context.Context, tx db.Tx, username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		"INSERT INTO users (username, password) VALUES ($1, $2)",
		username, string(hashedPassword))
	return err
}

func (r *UserRepo) ValidateUser(ctx context.Context, username, password string) (bool, error) {
	var hashedPassword string
	err := r.db.ExecQueryRow(ctx,
		"SELECT password FROM users WHERE username = $1", username).Scan(&hashedPassword)
	if err != nil {
		return false, errors.New("user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		fmt.Println(err)
	}
	return true, nil
}
