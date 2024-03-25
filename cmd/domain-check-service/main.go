package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/robfig/cron"
)

type Info struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

type EmailConfig struct {
	User string
	Pass string
}

func getEmailConfig(db *sqlx.DB) (EmailConfig, error) {
	var emailConfig EmailConfig
	err := db.Get(&emailConfig, "SELECT key, value FROM info WHERE type = 'email' AND key IN ('admin-email', 'admin-email-psw')")
	if err != nil {
		return EmailConfig{}, err
	}
	return emailConfig, nil
}

func getDomains(db *sqlx.DB) ([]Info, error) {
	var domains []Info
	err := db.Select(&domains, "SELECT key, value FROM info WHERE is_active = true AND type = 'domain'")
	if err != nil {
		return nil, err
	}
	return domains, nil
}

func getRecipients(db *sqlx.DB) ([]Info, error) {
	var recipients []Info
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

func main() {
	db, err := sqlx.Connect("postgres", "user=foo dbname=bar sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}

	c := cron.New()
	c.AddFunc("@every 1h", func() {
		domains, err := getDomains(db)
		if err != nil {
			log.Println(err)
			return
		}

		recipients, err := getRecipients(db)
		if err != nil {
			log.Println(err)
			return
		}

		emailConfig, err := getEmailConfig(db)
		if err != nil {
			log.Println(err)
			return
		}

		for _, domain := range domains {
			expireDate, err := checkCertificate(domain.Value)
			if err != nil {
				log.Println(err)
				continue
			}

			if time.Until(expireDate) < 24*time.Hour {
				for _, recipient := range recipients {
					sendEmail(emailConfig, recipient.Value, domain.Value, expireDate)
				}
			}
		}
	})
	c.Start()

	select {} // Keep the program running
}
