package model

type SwanChainData struct {
	ID            int    `db:"id"`
	WalletAddress string `db:"wallet_address"`
	Balance       string `db:"balance"`
	BalanceChange string `db:"balance_change"`
	Explorer      string `db:"explorer"`
	NetworkEnv    string `db:"network_env"`
	UpdatedAt     string `db:"updated_at"`
}
