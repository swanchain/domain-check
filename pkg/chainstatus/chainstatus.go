package chainstatus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/onrik/ethrpc"
)

type TeamsMessage struct {
	Type     string `json:"@type"`
	Context  string `json:"@context"`
	Summary  string `json:"summary"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Markdown bool   `json:"markdown"`
}

func CheckChainStatus(swan_rpc string) (string, error) {
	log.Printf("Connecting to Swan Chain node at: %s", swan_rpc)
	client := ethrpc.New(swan_rpc)

	blockNumber, err := client.EthBlockNumber()
	if err != nil {
		return "", fmt.Errorf("failed to get the latest block number: %v", err)
	}

	startBlock := blockNumber - 10
	if startBlock < 0 {
		startBlock = 0
	}

	var totalTransactions int
	for i := startBlock; i <= blockNumber; i++ {
		block, err := client.EthGetBlockByNumber(i, true)
		if err != nil {
			return "", fmt.Errorf("failed to get block %d: %v", i, err)
		}

		totalTransactions += len(block.Transactions)
		log.Printf("Block %d has %d transactions", i, len(block.Transactions))
	}

	log.Printf("Total transactions in the last 10 blocks: %d", totalTransactions)

	if totalTransactions >= 5 {
		return "healthy", nil
	}

	return "", fmt.Errorf("less than 5 transactions in the last 10 blocks")
}

func SendTeamsNotification(webhookURL string, message string, isMarkdown bool) {
	msg := TeamsMessage{
		Type:     "MessageCard",
		Context:  "http://schema.org/extensions",
		Summary:  "Chain Status Warning",
		Title:    "Chain Status Warning",
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
