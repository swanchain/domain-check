package sslcert

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

type TeamsMessage struct {
	Type     string `json:"@type"`
	Context  string `json:"@context"`
	Summary  string `json:"summary"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Markdown bool   `json:"markdown"`
}

type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(a.username), nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("unknown from server")
		}
	}
	return nil, nil
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

func FormatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	min := int(d.Minutes())
	h := min / 60
	min %= 60
	days := h / 24
	h %= 24

	return fmt.Sprintf("%d days %d hours %d minutes", days, h, min)
}
func SendEmail(emailConfig EmailConfig, recipient string, message string) {
	from := emailConfig.User
	pass := emailConfig.Pass
	to := recipient

	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: SSL Certificate Expiration Warning\n\n" +
		message

	tlsconfig := &tls.Config{
		ServerName: "smtp.office365.com",
	}
	conn, err := net.Dial("tcp", "smtp.office365.com:587")
	if err != nil {
		log.Panic(err)
	}

	c, err := smtp.NewClient(conn, "smtp.office365.com")
	if err != nil {
		log.Panic(err)
	}

	if err = c.StartTLS(tlsconfig); err != nil {
		log.Panic(err)
	}

	auth := LoginAuth(from, pass)
	if err = c.Auth(auth); err != nil {
		log.Panic(err)
	}

	if err = c.Mail(from); err != nil {
		log.Panic(err)
	}
	if err = c.Rcpt(to); err != nil {
		log.Panic(err)
	}

	wc, err := c.Data()
	if err != nil {
		log.Panic(err)
	}
	_, err = wc.Write([]byte(msg))
	if err != nil {
		log.Panic(err)
	}
	err = wc.Close()
	if err != nil {
		log.Panic(err)
	}

	err = c.Quit()
	if err != nil {
		log.Panic(err)
	}

	log.Print("sent email to ", to)
}

func SendTeamsNotification(webhookURL string, message string, isMarkdown bool) {
	msg := TeamsMessage{
		Type:     "MessageCard",
		Context:  "http://schema.org/extensions",
		Summary:  "SSL Certificate Expiration Warning",
		Title:    "SSL Certificate Expiration Warning",
		Text:     message,
		Markdown: isMarkdown,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("json marshal error: %s", err)
		return
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(msgBytes))
	if err != nil {
		log.Printf("http post error: %s", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("teams webhook error: %s", bodyBytes)
	}
}
