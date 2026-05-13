package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const httpTimeout = 15 * time.Second

// GoogleProfile holds normalized identity from Google userinfo.
type GoogleProfile struct {
	ProviderUserID string
	Email          string
	EmailVerified  bool
}

// GitHubProfile holds normalized identity from GitHub API.
type GitHubProfile struct {
	ProviderUserID string
	Email          string
	EmailVerified  bool
}

// FetchGoogleUserinfo calls Google's userinfo endpoint (requires verified_email for trust).
func FetchGoogleUserinfo(ctx context.Context, accessToken string) (*GoogleProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}
	var raw struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.ID == "" || strings.TrimSpace(raw.Email) == "" {
		return nil, fmt.Errorf("google userinfo: missing id or email")
	}
	return &GoogleProfile{
		ProviderUserID: raw.ID,
		Email:          strings.TrimSpace(strings.ToLower(raw.Email)),
		EmailVerified:  raw.VerifiedEmail,
	}, nil
}

type ghUser struct {
	ID int64 `json:"id"`
}

type ghEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// FetchGitHubProfile resolves primary verified email and numeric user id as string.
func FetchGitHubProfile(ctx context.Context, accessToken string) (*GitHubProfile, error) {
	client := &http.Client{Timeout: httpTimeout}

	reqUser, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	reqUser.Header.Set("Authorization", "Bearer "+accessToken)
	reqUser.Header.Set("Accept", "application/vnd.github+json")
	respUser, err := client.Do(reqUser)
	if err != nil {
		return nil, err
	}
	defer respUser.Body.Close()
	bUser, err := io.ReadAll(io.LimitReader(respUser.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if respUser.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user: status %d", respUser.StatusCode)
	}
	var u ghUser
	if err := json.Unmarshal(bUser, &u); err != nil {
		return nil, err
	}
	if u.ID == 0 {
		return nil, fmt.Errorf("github user: missing id")
	}

	reqEm, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return nil, err
	}
	reqEm.Header.Set("Authorization", "Bearer "+accessToken)
	reqEm.Header.Set("Accept", "application/vnd.github+json")
	respEm, err := client.Do(reqEm)
	if err != nil {
		return nil, err
	}
	defer respEm.Body.Close()
	bEm, err := io.ReadAll(io.LimitReader(respEm.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if respEm.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github emails: status %d", respEm.StatusCode)
	}
	var emails []ghEmail
	if err := json.Unmarshal(bEm, &emails); err != nil {
		return nil, err
	}
	var chosen string
	for _, e := range emails {
		if !e.Verified {
			continue
		}
		if e.Primary && strings.TrimSpace(e.Email) != "" {
			chosen = strings.TrimSpace(strings.ToLower(e.Email))
			break
		}
	}
	if chosen == "" {
		for _, e := range emails {
			if e.Verified && strings.TrimSpace(e.Email) != "" {
				chosen = strings.TrimSpace(strings.ToLower(e.Email))
				break
			}
		}
	}
	if chosen == "" {
		return nil, fmt.Errorf("github: no verified email")
	}

	return &GitHubProfile{
		ProviderUserID: fmt.Sprintf("%d", u.ID),
		Email:          chosen,
		EmailVerified:  true,
	}, nil
}
