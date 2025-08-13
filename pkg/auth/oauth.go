package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	ScopeGmailModify  = "https://www.googleapis.com/auth/gmail.modify"
	ScopeGmailFull    = "https://mail.google.com/"
	GoogleUserInfoAPI = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// UserInfo represents the user information from Google OAuth
type UserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// AuthResponse represents the complete authentication response
type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	UserInfo     *UserInfo `json:"user_info"`
}

func NewGoogleOAuth2Config() (*oauth2.Config, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil, fmt.Errorf("missing Google OAuth2 env vars")
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{ScopeGmailFull, "openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}, nil
}

func ExchangeCode(ctx context.Context, conf *oauth2.Config, code string) (*oauth2.Token, error) {
	return conf.Exchange(ctx, code)
}

func Client(ctx context.Context, conf *oauth2.Config, token *oauth2.Token) *http.Client {
	return conf.Client(ctx, token)
}

// GetUserInfo retrieves user information using the access token
func GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", GoogleUserInfoAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &userInfo, nil
}

// ExchangeCodeWithUserInfo exchanges code for token and retrieves user info
func ExchangeCodeWithUserInfo(ctx context.Context, conf *oauth2.Config, code string) (*AuthResponse, error) {
	token, err := conf.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}

	userInfo, err := GetUserInfo(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("getting user info: %w", err)
	}

	return &AuthResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		ExpiresIn:    int(token.Expiry.Unix()),
		UserInfo:     userInfo,
	}, nil
}
