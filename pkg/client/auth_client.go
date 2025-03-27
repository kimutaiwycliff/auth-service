package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

type AuthClient struct {
	baseURL string
	client  *resty.Client
}

func New(baseURL string) *AuthClient {
	return &AuthClient{
		baseURL: baseURL,
		client:  resty.New().SetTimeout(5 * time.Second),
	}
}

func (c *AuthClient) Register(email, password string) (string, error) {
	var result struct {
		ID string `json:"id"`
	}

	resp, err := c.client.R().
		SetBody(map[string]string{
			"email":    email,
			"password": password,
		}).
		SetResult(&result).
		Post(c.baseURL + "/api/v1/auth/register")

	if err != nil {
		return "", err
	}

	if resp.StatusCode() != http.StatusCreated {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	return result.ID, nil
}

func (c *AuthClient) Login(email, password string) (string, string, error) {
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	resp, err := c.client.R().
		SetBody(map[string]string{
			"email":    email,
			"password": password,
		}).
		SetResult(&result).
		Post(c.baseURL + "/api/v1/auth/login")

	if err != nil {
		return "", "", err
	}

	if resp.StatusCode() != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	return result.AccessToken, result.RefreshToken, nil
}

func (c *AuthClient) VerifyToken(ctx context.Context, token string) (string, error) {
	var result struct {
		UserID string `json:"user_id"`
	}

	resp, err := c.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+token).
		SetResult(&result).
		Get(c.baseURL + "/api/v1/auth/verify")

	if err != nil {
		return "", err
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	return result.UserID, nil
}
