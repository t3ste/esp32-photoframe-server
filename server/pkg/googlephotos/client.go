package googlephotos

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type ConfigProvider interface {
	GetGoogleConfig() (Config, error)
}

type TokenStore interface {
	GetToken() (*oauth2.Token, error)
	SaveToken(*oauth2.Token) error
}

type Client struct {
	configProvider ConfigProvider
	store          TokenStore
	client         *http.Client
}

func NewClient(provider ConfigProvider, store TokenStore) *Client {
	return &Client{
		configProvider: provider,
		store:          store,
	}
}

func (c *Client) getOAuthConfig() (*oauth2.Config, error) {
	cfg, err := c.configProvider.GetGoogleConfig()
	if err != nil {
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/photospicker.mediaitems.readonly",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}, nil
}

func (c *Client) GetAuthURL() string {
	conf, err := c.getOAuthConfig()
	if err != nil {
		return "" // TODO: Handle error better?
	}
	// prompt=consent ensures we get a refresh token
	return conf.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
}

func (c *Client) Exchange(code string) error {
	conf, err := c.getOAuthConfig()
	if err != nil {
		return err
	}
	token, err := conf.Exchange(context.Background(), code)
	if err != nil {
		return err
	}
	if err := c.store.SaveToken(token); err != nil {
		return err
	}

	// Reset the cached client so the next request rebuilds it with the new token
	c.client = nil
	return nil
}

func (c *Client) GetClient() (*http.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	token, err := c.store.GetToken()
	if err != nil {
		return nil, err
	}

	conf, err := c.getOAuthConfig()
	if err != nil {
		return nil, err
	}

	c.client = conf.Client(context.Background(), token)
	return c.client, nil
}
