package cli

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/janpereira-dev/quantum_log/internal/config"
)

func authenticatedCollector(home, listen string, next http.Handler) http.Handler {
	paths, err := config.Resolve(home)
	var token string
	if err == nil {
		token, err = config.CollectorToken(paths)
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !collectorRequestOriginAllowed(request, listen) {
			http.Error(writer, "collector request origin is not allowed", http.StatusForbidden)
			return
		}
		if err != nil || token == "" {
			http.Error(writer, "collector authentication is unavailable", http.StatusServiceUnavailable)
			return
		}
		if subtle.ConstantTimeCompare([]byte(request.Header.Get("X-Qlog-Token")), []byte(token)) != 1 {
			http.Error(writer, "collector authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func collectorRequestOriginAllowed(request *http.Request, listen string) bool {
	host := request.Host
	if withPort, _, err := net.SplitHostPort(host); err == nil {
		host = withPort
	}
	host = strings.Trim(host, "[]")
	allowed := loopbackHost(host)
	if configured, _, err := net.SplitHostPort(listen); err == nil {
		if configured == "" || configured == "0.0.0.0" || configured == "::" {
			allowed = net.ParseIP(host) != nil
		} else if host == configured {
			allowed = true
		}
	}
	if !allowed {
		return false
	}
	if origin := request.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host == request.Host && parsed.User == nil
	}
	return true
}
