package ledger

import "testing"

func TestChartOfAccounts_CreateAccount_Valid(t *testing.T) {
	db := newTestDB(t)

	id, err := CreateAccount(db, NewAccount{
		Code: "1000",
		Name: "Cash",
		Type: "asset",
	})
	if err != nil {
		t.Fatalf("CreateAccount: unexpected error: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateAccount: expected positive ID, got %d", id)
	}
}

func TestChartOfAccounts_CreateAccount_InvalidType(t *testing.T) {
	db := newTestDB(t)

	_, err := CreateAccount(db, NewAccount{
		Code: "1001",
		Name: "Bogus Account",
		Type: "bogus",
	})
	if err == nil {
		t.Fatal("CreateAccount: expected error for invalid type, got nil")
	}

	var count int
	if scanErr := db.QueryRow(`SELECT COUNT(*) FROM accounts WHERE code = ?`, "1001").Scan(&count); scanErr != nil {
		t.Fatalf("failed to query accounts: %v", scanErr)
	}
	if count != 0 {
		t.Fatalf("expected no row inserted for invalid type, found %d", count)
	}
}

func TestChartOfAccounts_CreateAccount_ParentNotFound(t *testing.T) {
	db := newTestDB(t)

	missingParent := int64(999999)
	_, err := CreateAccount(db, NewAccount{
		Code:     "1100",
		Name:     "Orphan Sub-Account",
		Type:     "asset",
		ParentID: &missingParent,
	})
	if err == nil {
		t.Fatal("CreateAccount: expected error for non-existent parent, got nil")
	}
}

func TestChartOfAccounts_CreateAccount_ParentExists(t *testing.T) {
	db := newTestDB(t)

	parentID, err := CreateAccount(db, NewAccount{
		Code: "2000",
		Name: "Liabilities",
		Type: "liability",
	})
	if err != nil {
		t.Fatalf("CreateAccount (parent): unexpected error: %v", err)
	}

	childID, err := CreateAccount(db, NewAccount{
		Code:     "2001",
		Name:     "Accounts Payable",
		Type:     "liability",
		ParentID: &parentID,
	})
	if err != nil {
		t.Fatalf("CreateAccount (child): unexpected error: %v", err)
	}

	var gotParentID int64
	if scanErr := db.QueryRow(`SELECT parent_id FROM accounts WHERE id = ?`, childID).Scan(&gotParentID); scanErr != nil {
		t.Fatalf("failed to query child account: %v", scanErr)
	}
	if gotParentID != parentID {
		t.Fatalf("expected parent_id %d, got %d", parentID, gotParentID)
	}
}

func TestChartOfAccounts_CreateAccount_DuplicateCode(t *testing.T) {
	db := newTestDB(t)

	if _, err := CreateAccount(db, NewAccount{
		Code: "3000",
		Name: "Equity",
		Type: "equity",
	}); err != nil {
		t.Fatalf("CreateAccount (first): unexpected error: %v", err)
	}

	_, err := CreateAccount(db, NewAccount{
		Code: "3000",
		Name: "Equity Duplicate",
		Type: "equity",
	})
	if err == nil {
		t.Fatal("CreateAccount: expected error for duplicate code, got nil")
	}
}

func TestChartOfAccounts_CreateAccount_IsActiveDefaultsTrue(t *testing.T) {
	db := newTestDB(t)

	id, err := CreateAccount(db, NewAccount{
		Code: "4000",
		Name: "Revenue",
		Type: "revenue",
	})
	if err != nil {
		t.Fatalf("CreateAccount: unexpected error: %v", err)
	}

	var isActive int
	if scanErr := db.QueryRow(`SELECT is_active FROM accounts WHERE id = ?`, id).Scan(&isActive); scanErr != nil {
		t.Fatalf("failed to query account: %v", scanErr)
	}
	if isActive != 1 {
		t.Fatalf("expected is_active = 1 by default, got %d", isActive)
	}
}
