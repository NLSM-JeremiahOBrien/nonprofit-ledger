package ledger

import (
	"database/sql"
	"fmt"
)

// validNetAssetClasses is the current FASB ASC 958 two-category net-asset
// classification model. The deprecated pre-2018 three-category model
// (unrestricted / temporarily restricted / permanently restricted) is
// intentionally NOT accepted here, even as aliases; that older
// classification survives only as optional free-text in
// RestrictionDetail. Kept in sync with the CHECK constraint on the
// funds table in server/db/migrations/0002_funds.up.sql.
var validNetAssetClasses = map[string]bool{
	"without_donor_restrictions": true,
	"with_donor_restrictions":    true,
}

// Fund represents a fund classified per current FASB ASC 958 net-asset
// categories.
type Fund struct {
	ID                int64
	Code              string
	Name              string
	NetAssetClass     string // "without_donor_restrictions"|"with_donor_restrictions"
	RestrictionDetail *string
	IsActive          bool
}

// NewFund is the input to CreateFund.
type NewFund struct {
	Code              string
	Name              string
	NetAssetClass     string
	RestrictionDetail *string
	// IsActive defaults to true when not explicitly set to false via
	// IsActiveSet. Most callers can leave both fields zero-valued.
	IsActive    bool
	IsActiveSet bool
}

// CreateFund validates and inserts a new fund row. NetAssetClass must be
// one of the two current ASC 958 categories; the deprecated
// three-category model is rejected outright, before any SQL is issued.
func CreateFund(db *sql.DB, f NewFund) (int64, error) {
	if f.Code == "" {
		return 0, fmt.Errorf("ledger: fund code is required")
	}
	if !validNetAssetClasses[f.NetAssetClass] {
		return 0, fmt.Errorf("ledger: invalid net_asset_class %q (must be one of: without_donor_restrictions, with_donor_restrictions)", f.NetAssetClass)
	}

	isActive := 1
	if f.IsActiveSet && !f.IsActive {
		isActive = 0
	}

	res, err := db.Exec(
		`INSERT INTO funds (code, name, net_asset_class, restriction_detail, is_active) VALUES (?, ?, ?, ?, ?)`,
		f.Code, f.Name, f.NetAssetClass, f.RestrictionDetail, isActive,
	)
	if err != nil {
		return 0, fmt.Errorf("ledger: create fund: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("ledger: create fund: %w", err)
	}

	return id, nil
}
