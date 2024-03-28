package wallet

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/swanchain/domain-check/pkg/model"
)

func TestCheckBalance(t *testing.T) {
	balance, err := CheckBalance("http://test-rpc-url", "test-wallet-address")
	if err != nil {
		t.Errorf("CheckBalance() returned error: %v", err)
	}

	if balance != 10.0 {
		t.Errorf("Unexpected balance: got %v, want 10.0", balance)
	}
}

func TestSetExplorerAndRpcVars(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"key", "value"}).
		AddRow("sepolia-rpc", "test-sepolia-rpc").
		AddRow("saturn-rpc", "test-saturn-rpc").
		AddRow("saturn-block-explorer", "test-saturn-block-explorer").
		AddRow("sepolia-block-explorer", "test-sepolia-block-explorer")

	mock.ExpectQuery("SELECT key, value FROM info WHERE key IN").WillReturnRows(rows)

	err = SetExplorerAndRpcVars(sqlxDB)
	if err != nil {
		t.Errorf("SetExplorerAndRpcVars() returned error: %v", err)
	}
}

func TestGetL1Wallet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"key", "value", "type"}).
		AddRow("l1-test", "test-wallet-address", "wallet-address")

	mock.ExpectQuery("SELECT \\* FROM info WHERE key ILIKE 'l1%' AND type = 'wallet-address'").WillReturnRows(rows)

	wallets, err := GetL1Wallet(sqlxDB)
	if err != nil {
		t.Errorf("GetL1Wallet() returned error: %v", err)
	}

	if len(wallets) != 1 {
		t.Errorf("Unexpected number of wallets: got %v, want 1", len(wallets))
	}
}

func TestGetL2Wallet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	rows := sqlmock.NewRows([]string{"key", "value", "type"}).
		AddRow("l2-test", "test-wallet-address", "wallet-address")

	mock.ExpectQuery("SELECT \\* FROM info WHERE key ILIKE 'l2%' AND type = 'wallet-address'").WillReturnRows(rows)

	wallets, err := GetL2Wallet(sqlxDB)
	if err != nil {
		t.Errorf("GetL2Wallet() returned error: %v", err)
	}

	if len(wallets) != 1 {
		t.Errorf("Unexpected number of wallets: got %v, want 1", len(wallets))
	}
}

// Note: Testing CheckBalance, CheckSepoliaBalance, and CheckSwanBalance would require mocking HTTP requests, which is beyond the scope of this example.

func TestUpdateL1Wallet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	mock.ExpectQuery("SELECT balance FROM swan_chain_data WHERE wallet_address = \\$1").
		WithArgs("test-wallet-address").
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(5.0))

	mock.ExpectExec("UPDATE swan_chain_data SET balance = \\$1, balance_change = \\$2, network_env = \\$3, update_at = \\$4 WHERE wallet_address = \\$5").
		WithArgs(fmt.Sprintf("%f", 10.0), fmt.Sprintf("%f", 5.0), "sepolia", sqlmock.AnyArg(), "test-wallet-address").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = UpdateL1Wallet(sqlxDB, model.Config{Key: "test-key", Value: "test-wallet-address"}, 10.0)
	if err != nil {
		t.Errorf("UpdateL1Wallet() returned error: %v", err)
	}
}

func TestUpdateL2Wallet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")

	mock.ExpectQuery("SELECT balance FROM swan_chain_data WHERE wallet_address = \\$1").
		WithArgs("test-wallet-address").
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(5.0))

	mock.ExpectExec("UPDATE swan_chain_data SET balance = \\$1, balance_change = \\$2, network_env = \\$3, update_at = \\$4 WHERE wallet_address = \\$5").
		WithArgs(fmt.Sprintf("%f", 10.0), fmt.Sprintf("%f", 5.0), "swan", sqlmock.AnyArg(), "test-wallet-address").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = UpdateL2Wallet(sqlxDB, model.Config{Key: "test-key", Value: "test-wallet-address"}, 10.0)
	if err != nil {
		t.Errorf("UpdateL2Wallet() returned error: %v", err)
	}
}
