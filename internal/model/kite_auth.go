package model

import (
	"time"

	"gorm.io/gorm"
)

// KiteAuth stores Kite Connect authentication data for each account
type KiteAuth struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	APIKey       string    `json:"api_key" gorm:"uniqueIndex"` // Added to support multiple accounts
	RequestToken string    `json:"request_token"`
	AccessToken  string    `json:"access_token"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName specifies the table name for KiteAuth
func (KiteAuth) TableName() string {
	return "kite_auth"
}

// GetAuthByAPIKey retrieves authentication data for a specific API key
func GetAuthByAPIKey(db *gorm.DB, apiKey string) (*KiteAuth, error) {
	var auth KiteAuth
	err := db.Where("api_key = ?", apiKey).First(&auth).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &auth, nil
}

// GetLatestAuth retrieves the current Kite authentication data (for backward compatibility)
func GetLatestAuth(db *gorm.DB) (*KiteAuth, error) {
	var auth KiteAuth
	err := db.First(&auth).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &auth, nil
}

// StoreRequestToken stores a new request token for a specific API key
func StoreRequestToken(db *gorm.DB, apiKey string, requestToken string) error {
	// Use Upsert to either update existing entry or create new one
	var auth KiteAuth
	result := db.Where("api_key = ?", apiKey).First(&auth)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// No existing entry, create new one
			auth = KiteAuth{
				APIKey:       apiKey,
				RequestToken: requestToken,
			}
			return db.Create(&auth).Error
		}
		return result.Error
	}

	// Update existing entry
	auth.RequestToken = requestToken
	return db.Save(&auth).Error
}

// UpdateAccessToken updates the access token for a specific API key
func UpdateAccessToken(db *gorm.DB, apiKey string, accessToken string) error {
	// First get the auth record for this API key
	auth, err := GetAuthByAPIKey(db, apiKey)
	if err != nil {
		return err
	}

	if auth == nil {
		return gorm.ErrRecordNotFound
	}

	// Update the specific record
	return db.Model(auth).
		Update("access_token", accessToken).Error
}

// ClearAuth clears all authentication data
func ClearAuth(db *gorm.DB) error {
	return db.Delete(&KiteAuth{}).Error
}

// ClearAuthByAPIKey clears authentication data for a specific API key
func ClearAuthByAPIKey(db *gorm.DB, apiKey string) error {
	return db.Where("api_key = ?", apiKey).Delete(&KiteAuth{}).Error
}
