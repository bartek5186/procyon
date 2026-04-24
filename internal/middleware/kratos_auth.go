package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/bartek5186/procyon/internal/apierr"
	"github.com/labstack/echo/v4"
	ory "github.com/ory/client-go"
)

const ContextKeySession = "kratosSession"

var kratosHTTP = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	Timeout: 2 * time.Second,
}

type KratosAuth struct {
	publicBaseURL string
	httpClient    *http.Client
}

func NewKratosAuth(kratosPublicURL string) *KratosAuth {
	return &KratosAuth{
		publicBaseURL: strings.TrimRight(kratosPublicURL, "/"),
		httpClient:    kratosHTTP,
	}
}

func (a *KratosAuth) RequireSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		httpReq, err := http.NewRequestWithContext(req.Context(), http.MethodGet, a.publicBaseURL+"/sessions/whoami", nil)
		if err != nil {
			return apierr.ReplyUnauthorized(c, "unauthorized")
		}

		if tok := req.Header.Get("X-Session-Token"); tok != "" {
			httpReq.Header.Set("X-Session-Token", tok)
		} else if h := req.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
			httpReq.Header.Set("Authorization", h)
		} else if cookie := req.Header.Get("Cookie"); cookie != "" {
			httpReq.Header.Set("Cookie", cookie)
		}

		resp, err := a.httpClient.Do(httpReq)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			return apierr.ReplyUnauthorized(c, "unauthorized")
		}
		defer resp.Body.Close()

		var session ory.Session
		if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
			return apierr.ReplyUnauthorized(c, "unauthorized")
		}

		c.Set(ContextKeySession, &session)
		return next(c)
	}
}

func SessionFromContext(c echo.Context) (*ory.Session, bool) {
	session, ok := c.Get(ContextKeySession).(*ory.Session)
	return session, ok
}
