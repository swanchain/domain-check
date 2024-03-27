package wallet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/swanchain/domain-check/pkg/model"
)

var (
	sepolia_rpc            string
	swan_rpc               string
	swan_block_explorer    string
	sepolia_block_explorer string
)

type rpcRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponse struct {
	Jsonrpc string  `json:"jsonrpc"`
	ID      int     `json:"id"`
	Result  float64 `json:"result"`
}

type EmailConfig struct {
	User string
	Pass string
}

type TeamsMessage struct {
	Type    string `json:"@type"`
	Context string `json:"@context"`
	Summary string `json:"summary"`
	Title   string `json:"title"`
	Text    string `json:"text"`
}

func SetExplorerAndRpcVars(db *sqlx.DB) error {
	rows, err := db.Queryx("SELECT key, value FROM info WHERE key IN ('sepolia-rpc', 'saturn-rpc', 'saturn-block-explorer', 'sepolia-block-explorer')")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var config model.Config
		if err := rows.StructScan(&config); err != nil {
			return err
		}

		switch config.Key {
		case "sepolia-rpc":
			sepolia_rpc = config.Value
		case "saturn-rpc":
			swan_rpc = config.Value
		case "saturn-block-explorer":
			swan_block_explorer = config.Value
		case "sepolia-block-explorer":
			sepolia_block_explorer = config.Value
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

func GetL1Wallet(db *sqlx.DB) ([]model.Config, error) {
	var wallets []model.Config
	err := db.Select(&wallets, "SELECT * FROM info WHERE key LIKE 'l1%' AND type = 'wallet-address'")
	if err != nil {
		return nil, err
	}
	return wallets, nil
}

func GetL2Wallet(db *sqlx.DB) ([]model.Config, error) {
	var wallets []model.Config
	err := db.Select(&wallets, "SELECT * FROM info WHERE key LIKE 'l2%' AND type = 'wallet-address'")
	if err != nil {
		return nil, err
	}
	return wallets, nil
}

func CheckBalance(rpcURL, walletAddress string) (float64, error) {
	reqBody := &rpcRequest{
		Jsonrpc: "2.0",
		Method:  "getbalance",
		Params:  []interface{}{walletAddress},
		ID:      1,
	}
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return 0, err
	}

	resp, err := http.Post(rpcURL, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return 0, err
	}

	return rpcResp.Result, nil
}

func CheckSepoliaBalance(walletAddress string) (float64, error) {
	return CheckBalance(sepolia_rpc, walletAddress)
}

func CheckSwanBalance(walletAddress string) (float64, error) {
	return CheckBalance(swan_rpc, walletAddress)
}

func UpdateL1Wallet(db *sqlx.DB, wallet model.Config, newBalance float64) error {
	var currentBalance float64
	err := db.Get(&currentBalance, "SELECT balance FROM swan_chain_data WHERE wallet_address = ?", wallet.Value)
	if err != nil {
		return err
	}

	balanceChange := newBalance - currentBalance

	query := `
		UPDATE swan_chain_data
		SET balance = ?, balance_change = ?, explorer = ?, network_env = ?, updated_at = ?
		WHERE wallet_address = ?
	`
	_, err = db.Exec(query, fmt.Sprintf("%f", newBalance), fmt.Sprintf("%f", balanceChange), sepolia_block_explorer, "sepolia", time.Now().Format(time.RFC3339), wallet.Value)
	return err
}

func UpdateL2Wallet(db *sqlx.DB, wallet model.Config, newBalance float64) error {
	var currentBalance float64
	err := db.Get(&currentBalance, "SELECT balance FROM swan_chain_data WHERE wallet_address = ?", wallet.Value)
	if err != nil {
		return err
	}

	balanceChange := newBalance - currentBalance

	query := `
		UPDATE swan_chain_data
		SET balance = ?, balance_change = ?, explorer = ?, network_env = ?, updated_at = ?
		WHERE wallet_address = ?
	`
	_, err = db.Exec(query, fmt.Sprintf("%f", newBalance), fmt.Sprintf("%f", balanceChange), swan_block_explorer, "swan", time.Now().Format(time.RFC3339), wallet.Value)
	return err
}

func SendWalletBalanceEmail(emailConfig EmailConfig, recipient string, message string) {
	from := emailConfig.User
	pass := emailConfig.Pass
	to := recipient

	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: Wallet Balance Update\n\n" +
		message

	err := smtp.SendMail("smtp.office365.com:587",
		smtp.PlainAuth("", from, pass, "smtp.office365.com"),
		from, []string{to}, []byte(msg))

	if err != nil {
		log.Printf("smtp error: %s", err)
		return
	}

	log.Print("sent email to ", to)
}

func SendTeamsNotification(webhookURL string, message string) {
	msg := TeamsMessage{
		Type:    "MessageCard",
		Context: "http://schema.org/extensions",
		Summary: "Wallet Balance Update",
		Title:   "Wallet Balance Update",
		Text:    message,
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
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("teams webhook error: %s", bodyBytes)
	}
}
