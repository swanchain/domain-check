package database

import (
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func ConnectToDB() (*sqlx.DB, error) {
	host := os.Getenv("INFO_DB_HOST")
	port := os.Getenv("INFO_DB_PORT")
	user := os.Getenv("INFO_DB_USERNAME")
	password := os.Getenv("INFO_DB_PASSWORD")
	dbname := os.Getenv("INFO_DB_NAME")

	log.Printf("INFO_DB_HOST: %s", host)
	log.Printf("INFO_DB_PORT: %s", port)
	log.Printf("INFO_DB_USERNAME: %s", user)
	log.Printf("INFO_DB_PASSWORD: %s", password)
	log.Printf("INFO_DB_NAME: %s", dbname)

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return nil, err
	}

	_, err = db.Exec("SET search_path TO swan_ssl_certificate")
	if err != nil {
		log.Printf("Failed to set search_path: %v", err)
		return nil, err
	}

	log.Println("Successfully connected to the database!")
	return db, nil
}
