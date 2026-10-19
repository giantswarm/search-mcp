package search

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const signInFailedHeading = "<h1>Sign-in failed</h1>"

// fakeAuthManager answers the OAuth routes' calls without a provider.
type fakeAuthManager struct {
	initiateErr error
	callbackErr error
}

func (f *fakeAuthManager) GetToken(context.Context) (string, error) { return "", nil }
func (f *fakeAuthManager) IsAuthenticated() bool                    { return false }
func (f *fakeAuthManager) ClearTokens() error                       { return nil }
func (f *fakeAuthManager) InitiateAuth(context.Context) (string, error) {
	return "https://idp.example.com/authorize", f.initiateErr
}
func (f *fakeAuthManager) HandleCallback(context.Context, string, string) error {
	return f.callbackErr
}

func TestOAuthRoutes_Pages(t *testing.T) {
	tests := []struct {
		name       string
		authMgr    *fakeAuthManager
		request    func() *http.Request
		wantStatus int
		want       []string
		notWant    []string
	}{
		{
			name:       "signed in",
			authMgr:    &fakeAuthManager{},
			request:    func() *http.Request { return httptest.NewRequest(http.MethodGet, "/callback?code=c&state=s", nil) },
			wantStatus: http.StatusOK,
			want:       []string{"<h1>Signed in</h1>", "Giant Swarm search is now connected", "return to where you started the sign-in"},
		},
		{
			name:    "device signed in",
			authMgr: &fakeAuthManager{},
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/callback?code=c&state=s", nil)
				r.AddCookie(&http.Cookie{Name: deviceUserCodeCookie, Value: "<b>WDJB-MJHT</b>", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
				return r
			},
			wantStatus: http.StatusOK,
			want:       []string{"<h1>Signed in</h1>", "Your device is now connected", "&lt;b&gt;WDJB-MJHT&lt;/b&gt;", "return to your device"},
			notWant:    []string{"<b>WDJB"},
		},
		{
			name:       "callback without a code",
			authMgr:    &fakeAuthManager{},
			request:    func() *http.Request { return httptest.NewRequest(http.MethodGet, "/callback?state=s", nil) },
			wantStatus: http.StatusBadRequest,
			want:       []string{signInFailedHeading, "The sign-in response is incomplete"},
		},
		{
			name:       "callback fails",
			authMgr:    &fakeAuthManager{callbackErr: errors.New("token endpoint said no: internal detail")},
			request:    func() *http.Request { return httptest.NewRequest(http.MethodGet, "/callback?code=c&state=s", nil) },
			wantStatus: http.StatusInternalServerError,
			want:       []string{signInFailedHeading, "The sign-in could not be completed"},
			notWant:    []string{"internal detail"},
		},
		{
			name:       "login cannot start",
			authMgr:    &fakeAuthManager{initiateErr: errors.New("discovery failed: internal detail")},
			request:    func() *http.Request { return httptest.NewRequest(http.MethodGet, "/oauth/login", nil) },
			wantStatus: http.StatusInternalServerError,
			want:       []string{signInFailedHeading, "The sign-in could not be started"},
			notWant:    []string{"internal detail"},
		},
		{
			name:    "device code missing",
			authMgr: &fakeAuthManager{},
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/device", strings.NewReader(url.Values{"user_code": {""}}.Encode()))
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return r
			},
			wantStatus: http.StatusBadRequest,
			want:       []string{signInFailedHeading, "No code was entered", "enter the code shown on your device"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{authMgr: tc.authMgr, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			mux := http.NewServeMux()
			s.registerOAuthRoutes(mux)
			rr := httptest.NewRecorder()

			mux.ServeHTTP(rr, tc.request())

			if rr.Code != tc.wantStatus {
				t.Errorf("expected status %d, got %d", tc.wantStatus, rr.Code)
			}
			if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("expected an HTML page, got Content-Type %q", got)
			}
			if !strings.Contains(rr.Header().Get("Content-Security-Policy"), "default-src 'none'") {
				t.Errorf("expected a strict Content-Security-Policy, got %q", rr.Header().Get("Content-Security-Policy"))
			}
			body := rr.Body.String()
			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Errorf("expected body to contain %q, got %q", want, body)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(body, notWant) {
					t.Errorf("expected body not to contain %q", notWant)
				}
			}
		})
	}
}
