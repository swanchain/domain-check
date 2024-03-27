package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/robfig/cron"
	"github.com/swanchain/domain-check/pkg/database"
	"github.com/swanchain/domain-check/pkg/model"
	"github.com/swanchain/domain-check/pkg/sslcert"
	"github.com/swanchain/domain-check/pkg/wallet"
)

type Info struct {
	Key   string `db:"key"`
	Value string `db:"value"`
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

func getRecipients(db *sqlx.DB) ([]model.Info, error) {
	var recipients []model.Info
	err := db.Select(&recipients, "SELECT key, value FROM info WHERE type = 'email'")
	if err != nil {
		return nil, err
	}
	return recipients, nil
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

func getTeamsWebhookURL(db *sqlx.DB) (string, error) {
	var teamsWebhookURL string
	err := db.Get(&teamsWebhookURL, "SELECT value FROM info WHERE key = 'teams-webhook-url'")
	if err != nil {
		return "", err
	}
	return teamsWebhookURL, nil
}

func main() {
	db, err := database.ConnectToDB()
	if err != nil {
		log.Fatalln(err)
	}
	walletTask := func() {
		log.Println("Wallet Scheduler started")
		var messages []string
		err := wallet.SetExplorerAndRpcVars(db)
		if err != nil {
			log.Println(err)
			return
		}

		l1Wallets, err := wallet.GetL1Wallet(db)
		if err != nil {
			log.Println(err)
			return
		}

		l2Wallets, err := wallet.GetL2Wallet(db)
		if err != nil {
			log.Println(err)
			return
		}

		emailConfigMap, err := getEmailConfig(db)
		if err != nil {
			log.Println(err)
			return
		}

		emailConfig := wallet.EmailConfig{
			User: emailConfigMap["admin-email"],
			Pass: emailConfigMap["admin-email-psw"],
		}

		recipients, err := getRecipients(db)
		if err != nil {
			log.Println(err)
			return
		}

		teamsWebhookURL, err := getTeamsWebhookURL(db)
		if err != nil {
			log.Println(err)
			return
		}

		for _, l1Wallet := range l1Wallets {
			balance, err := wallet.CheckSepoliaBalance(l1Wallet.Value)
			if err != nil {
				log.Println(err)
				continue
			}

			err = wallet.UpdateL1Wallet(db, l1Wallet, balance)
			if err != nil {
				log.Println(err)
				continue
			}

			message := fmt.Sprintf("The balance for wallet %s is now %f.", l1Wallet.Value, balance)
			messages = append(messages, message)
		}

		for _, l2Wallet := range l2Wallets {
			balance, err := wallet.CheckSwanBalance(l2Wallet.Value)
			if err != nil {
				log.Println(err)
				continue
			}

			err = wallet.UpdateL2Wallet(db, l2Wallet, balance)
			if err != nil {
				log.Println(err)
				continue
			}

			message := fmt.Sprintf("The balance for wallet %s is now %f.", l2Wallet.Value, balance)
			messages = append(messages, message)
		}

		emailBody := strings.Join(messages, "\n")
		decryptedPassword, err := decryptPassword(emailConfig.Pass)
		if err != nil {
			log.Println(err)
			return
		}
		emailConfig.Pass = decryptedPassword
		for _, recipient := range recipients {
			wallet.SendWalletBalanceEmail(emailConfig, recipient.Value, emailBody)
			log.Printf("Sent email to %s", recipient.Value)
		}
		wallet.SendTeamsNotification(teamsWebhookURL, emailBody)

		log.Println("Wallet Scheduler finished")
	}

	SSLtask := func() {
		log.Println("SSL Scheduler started")

		domains, err := sslcert.GetDomains(db)
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

		emailConfig := sslcert.EmailConfig{
			User: emailConfigMap["admin-email"],
			Pass: emailConfigMap["admin-email-psw"],
		}

		teamsWebhookURL, err := getTeamsWebhookURL(db)
		if err != nil {
			log.Println(err)
			return
		}

		var messages []string

		for _, domain := range domains {
			expireDate, err := sslcert.CheckCertificate(domain.Value)
			if err != nil {
				log.Println(err)
				continue
			}

			log.Printf("Domain %s expires in %s", domain.Value, sslcert.FormatDuration(time.Until(expireDate)))

			if time.Until(expireDate) < 48*time.Hour {
				message := fmt.Sprintf("The SSL certificate for %s will expire on %s.", domain.Value, expireDate.String())
				messages = append(messages, message)
			}
		}

		emailBody := strings.Join(messages, "\n")
		decryptedPassword, err := decryptPassword(emailConfig.Pass)
		if err != nil {
			log.Println(err)
			return
		}
		emailConfig.Pass = decryptedPassword
		for _, recipient := range recipients {
			sslcert.SendEmail(emailConfig, recipient.Value, emailBody)
			log.Printf("Sent email to %s", recipient.Value)
		}
		sslcert.SendTeamsNotification(teamsWebhookURL, emailBody)

		log.Println("SSL Scheduler finished")
	}

	SSLtask()
	walletTask()

	c := cron.New()
	c.AddFunc("@daily", SSLtask)
	c.AddFunc("30 9 * * *", walletTask)
	c.Start()

	select {}
}
