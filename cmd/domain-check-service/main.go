package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/robfig/cron"
	"github.com/swanchain/domian-check/pkg/database"
	"github.com/swanchain/domian-check/pkg/model"
)

type Info struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

type EmailConfig struct {
	User string
	Pass string
}

func getEmailConfig(db *sqlx.DB) (map[string]string, error) {
	rows, err := db.Queryx("SELECT key, value FROM info WHERE type = 'email' AND key IN ('admin-email', 'admin-email-psw')")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	emailConfig := make(map[string]string)
	for rows.Next() {
		var info model.Info
		err = rows.StructScan(&info)
		if err != nil {
			return nil, err
		}
		emailConfig[info.Key] = info.Value
	}

	return emailConfig, nil
}

func getDomains(db *sqlx.DB) ([]model.Info, error) {
	var domains []model.Info
	err := db.Select(&domains, "SELECT key, value FROM info WHERE is_active = true AND type = 'domain'")
	if err != nil {
		return nil, err
	}
	return domains, nil
}

func getRecipients(db *sqlx.DB) ([]model.Info, error) {
	var recipients []model.Info
	err := db.Select(&recipients, "SELECT key, value FROM info WHERE type = 'email'")
	if err != nil {
		return nil, err
	}
	return recipients, nil
}

func checkCertificate(domain string) (time.Time, error) {
	conn, err := tls.Dial("tcp", domain+":443", nil)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()

	cert := conn.ConnectionState().PeerCertificates[0]
	return cert.NotAfter, nil
}

func sendEmail(emailConfig EmailConfig, recipient string, domain string, expireDate time.Time) {
	from := emailConfig.User
	pass := emailConfig.Pass
	to := recipient

	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: SSL Certificate Expiration Warning\n\n" +
		fmt.Sprintf("The SSL certificate for %s will expire on %s.", domain, expireDate.String())

	err := smtp.SendMail("smtp.office365.com:587",
		smtp.PlainAuth("", from, pass, "smtp.office365.com"),
		from, []string{to}, []byte(msg))

	if err != nil {
		log.Printf("smtp error: %s", err)
		return
	}

	log.Print("sent email to ", to)
}

func decryptPassword(encryptedPassword string) (string, error) {
	key := []byte(os.Getenv("DECRYPT_KEY"))
	ciphertext, _ := base64.URLEncoding.DecodeString(encryptedPassword)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", err
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func main() {
	db, err := database.ConnectToDB()
	if err != nil {
		log.Fatalln(err)
	}

	c := cron.New()
	c.AddFunc("@every 1h", func() {
		log.Println("Scheduler started")

		domains, err := getDomains(db)
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("Got %d domains", len(domains))

		recipients, err := getRecipients(db)
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("Got %d recipients", len(recipients))

		emailConfigMap, err := getEmailConfig(db)
		if err != nil {
			log.Println(err)
			return
		}

		emailConfig := EmailConfig{
			User: emailConfigMap["admin-email"],
			Pass: emailConfigMap["admin-email-psw"],
		}

		for _, domain := range domains {
			expireDate, err := checkCertificate(domain.Value)
			if err != nil {
				log.Println(err)
				continue
			}

			log.Printf("Domain %s expires in %s", domain.Value, time.Until(expireDate).String())

			if time.Until(expireDate) < 24*time.Hour {
				decryptedPassword, err := decryptPassword(emailConfig.Pass)
				if err != nil {
					log.Println(err)
					continue
				}

				emailConfig.Pass = decryptedPassword

				for _, recipient := range recipients {
					sendEmail(emailConfig, recipient.Value, domain.Value, expireDate)
					log.Printf("Sent email to %s about domain %s", recipient.Value, domain.Value)
				}
			}
		}
	})
	c.Start()

	select {}
}
