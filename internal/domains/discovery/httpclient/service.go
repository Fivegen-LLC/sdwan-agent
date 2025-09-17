package httpclient

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

const (
	discoveryRetryCount = 3
	discoveryReqTimeout = time.Second * 5
	orchNodeStatePath   = "/api/v1/dr/instance/state"
	primaryState        = "primary"
)

type Service struct {
	client *resty.Client
}

func NewService() *Service {
	client := resty.New().
		SetRetryCount(discoveryRetryCount).
		SetTimeout(discoveryReqTimeout).
		SetScheme("https").
		SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // we are using self-signed certificate

	// Close connection after each request
	// Devices can switch transport path to orchestrator, this can cause problems if we stay keep connection open
	client.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
		r.SetHeader("Connection", "close")
		return nil
	})

	return &Service{
		client: client,
	}
}

func (s *Service) CheckPrimary(host string) (isPrimary bool, err error) {
	var respBody struct {
		Data struct {
			State string `json:"state"`
		} `json:"data"`
	}
	resp, err := s.client.R().
		SetResult(&respBody).
		Get(fmt.Sprintf("%s%s", host, orchNodeStatePath))
	if err != nil {
		return isPrimary, fmt.Errorf("CheckPrimary: %w", err)
	}

	if resp.IsError() {
		return isPrimary, fmt.Errorf("CheckPrimary: %d %s: %w", resp.StatusCode(), resp.Status(), errs.ErrAPIError)
	}

	return respBody.Data.State == primaryState, nil
}
