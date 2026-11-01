package cli

import (
	"net/http"
	"time"

	"github.com/janpereira-dev/quantum_log/internal/config"
)

func collectorHTTPSClient(home string, timeout time.Duration) (*http.Client, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return nil, err
	}
	trust, err := config.CollectorClientTLS(paths)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = trust
	return &http.Client{
		Timeout: timeout, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}
