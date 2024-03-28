package model

type SwanChainData struct {
	ID            int    `db:"id"`
	WalletAddress string `db:"wallet_address"`
	Balance       string `db:"balance"`
	BalanceChange string `db:"balance_change"`
	NetworkEnv    string `db:"network_env"`
	UpdatedAt     string `db:"update_at"`
}
