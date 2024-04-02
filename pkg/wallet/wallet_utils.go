package wallet

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"net/http"
	"net/smtp"
	"strconv"
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
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  string `json:"result"`
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

func SetExplorerAndRpcVars(db *sqlx.DB) error {
	rows, err := db.Queryx("SELECT key, value FROM info WHERE key IN ('sepolia-rpc', 'saturn-rpc', 'saturn-block-explorer', 'sepolia-block-explorer')")
	if err != nil {
		log.Printf("Error executing query in SetExplorerAndRpcVars: %s", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var config model.Config
		if err := rows.StructScan(&config); err != nil {
			log.Printf("Error scanning row in SetExplorerAndRpcVars: %s", err)
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
		log.Printf("Error after iterating rows in SetExplorerAndRpcVars: %s", err)
		return err
	}

	return nil
}
func GetL1Wallet(db *sqlx.DB) ([]model.Config, error) {
	var wallets []model.Config
	err := db.Select(&wallets, "SELECT * FROM info WHERE key ILIKE 'l1%' AND type = 'wallet-address'")
	if err != nil {
		log.Printf("Error retrieving L1 wallets: %s", err)
		return nil, err
	}
	log.Printf("Number of L1 wallets retrieved: %d", len(wallets))
	return wallets, nil
}

func GetL2Wallet(db *sqlx.DB) ([]model.Config, error) {
	var wallets []model.Config
	err := db.Select(&wallets, "SELECT * FROM info WHERE key ILIKE 'l2%' AND type = 'wallet-address'")
	if err != nil {
		log.Printf("Error retrieving L2 wallets: %s", err)
		return nil, err
	}
	log.Printf("Number of L2 wallets retrieved: %d", len(wallets))
	return wallets, nil
}

func CheckBalance(rpcURL, walletAddress string) (float64, error) {
	reqBody := &rpcRequest{
		Jsonrpc: "2.0",
		Method:  "eth_getBalance",
		Params:  []interface{}{walletAddress, "latest"},
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

	// Convert the balance from Wei (the smallest unit of Ether) to Ether
	balanceInWei := new(big.Int)
	balanceInWei.SetString(rpcResp.Result[2:], 16) // The balance is returned as a hexadecimal string
	balanceInEth := new(big.Float).Quo(new(big.Float).SetInt(balanceInWei), big.NewFloat(math.Pow10(18)))

	// Convert the balance to a float64
	balance, _ := balanceInEth.Float64()

	return balance, nil
}

func GetWalletBalanceChange(db *sqlx.DB, walletAddress string) (float64, error) {
	var balanceChange float64
	err := db.Get(&balanceChange, "SELECT balance_change FROM wallets WHERE address = $1", walletAddress)
	if err != nil {
		return 0, err
	}
	return balanceChange, nil
}

func CheckSepoliaBalance(walletAddress string) (float64, error) {
	balance, err := CheckBalance(sepolia_rpc, walletAddress)
	if err != nil {
		log.Printf("Error checking Sepolia balance for wallet %s: %s", walletAddress, err)
		return 0, err
	}
	log.Printf("Sepolia balance for wallet %s: %f", walletAddress, balance)
	return balance, nil
}

func CheckSwanBalance(walletAddress string) (float64, error) {
	balance, err := CheckBalance(swan_rpc, walletAddress)
	if err != nil {
		log.Printf("Error checking Swan balance for wallet %s: %s", walletAddress, err)
		return 0, err
	}
	log.Printf("Swan balance for wallet %s: %f", walletAddress, balance)
	return balance, nil
}

func UpsertWallet(db *sqlx.DB, wallet model.Config, newBalance float64, networkEnv string) error {
	var currentBalanceStr string
	err := db.Get(&currentBalanceStr, "SELECT balance FROM swan_chain_data WHERE wallet_address = $1 AND network_env = $2", wallet.Value, networkEnv)

	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error retrieving wallet data: %s", err)
		return err
	}

	currentBalance, _ := strconv.ParseFloat(currentBalanceStr, 64)
	balanceChange := newBalance - currentBalance

	updateQuery := `
		UPDATE swan_chain_data
		SET balance = $1, balance_change = $2, update_at = $3
		WHERE wallet_address = $4 AND network_env = $5
	`
	result, err := db.Exec(updateQuery, fmt.Sprintf("%f", newBalance), fmt.Sprintf("%f", balanceChange), time.Now().Format(time.RFC3339), wallet.Value, networkEnv)

	if err != nil {
		log.Printf("Error updating wallet data: %s", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %s", err)
		return err
	}

	if rowsAffected == 0 {
		insertQuery := `
			INSERT INTO swan_chain_data (wallet_address, balance, balance_change, network_env, update_at)
			VALUES ($1, $2, $3, $4, $5)
		`
		_, err = db.Exec(insertQuery, wallet.Value, fmt.Sprintf("%f", newBalance), fmt.Sprintf("%f", balanceChange), networkEnv, time.Now().Format(time.RFC3339))
		if err != nil {
			log.Printf("Error inserting wallet data: %s", err)
			return err
		}
	}

	return nil
}

func UpdateL1Wallet(db *sqlx.DB, wallet model.Config, newBalance float64) error {
	return UpsertWallet(db, wallet, newBalance, "sepolia")
}

func UpdateL2Wallet(db *sqlx.DB, wallet model.Config, newBalance float64) error {
	return UpsertWallet(db, wallet, newBalance, "swan")
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
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("teams webhook error: %s", bodyBytes)
	}
}
