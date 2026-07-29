package repository

import "testing"

func TestValidateMySQLDSNRejectsEmptyValue(t *testing.T) {
	err := validateMySQLDSN("")
	if err == nil {
		t.Fatal("expected an error for an empty MySQL DSN")
	}
}
