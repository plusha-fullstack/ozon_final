package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

func loadEnv() {
	exePath, err := os.Getwd()
	if err != nil {
		log.Fatal("Error getting working directory:", err)
	}

	possiblePaths := []string{
		filepath.Join(exePath, ".env"),
		filepath.Join(exePath, "..", ".env"),
		filepath.Join(exePath, "..", "..", ".env"),
	}

	for _, envPath := range possiblePaths {
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("Loaded environment variables from %s", envPath)
			return
		}
	}

	for _, envPath := range possiblePaths {
		examplePath := filepath.Join(filepath.Dir(envPath), ".example.env")
		if err := godotenv.Load(examplePath); err == nil {
			log.Printf("Loaded environment variables from %s", examplePath)
			return
		}
	}

	log.Fatal("No .env or .env.example file found in expected locations")
}

func init() {
	loadEnv()
}

func NewDb(ctx context.Context) (*Database, error) {
	pool, err := pgxpool.Connect(ctx, generateDsn())
	if err != nil {
		return nil, err
	}
	return NewDatabase(pool), nil
}

func generateDsn() string {
	host := os.Getenv("DB_HOST")
	port, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")

	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
}
