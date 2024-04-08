package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/robfig/cron"
	"github.com/swanchain/domain-check/pkg/chainstatus"
	"github.com/swanchain/domain-check/pkg/database"
	"github.com/swanchain/domain-check/pkg/model"
	"github.com/swanchain/domain-check/pkg/sslcert"
	"github.com/swanchain/domain-check/pkg/wallet"
)

type Info struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

/*
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
*/
func getRecipients(db *sqlx.DB) ([]model.Info, error) {
	var recipients []model.Info
	err := db.Select(&recipients, "SELECT key, value FROM info WHERE type = 'email'")
	if err != nil {
		return nil, err
	}
	return recipients, nil
}

/*
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
*/
func getTeamsWebhookURL(db *sqlx.DB) (string, error) {
	var teamsWebhookURL string
	err := db.Get(&teamsWebhookURL, "SELECT value FROM info WHERE key = 'teams-webhook'")
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
	err = wallet.SetExplorerAndRpcVars(db)
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

		emailConfig := wallet.EmailConfig{
			User: os.Getenv("ADMIN_EMAIL"),
			Pass: os.Getenv("ADMIN_PW"),
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

			balanceChange, err := wallet.GetWalletBalanceChange(db, l1Wallet.Value)
			if err != nil {
				log.Println(err)
				continue
			}
			message := fmt.Sprintf("Wallet balance for %s is %f.\nBalance change: %f\n", l1Wallet.Value, balance, balanceChange)
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

			balanceChange, err := wallet.GetWalletBalanceChange(db, l2Wallet.Value)
			if err != nil {
				log.Println(err)
				continue
			}

			message := fmt.Sprintf("The balance for wallet %s is now %f.\nBalance change: %f\n", l2Wallet.Value, balance, balanceChange)
			messages = append(messages, message)
		}

		teamsMessage := strings.Join(messages, "\n")
		err = wallet.SendTeamsNotification(teamsWebhookURL, teamsMessage, true)
		if err != nil {
			log.Printf("Error sending Teams notification: %v", err)
		} else {
			log.Println("Teams notification sent.")
		}

		emailBody := strings.Join(messages, "\n")
		for _, recipient := range recipients {
			err := wallet.SendWalletBalanceEmail(emailConfig, recipient.Value, emailBody)
			if err != nil {
				log.Printf("Error sending email to %s: %v", recipient.Value, err)
				continue
			}
			log.Printf("Sent email to %s", recipient.Value)
		}

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

		emailConfig := sslcert.EmailConfig{
			User: os.Getenv("ADMIN_EMAIL"),
			Pass: os.Getenv("ADMIN_PW"),
		}

		teamsWebhookURL, err := getTeamsWebhookURL(db)
		if err != nil {
			log.Println(err)
			return
		}

		var emailMessages []string
		var teamsMessages []string
		for _, domain := range domains {
			expireDate, err := sslcert.CheckCertificate(domain.Value)
			if err != nil {
				log.Println(err)
				continue
			}

			if time.Until(expireDate) < 48*time.Hour {
				expireMessage := fmt.Sprintf("The SSL certificate for %s will expire on %s.\n", domain.Value, expireDate.String())
				log.Println(expireMessage)
				emailMessages = append(emailMessages, expireMessage)
				teamsMessages = append(teamsMessages, expireMessage)
			}
		}

		if len(teamsMessages) > 0 {
			teamsMessage := strings.Join(teamsMessages, "")
			sslcert.SendTeamsNotification(teamsWebhookURL, teamsMessage, true)
			log.Println("Teams notification sent.")
		} else {
			log.Println("No SSL certificates expiring in under 48 hours. No Teams notification sent.")
		}

		if len(emailMessages) > 0 {
			emailBody := strings.Join(emailMessages, "")
			for _, recipient := range recipients {
				sslcert.SendEmail(emailConfig, recipient.Value, emailBody)
				log.Printf("Sent email to %s", recipient.Value)
			}
		} else {
			log.Println("No SSL certificates expiring in under 48 hours. No emails sent.")
		}

		log.Println("SSL Scheduler finished")
	}

	chainStatusTask := func() {
		swan_rpc := wallet.GetSwanRPC()
		if chainstatus.CheckChainStatus(swan_rpc) {
			log.Println("Less than 5 transactions in the last 10 blocks.")
			teamsWebhookURL, err := getTeamsWebhookURL(db)
			if err != nil {
				log.Println(err)
				return
			}
			chainstatus.SendTeamsNotification(teamsWebhookURL, "There are less than 5 transactions in the last 10 blocks.", true)
		}
	}
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatal(err)
	}
	c := cron.NewWithLocation(loc)
	c.AddFunc("30 9 * * *", walletTask)
	//c.AddFunc("30 9 * * *", SSLtask)
	c.Start()

	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				chainStatusTask()
			}
		}
	}()
	select {}
}
