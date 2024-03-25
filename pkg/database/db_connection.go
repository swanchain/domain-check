package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func ConnectToDB() (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("INFO_DB_HOST"),
		os.Getenv("INFO_DB_PORT"),
		os.Getenv("INFO_DB_USERNAME"),
		os.Getenv("INFO_DB_PASSWORD"),
		os.Getenv("INFO_DB_NAME"),
	)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return nil, err
	}

	log.Println("Successfully connected to the database!")
	return db, nil
}
