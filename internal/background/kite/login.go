package kite

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ananthakumaran/paisa/internal/model"
	"github.com/ananthakumaran/paisa/internal/utils"
	"github.com/pquerna/otp/totp"
	log "github.com/sirupsen/logrus"
	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

const (
	BASE_URL        = "https://kite.zerodha.com"
	LOGIN_URL       = BASE_URL + "/api/login"
	TWOFA_URL       = BASE_URL + "/api/twofa"
	INSTRUMENTS_URL = "https://api.kite.trade/instruments"
)

// KiteAccount represents a single KITE account configuration
type KiteAccount struct {
	Name      string `json:"name" yaml:"name"`
	APIKey    string `json:"api_key" yaml:"api_key"`
	APISecret string `json:"api_secret" yaml:"api_secret"`
	UserID    string `json:"user_id" yaml:"user_id"`
	Password  string `json:"password" yaml:"password"`
	TOTPToken string `json:"totp_token" yaml:"totp_token"`
}

// KiteConfig holds the configuration for multiple KITE Connect accounts
type KiteConfig struct {
	Accounts []KiteAccount `json:"accounts" yaml:"accounts"`
}

// LoginResponse represents the response from KITE login
type LoginResponse struct {
	Status string `json:"status"`
	Data   struct {
		RequestID string `json:"request_id"`
	} `json:"data"`
}

// TwoFAResponse represents the response from 2FA
type TwoFAResponse struct {
	Status string `json:"status"`
	Data   struct {
		RequestID string `json:"request_id"`
	} `json:"data"`
}

// generateTOTP generates a TOTP code using the proper OTP library
func generateTOTP(secret string) (string, error) {
	// Generate TOTP code using the proper library
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP code: %w", err)
	}
	return code, nil
}

// This creates a fresh request token and stores it in the database for a specific API key.
func LoginAndStoreTokenForAPIKey(db *gorm.DB, apiKey string) error {
	kiteConfig, err := loadKiteConfig()
	if err != nil {
		return fmt.Errorf("failed to load KITE config: %w", err)
	}

	if len(kiteConfig.Accounts) == 0 {
		return fmt.Errorf("no KITE accounts configured")
	}

	// Find the account with the matching API key
	var targetAccount *KiteAccount
	for i := range kiteConfig.Accounts {
		if kiteConfig.Accounts[i].APIKey == apiKey {
			targetAccount = &kiteConfig.Accounts[i]
			break
		}
	}

	if targetAccount == nil {
		return fmt.Errorf("no account found with API key: %s", apiKey)
	}

	if targetAccount.APIKey == "" || targetAccount.APISecret == "" || targetAccount.UserID == "" || targetAccount.Password == "" || targetAccount.TOTPToken == "" {
		return fmt.Errorf("KITE Connect API credentials not configured for account %s (missing API key, secret, user ID, password, or TOTP token)", targetAccount.Name)
	}

	// Attempt to auto login using saved credentials first. If successful, this should return a request token.
	requestToken, err := DoAutoLogin(targetAccount)
	if err == nil {
		model.StoreRequestToken(db, apiKey, requestToken)
		return nil
	} else {
		log.Errorf("Failed to login with web flow for account %s: %v", targetAccount.Name, err)
		DoManualLogin(targetAccount)
	}

	return nil
}

// FetchAccessTokenFromRequestToken gets an access token from a request token for a specific API key.
func FetchAccessTokenFromRequestToken(apiKey string, requestToken string) (string, error) {
	kiteConfig, err := loadKiteConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load KITE config: %w", err)
	}

	if len(kiteConfig.Accounts) == 0 {
		return "", fmt.Errorf("no KITE accounts configured")
	}

	// Find the account with the matching API key
	var targetAccount *KiteAccount
	for i := range kiteConfig.Accounts {
		if kiteConfig.Accounts[i].APIKey == apiKey {
			targetAccount = &kiteConfig.Accounts[i]
			break
		}
	}

	if targetAccount == nil {
		return "", fmt.Errorf("no account found with API key: %s", apiKey)
	}

	if targetAccount.APIKey == "" || targetAccount.APISecret == "" || targetAccount.UserID == "" || targetAccount.Password == "" || targetAccount.TOTPToken == "" {
		return "", fmt.Errorf("KITE Connect API credentials not configured for account %s (missing API key, secret, user ID, password, or TOTP token)", targetAccount.Name)
	}

	// Calculate the checksum: SHA-256 of api_key + request_token + api_secret
	checksumInput := targetAccount.APIKey + requestToken + targetAccount.APISecret
	checksum := utils.Sha256(checksumInput)

	sessionURL := "https://api.kite.trade/session/token"
	sessionData := url.Values{}
	sessionData.Set("api_key", targetAccount.APIKey)
	sessionData.Set("request_token", requestToken)
	sessionData.Set("checksum", checksum)

	sessionReq, err := http.NewRequest("POST", sessionURL, strings.NewReader(sessionData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create session request: %w", err)
	}
	sessionReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	sessionResp, err := client.Do(sessionReq)
	if err != nil {
		return "", fmt.Errorf("failed to generate session: %w", err)
	}
	defer sessionResp.Body.Close()

	sessionBody, err := io.ReadAll(sessionResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read session response: %w", err)
	}

	var sessionResponse struct {
		Status string `json:"status"`
		Data   struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}

	err = json.Unmarshal(sessionBody, &sessionResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse session response: %w", err)
	}

	if sessionResponse.Status != "success" {
		return "", fmt.Errorf("session generation failed: %s", string(sessionBody))
	}

	accessToken := sessionResponse.Data.AccessToken
	log.Infof("Successfully got access token for account %s: %s", targetAccount.Name, accessToken)

	return accessToken, nil
}

func DoManualLogin(account *KiteAccount) {
	kc := kiteconnect.New(account.APIKey)

	// Get the login URL
	kiteLoginURL := kc.GetLoginURL()
	log.Infof("--------------------------------")
	log.Info("Please login to Kite Connect by visiting the below URL:")
	log.Info(kiteLoginURL)
	log.Infof("--------------------------------")

	log.Info("After authentication, the request token will be automatically captured and stored. Please copy the request token and paste it in the config file.")
	log.Info("Retry the job once after you have completed the login process.")

	// TODO(Nitin) : Add a way to wait for user to press Enter.
	return
}

// DoAutoLogin mimics the web-based Kite Connect authentication flow. It returns a request token if successful.
func DoAutoLogin(account *KiteAccount) (string, error) {

	kc := kiteconnect.New(account.APIKey)
	// This will be like: https://kite.zerodha.com/connect/login?api_key=random_api_key&v=3
	initialLoginURL := kc.GetLoginURL()

	// Create a session with cookies
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects automatically, we want to handle them manually
			return http.ErrUseLastResponse
		},
	}

	// Step 1: Make a call to the login URL.
	sessionReq, err := http.NewRequest("GET", initialLoginURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create session request: %w", err)
	}

	sessionReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	sessionReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	sessionReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	sessionResp, err := client.Do(sessionReq)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}
	defer sessionResp.Body.Close()

	// Extract session ID from the response
	var sessID string

	// Making a call to the login URL will redirect to a URL with session ID.
	location := sessionResp.Header.Get("Location")
	log.Infof("Response status: %d", sessionResp.StatusCode) // This should be 302.
	log.Infof("Response location header: %s", location)      // This will be like: https://kite.zerodha.com/connect/login?api_key=random_api_key&sess_id=random_sess_id

	if location != "" {
		// Extract session ID from redirect URL
		re := regexp.MustCompile(`sess_id=([^&]+)`)
		matches := re.FindStringSubmatch(location)
		if len(matches) >= 2 {
			sessID = matches[1]
			log.Infof("Got session ID from redirect: %s", sessID)
		} else {
			return "", fmt.Errorf("session ID not found in redirect URL: %s", location)
		}
	} else {
		return "", fmt.Errorf("session ID not found in redirect URL: %s", location)
	}

	// Step 2: Make login request with session ID
	loginData := url.Values{}
	loginData.Set("user_id", account.UserID)
	loginData.Set("password", account.Password)

	loginReq, err := http.NewRequest("POST", LOGIN_URL, strings.NewReader(loginData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	loginReq.Header.Set("Accept", "application/json, text/plain, */*")
	loginReq.Header.Set("Accept-Language", "en-US,en;q=0.9")
	loginReq.Header.Set("Referer", fmt.Sprintf("%s/connect/login?api_key=%s&sess_id=%s", BASE_URL, account.APIKey, sessID))
	loginReq.Header.Set("Origin", BASE_URL)

	// Copy cookies from session response
	for _, cookie := range sessionResp.Cookies() {
		loginReq.AddCookie(cookie)
	}

	loginResp, err := client.Do(loginReq)
	if err != nil {
		return "", fmt.Errorf("failed to make login request: %w", err)
	}
	defer loginResp.Body.Close()

	loginBody, err := io.ReadAll(loginResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read login response: %w", err)
	}

	var loginResponse LoginResponse
	err = json.Unmarshal(loginBody, &loginResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse login response: %w", err)
	}

	if loginResponse.Status != "success" {
		return "", fmt.Errorf("login failed: %s", string(loginBody))
	}

	requestID := loginResponse.Data.RequestID
	log.Infof("Got request ID: %s", requestID)

	// Step 3: Two factor authentication
	totpCode, err := generateTOTP(account.TOTPToken)
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP code: %w", err)
	}

	twofaData := url.Values{}
	twofaData.Set("user_id", account.UserID)
	twofaData.Set("request_id", requestID)
	twofaData.Set("twofa_value", totpCode)
	twofaData.Set("twofa_type", "totp")

	twofaReq, err := http.NewRequest("POST", TWOFA_URL, strings.NewReader(twofaData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create 2FA request: %w", err)
	}
	twofaReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	twofaReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	twofaReq.Header.Set("Accept", "application/json, text/plain, */*")
	twofaReq.Header.Set("Accept-Language", "en-US,en;q=0.9")
	twofaReq.Header.Set("Referer", fmt.Sprintf("%s/connect/login?api_key=%s&sess_id=%s", BASE_URL, account.APIKey, sessID))
	twofaReq.Header.Set("Origin", BASE_URL)

	// Copy cookies from login response
	for _, cookie := range loginResp.Cookies() {
		twofaReq.AddCookie(cookie)
	}

	twofaResp, err := client.Do(twofaReq)
	if err != nil {
		return "", fmt.Errorf("failed to make 2FA request: %w", err)
	}
	defer twofaResp.Body.Close()

	twofaBody, err := io.ReadAll(twofaResp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read 2FA response: %w", err)
	}

	var twofaResponse TwoFAResponse
	err = json.Unmarshal(twofaBody, &twofaResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse 2FA response: %w", err)
	}

	if twofaResponse.Status != "success" {
		return "", fmt.Errorf("2FA failed: %s", string(twofaBody))
	}

	log.Info("2FA successful")

	// Step 4: Get the redirect URL to extract request token
	// The 2FA success should trigger a redirect to the callback URL with the request token
	redirectURL := fmt.Sprintf("%s/connect/login?api_key=%s&sess_id=%s", BASE_URL, account.APIKey, sessID)

	redirectReq, err := http.NewRequest("GET", redirectURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create redirect request: %w", err)
	}
	redirectReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	redirectReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	redirectReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Copy cookies from 2FA response
	for _, cookie := range twofaResp.Cookies() {
		redirectReq.AddCookie(cookie)
	}

	redirectResp, err := client.Do(redirectReq)
	if err != nil {
		return "", fmt.Errorf("failed to get redirect: %w", err)
	}
	defer redirectResp.Body.Close()

	// The redirect url above should redirect to a URL like: https://kite.zerodha.com/connect/finish?api_key=random_api_key&sess_id=random_sess_id
	location = redirectResp.Header.Get("Location")

	if location == "" {
		return "", fmt.Errorf("no redirect location found after 2FA")
	}

	// Step 5: Visit the finish URL to get the final redirect with request token
	finishReq, err := http.NewRequest("GET", location, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create finish request: %w", err)
	}
	finishReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	finishReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	finishReq.Header.Set("Accept-Language", "en-US,en;q=0.9")
	finishReq.Header.Set("Referer", fmt.Sprintf("%s/connect/login?api_key=%s&sess_id=%s", BASE_URL, account.APIKey, sessID))

	// Copy cookies from 2FA response
	for _, cookie := range twofaResp.Cookies() {
		finishReq.AddCookie(cookie)
	}

	finishResp, err := client.Do(finishReq)
	if err != nil {
		return "", fmt.Errorf("failed to get finish redirect: %w", err)
	}
	defer finishResp.Body.Close()

	// The response from above request will contain the final redirect url with request token.
	// It will be like: http://localhost:5173/api/callback/kite?action=login&type=login&status=success&request_token=jOMH1zWWreJ47FEVDHLtzIYBxQ6BHLJz
	// The above URL is what was specified in the callback URL in the kite connect app.
	finalLocation := finishResp.Header.Get("Location")
	if finalLocation == "" {
		return "", fmt.Errorf("No final redirect location.")
	}

	// Extract request_token from the final redirect URL
	re := regexp.MustCompile(`request_token=([^&]+)`)
	matches := re.FindStringSubmatch(finalLocation)
	if len(matches) < 2 {
		return "", fmt.Errorf("request token not found in final redirect URL: %s", finalLocation)
	}

	requestToken := matches[1]
	log.Infof("Got request token from final redirect: %s", requestToken)

	return requestToken, nil
}
