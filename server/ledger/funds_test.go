package ledger

import "testing"

func TestFundsSchema_CreateFund_WithoutDonorRestrictions(t *testing.T) {
	db := newTestDB(t)

	id, err := CreateFund(db, NewFund{
		Code:          "GEN",
		Name:          "General Operating",
		NetAssetClass: "without_donor_restrictions",
	})
	if err != nil {
		t.Fatalf("CreateFund: unexpected error: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateFund: expected positive ID, got %d", id)
	}
}

func TestFundsSchema_CreateFund_WithDonorRestrictions(t *testing.T) {
	db := newTestDB(t)

	id, err := CreateFund(db, NewFund{
		Code:          "REST1",
		Name:          "Restricted Program Fund",
		NetAssetClass: "with_donor_restrictions",
	})
	if err != nil {
		t.Fatalf("CreateFund: unexpected error: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateFund: expected positive ID, got %d", id)
	}
}

func TestFundsSchema_CreateFund_RejectsDeprecatedThreeCategoryValues(t *testing.T) {
	db := newTestDB(t)

	deprecated := []string{"unrestricted", "temporarily_restricted", "permanently_restricted", "bogus"}

	for i, class := range deprecated {
		_, err := CreateFund(db, NewFund{
			Code:          "DEP" + string(rune('A'+i)),
			Name:          "Deprecated Class Fund",
			NetAssetClass: class,
		})
		if err == nil {
			t.Fatalf("CreateFund: expected error for deprecated net_asset_class %q, got nil", class)
		}
	}
}

func TestFundsSchema_CreateFund_RestrictionDetailRoundTrips(t *testing.T) {
	db := newTestDB(t)

	detail := "endowment"
	id, err := CreateFund(db, NewFund{
		Code:              "ENDOW1",
		Name:              "Endowment Fund",
		NetAssetClass:     "with_donor_restrictions",
		RestrictionDetail: &detail,
	})
	if err != nil {
		t.Fatalf("CreateFund: unexpected error: %v", err)
	}

	var got string
	if scanErr := db.QueryRow(`SELECT restriction_detail FROM funds WHERE id = ?`, id).Scan(&got); scanErr != nil {
		t.Fatalf("failed to query fund: %v", scanErr)
	}
	if got != detail {
		t.Fatalf("expected restriction_detail %q, got %q", detail, got)
	}
}

func TestFundsSchema_CreateFund_DuplicateCode(t *testing.T) {
	db := newTestDB(t)

	if _, err := CreateFund(db, NewFund{
		Code:          "DUP1",
		Name:          "Duplicate Fund",
		NetAssetClass: "without_donor_restrictions",
	}); err != nil {
		t.Fatalf("CreateFund (first): unexpected error: %v", err)
	}

	_, err := CreateFund(db, NewFund{
		Code:          "DUP1",
		Name:          "Duplicate Fund Again",
		NetAssetClass: "without_donor_restrictions",
	})
	if err == nil {
		t.Fatal("CreateFund: expected error for duplicate code, got nil")
	}
}
