package model

type Info struct {
	ID       int    `db:"id"`
	Key      string `db:"key"`
	Value    string `db:"value"`
	Type     string `db:"type"`
	IsActive bool   `db:"is_active"`
	Note     string `db:"note"`
}
