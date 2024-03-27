package sslcert

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"net/url"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/swanchain/domain-check/pkg/model"
)

type Info struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

type EmailConfig struct {
	User string
	Pass string
}

func GetDomains(db *sqlx.DB) ([]model.Info, error) {
	var domains []model.Info
	err := db.Select(&domains, "SELECT key, value FROM info WHERE is_active = true AND type = 'domain'")
	if err != nil {
		return nil, err
	}
	return domains, nil
}

func CheckCertificate(domain string) (time.Time, error) {
	u, err := url.Parse(domain)
	if err != nil {
		return time.Time{}, err
	}
	conn, err := tls.Dial("tcp", u.Hostname()+":443", nil)
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()

	cert := conn.ConnectionState().PeerCertificates[0]
	return cert.NotAfter, nil
}

func SendEmail(emailConfig EmailConfig, recipient string, domain string, expireDate time.Time) {
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

func FormatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	min := int(d.Minutes())
	h := min / 60
	min %= 60
	days := h / 24
	h %= 24

	return fmt.Sprintf("%d days %d hours %d minutes", days, h, min)
}
