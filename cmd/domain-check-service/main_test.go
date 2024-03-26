package main

import (
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestGetEmailConfig(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"key", "value"}).
		AddRow("admin-email", "admin@example.com").
		AddRow("admin-email-psw", "password")
	mock.ExpectQuery("SELECT key, value FROM info WHERE type = 'email' AND key IN \\('admin-email', 'admin-email-psw'\\)").WillReturnRows(rows)

	emailConfig, err := getEmailConfig(sqlxDB)
	if err != nil {
		t.Errorf("getEmailConfig() returned error: %v", err)
	}

	if emailConfig["admin-email"] != "admin@example.com" || emailConfig["admin-email-psw"] != "password" {
		t.Errorf("Unexpected emailConfig: got %v, want {'admin-email': 'admin@example.com', 'admin-email-psw': 'password'}", emailConfig)
	}
}

func TestCheckCertificate(t *testing.T) {
	expireDate, err := checkCertificate("google.com")
	if err != nil {
		t.Errorf("checkCertificate() returned error: %v", err)
	}

	if time.Until(expireDate) < 0 {
		t.Errorf("Unexpected expireDate: got %v, want a future date", expireDate)
	}
}

func TestGetDomains(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"id", "key", "value", "type", "is_active", "note"}).
		AddRow(10, "swan-official", "https://swanchain.io/", "domain", true, "")
	mock.ExpectQuery("SELECT key, value FROM info WHERE is_active = true AND type = 'domain'").WillReturnRows(rows)

	domains, err := getDomains(sqlxDB)
	if err != nil {
		t.Errorf("getDomains() returned error: %v", err)
	}

	if len(domains) != 1 || domains[0].Key != "swan-official" || domains[0].Value != "https://swanchain.io/" {
		t.Errorf("Unexpected domains: got %v, want [{Key: 'swan-official', Value: 'https://swanchain.io/'}]", domains)
	}
}

func TestGetRecipients(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"id", "key", "value", "type", "is_active", "note"}).
		AddRow(11, "pluto-email", "Pluto.Zhang@nbai.io", "email", true, "").
		AddRow(12, "admin-email", "postman.swan@outlook.com", "email", true, "")
	mock.ExpectQuery("SELECT key, value FROM info WHERE type = 'email'").WillReturnRows(rows)

	recipients, err := getRecipients(sqlxDB)
	if err != nil {
		t.Errorf("getRecipients() returned error: %v", err)
	}

	if len(recipients) != 2 || recipients[0].Key != "pluto-email" || recipients[0].Value != "Pluto.Zhang@nbai.io" || recipients[1].Key != "admin-email" || recipients[1].Value != "postman.swan@outlook.com" {
		t.Errorf("Unexpected recipients: got %v, want [{Key: 'pluto-email', Value: 'Pluto.Zhang@nbai.io'}, {Key: 'admin-email', Value: 'postman.swan@outlook.com'}]", recipients)
	}
}

func TestDecryptPassword(t *testing.T) {
	os.Setenv("DECRYPT_KEY", "mock-decrypt-key")

	encryptedPassword := "mock-encrypted-password"

	_, err := decryptPassword(encryptedPassword)

	if err == nil {
		t.Errorf("decryptPassword() should return an error for invalid encrypted password and decryption key")
	}
}
