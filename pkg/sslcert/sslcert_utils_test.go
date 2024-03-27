package sslcert

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestCheckCertificate(t *testing.T) {
	expireDate, err := CheckCertificate("google.com")
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

	domains, err := GetDomains(sqlxDB)
	if err != nil {
		t.Errorf("getDomains() returned error: %v", err)
	}

	if len(domains) != 1 || domains[0].Key != "swan-official" || domains[0].Value != "https://swanchain.io/" {
		t.Errorf("Unexpected domains: got %v, want [{Key: 'swan-official', Value: 'https://swanchain.io/'}]", domains)
	}
}

func TestFormatDuration(t *testing.T) {
	d := time.Hour*24*5 + time.Hour*3 + time.Minute*30
	expected := "5 days 3 hours 30 minutes"
	got := FormatDuration(d)
	if got != expected {
		t.Errorf("FormatDuration() = %v, want %v", got, expected)
	}
}

type EmailSender interface {
	SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

type RealEmailSender struct{}

func (s RealEmailSender) SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	return smtp.SendMail(addr, a, from, to, msg)
}

type MockEmailSender struct {
	Addr string
	Auth smtp.Auth
	From string
	To   []string
	Msg  []byte
}

func (s *MockEmailSender) SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	s.Addr = addr
	s.Auth = a
	s.From = from
	s.To = to
	s.Msg = msg
	return nil
}

func TestableSendEmail(s EmailSender, emailConfig EmailConfig, recipient string, message string) {
	from := emailConfig.User
	pass := emailConfig.Pass
	to := recipient

	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: SSL Certificate Expiration Warning\n\n" +
		message

	err := s.SendMail("smtp.office365.com:587",
		smtp.PlainAuth("", from, pass, "smtp.office365.com"),
		from, []string{to}, []byte(msg))

	if err != nil {
		log.Printf("smtp error: %s", err)
		return
	}

	log.Print("sent email to ", to)
}

func TestSendEmail(t *testing.T) {
	emailConfig := EmailConfig{
		User: "test@example.com",
		Pass: "password",
	}
	recipient := "recipient@example.com"
	message := "Test message"

	sender := &MockEmailSender{}
	TestableSendEmail(sender, emailConfig, recipient, message)

	if sender.From != emailConfig.User || sender.To[0] != recipient || !bytes.Contains(sender.Msg, []byte(message)) {
		t.Errorf("SendEmail did not call SendMail with the correct arguments")
	}
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type RealHTTPDoer struct{}

func (d RealHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

type MockHTTPDoer struct {
	Req *http.Request
}

func (d *MockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	d.Req = req
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString("OK"))}, nil
}

func TestableSendTeamsNotification(d HTTPDoer, webhookURL string, message string) {
	msg := TeamsMessage{
		Type:    "MessageCard",
		Context: "http://schema.org/extensions",
		Summary: "SSL Certificate Expiration Warning",
		Title:   "SSL Certificate Expiration Warning",
		Text:    message,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("json marshal error: %s", err)
		return
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(msgBytes))
	if err != nil {
		log.Printf("http request creation error: %s", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.Do(req)
	if err != nil {
		log.Printf("http post error: %s", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("teams webhook error: %s", bodyBytes)
	}
}

func TestSendTeamsNotification(t *testing.T) {
	webhookURL := "http://example.com/webhook"
	message := "Test message"

	doer := &MockHTTPDoer{}
	TestableSendTeamsNotification(doer, webhookURL, message)

	if doer.Req == nil || doer.Req.URL.String() != webhookURL || doer.Req.Method != "POST" || doer.Req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("SendTeamsNotification did not call Do with the correct request")
	}
}
