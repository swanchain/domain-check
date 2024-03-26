package database

import (
	"testing"
)

func TestConnectToDB(t *testing.T) {
	db, err := ConnectToDB()
	if err != nil {
		t.Fatalf("ConnectToDB() returned error: %v", err)
	}
	defer db.Close()

	var result string
	err = db.Get(&result, "SELECT 'success'")
	if err != nil {
		t.Errorf("Failed to perform read operation: %v", err)
	}

	if result != "success" {
		t.Errorf("Unexpected result: got %v, want 'success'", result)
	}
}
