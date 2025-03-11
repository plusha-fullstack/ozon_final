// internal/pkg/repository/postgresql/user_repo.go
package postgresql

import (
	"context"
	"errors"
	"fmt"
	"log"

	"gitlab.ozon.dev/pupkingeorgij/homework/internal/pkg/db"
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

func NewUserRepo(database db.DB) *UserRepo {
	return &UserRepo{db: database}
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

func (r *UserRepo) ValidateUser(ctx context.Context, username, password string) (bool, error) {
	fmt.Println("args 1-usename,2-pass", username, password)
	var hashedPassword string
	err := r.db.ExecQueryRow(ctx,
		"SELECT password FROM users WHERE username = $1", username).Scan(&hashedPassword)
	if err != nil {
		return false, errors.New("user not found")
	}

	// Compare the stored hash with the provided password
	fmt.Println("hashedPas", hashedPassword)
	fmt.Println("password", password)

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		log.Fatal(err)
	}
	return true, nil
}
