package db

import (
	"context"
	"fmt"
	"log"
	"os"
)

func InitAdmin(database *Database) {
	loadEnv()
	adminUsername := os.Getenv("ADMIN_USERNAME")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	var count int
	err := database.ExecQueryRow(context.Background(), "SELECT COUNT(*) FROM users WHERE username = $1", adminUsername).Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	if count == 0 {
		_, err = database.Exec(context.Background(), "INSERT INTO users (username, password) VALUES ($1, $2)", adminUsername, adminPassword)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Admin user created successfully.")
	} else {
		fmt.Println("Admin user already exists.")
	}
}
