package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
)

// Azure B2C OAuth2 constants
const (
	clientID    = "09ea4903-9e95-45fe-ae1f-e3b7d32fa385"
	policy      = "b2c_1a_signuporsigninonline"
	redirectURI = "https://online.thermia.se/login"
	scope       = clientID + " offline_access openid"

	baseB2C      = "https://thermialogin.b2clogin.com"
	tenantDomain = "thermialogin.onmicrosoft.com"

	authorizeURL = baseB2C + "/" + tenantDomain + "/" + policy + "/oauth2/v2.0/authorize"
	tokenURL     = baseB2C + "/" + tenantDomain + "/" + policy + "/oauth2/v2.0/token"
	selfURL      = baseB2C + "/" + tenantDomain + "/" + policy + "/SelfAsserted"
	confirmURL   = baseB2C + "/" + tenantDomain + "/" + policy + "/api/CombinedSigninAndSignup/confirmed"
)

var errNeedSelfAsserted = errors.New("need SelfAsserted step")

// Credentials holds authentication credentials.
type Credentials struct {
	Username string
	Password string
}

// AuthResult contains the result of a successful authentication.
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

// authState holds intermediate authentication state.
type authState struct {
	Code       string
	CSRF       string
	StateProps string
	Cookies    []*http.Cookie
}

// AuthClient handles OAuth2 authentication with Azure B2C.
type AuthClient struct {
	httpClient *http.Client
	logger     *slog.Logger
}

// NewAuthClient creates a new authentication client.
func NewAuthClient(logger *slog.Logger) *AuthClient {
	jar, _ := cookiejar.New(nil)

	return &AuthClient{
		httpClient: &http.Client{
			Timeout: 30 * 1000 * 1000 * 1000, // 30 seconds in nanoseconds
			Jar:     jar,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * 1000 * 1000 * 1000, // 90 seconds
			},
		},
		logger: logger,
	}
}

// Authenticate performs the full OAuth2 PKCE authentication flow.
func (a *AuthClient) Authenticate(ctx context.Context, creds Credentials) (*AuthResult, error) {
	a.logger.Debug("Starting authentication", "username", creds.Username)

	verifier := generatePKCEVerifier()
	challenge := generatePKCEChallenge(verifier)

	// Step 1: Start authorization
	state, err := a.startAuthorize(ctx, challenge)
	if err != nil && !errors.Is(err, errNeedSelfAsserted) {
		a.logger.Error("Authorization failed", "error", err)
		return nil, fmt.Errorf("start authorize: %w", err)
	}

	// Step 2: Self-asserted login (if needed)
	if errors.Is(err, errNeedSelfAsserted) {
		a.logger.Debug("Performing self-asserted login")
		if err := a.doSelfAsserted(ctx, creds, state); err != nil {
			a.logger.Error("Self-asserted login failed", "error", err)
			return nil, fmt.Errorf("self-asserted: %w", err)
		}

		// Step 3: Confirm and get authorization code
		state.Code, err = a.confirmAndGetCode(ctx, state)
		if err != nil {
			a.logger.Error("Confirm failed", "error", err)
			return nil, fmt.Errorf("confirm: %w", err)
		}
	}

	// Step 4: Exchange authorization code for access token
	result, err := a.exchangeCode(ctx, state.Code, verifier)
	if err != nil {
		a.logger.Error("Token exchange failed", "error", err)
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	a.logger.Info("Authentication successful", "username", creds.Username)
	return result, nil
}

// startAuthorize initiates the OAuth2 authorization flow.
func (a *AuthClient) startAuthorize(ctx context.Context, challenge string) (*authState, error) {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("scope", scope)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	req, _ := http.NewRequestWithContext(ctx, "GET", authorizeURL+"?"+q.Encode(), nil)
	res, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	// Extract SETTINGS JSON from HTML
	setJSON := extractSettings(string(body))
	if setJSON == "" {
		return nil, errors.New("SETTINGS JSON not found in response")
	}

	var settings struct {
		TransId string `json:"transId"`
		Csrf    string `json:"csrf"`
	}
	if err := json.Unmarshal([]byte(setJSON), &settings); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}

	parts := strings.Split(settings.TransId, "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected transId format: %s", settings.TransId)
	}

	state := &authState{
		CSRF:       settings.Csrf,
		StateProps: parts[1],
		Cookies:    res.Cookies(),
	}

	// Check if we already got the code (user already authenticated)
	if code := res.Request.URL.Query().Get("code"); code != "" {
		state.Code = code
		return state, nil
	}

	return state, errNeedSelfAsserted
}

// doSelfAsserted performs the self-asserted login step.
func (a *AuthClient) doSelfAsserted(ctx context.Context, creds Credentials, state *authState) error {
	form := url.Values{}
	form.Set("request_type", "RESPONSE")
	form.Set("signInName", creds.Username)
	form.Set("password", creds.Password)

	u, _ := url.Parse(selfURL)
	q := u.Query()
	q.Set("tx", "StateProperties="+state.StateProps)
	q.Set("p", "B2C_1A_SignUpOrSigninOnline")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "POST", u.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Csrf-Token", state.CSRF)
	for _, c := range state.Cookies {
		req.AddCookie(c)
	}

	res, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	if res.StatusCode/100 != 2 || strings.Contains(string(b), `"status":"400"`) {
		return fmt.Errorf("self-asserted failed (status %d): %s", res.StatusCode, string(b))
	}

	return nil
}

// confirmAndGetCode confirms the login and retrieves the authorization code.
func (a *AuthClient) confirmAndGetCode(ctx context.Context, state *authState) (string, error) {
	u, _ := url.Parse(confirmURL)
	q := u.Query()
	q.Set("csrf_token", state.CSRF)
	q.Set("tx", "StateProperties="+state.StateProps)
	q.Set("p", "B2C_1A_SignUpOrSigninOnline")
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	for _, c := range state.Cookies {
		req.AddCookie(c)
	}

	res, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// Check if we got redirected to the callback URL with a code
	final := res.Request.URL
	if strings.HasPrefix(final.String(), redirectURI) {
		if code := final.Query().Get("code"); code != "" {
			return code, nil
		}
	}

	// Try one more redirect
	r2, err := a.httpClient.Get(final.String())
	if err != nil {
		return "", err
	}
	defer r2.Body.Close()

	if strings.HasPrefix(r2.Request.URL.String(), redirectURI) {
		if code := r2.Request.URL.Query().Get("code"); code != "" {
			return code, nil
		}
	}

	return "", errors.New("no authorization code returned")
}

// exchangeCode exchanges the authorization code for an access token.
func (a *AuthClient) exchangeCode(ctx context.Context, code, verifier string) (*AuthResult, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURI)
	form.Set("scope", scope)
	form.Set("code", code)
	form.Set("code_verifier", verifier)

	req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")

	res, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("token endpoint returned %d: %s", res.StatusCode, string(b))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(b, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, errors.New("no access_token in response")
	}

	return &AuthResult{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}, nil
}

// extractSettings extracts the SETTINGS JSON from the HTML response.
func extractSettings(html string) string {
	re := regexp.MustCompile(`var SETTINGS = ([\s\S]*?});`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
