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

	"github.com/giantswarm/search-mcp/internal/auth"
)

const (
	signInFailedHeading = "<h1>Sign-in failed</h1>"
	enterCodeAgain      = "enter the code shown on your device"
	// internalDetail is part of internal errors and must not reach a page.
	internalDetail = "internal detail"
)

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

// fakeDeviceAuthManager also looks up device flows, as auth.Manager does.
type fakeDeviceAuthManager struct {
	fakeAuthManager
	deviceErr error
}

func (f *fakeDeviceAuthManager) GetDeviceByUserCode(userCode string) (*auth.DeviceCodeData, error) {
	if f.deviceErr != nil {
		return nil, f.deviceErr
	}
	return &auth.DeviceCodeData{UserCode: userCode, DeviceCode: "device-code"}, nil
}

func deviceForm(code string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/device", strings.NewReader(url.Values{"user_code": {code}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func withDeviceCookie(r *http.Request) *http.Request {
	r.AddCookie(&http.Cookie{Name: deviceUserCodeCookie, Value: "WDJB-MJHT", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return r
}

func TestOAuthRoutes_Pages(t *testing.T) {
	tests := []struct {
		name       string
		authMgr    auth.AuthManager
		request    func() *http.Request
		wantStatus int
		want       []string
		notWant    []string
		// wantCookieCleared asks for the device cookie to be cleared.
		wantCookieCleared bool
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
			wantStatus:        http.StatusOK,
			want:              []string{"<h1>Signed in</h1>", "Your device is now connected", "&lt;b&gt;WDJB-MJHT&lt;/b&gt;", "return to your device"},
			notWant:           []string{"<b>WDJB"},
			wantCookieCleared: true,
		},
		{
			name:    "provider refuses",
			authMgr: &fakeAuthManager{},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/callback?error=access_denied&state=s", nil)
			},
			wantStatus: http.StatusBadRequest,
			want:       []string{signInFailedHeading, "The sign-in was denied or failed", "<title>Sign-in to Giant Swarm search failed</title>"},
			notWant:    []string{"incomplete"},
		},
		{
			name:    "device sign-in fails at the callback",
			authMgr: &fakeAuthManager{callbackErr: errors.New("token endpoint said no")},
			request: func() *http.Request {
				return withDeviceCookie(httptest.NewRequest(http.MethodGet, "/callback?code=c&state=s", nil))
			},
			wantStatus:        http.StatusInternalServerError,
			want:              []string{signInFailedHeading, "The sign-in could not be completed", enterCodeAgain},
			wantCookieCleared: true,
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
			authMgr:    &fakeAuthManager{callbackErr: errors.New("token endpoint said no: " + internalDetail)},
			request:    func() *http.Request { return httptest.NewRequest(http.MethodGet, "/callback?code=c&state=s", nil) },
			wantStatus: http.StatusInternalServerError,
			want:       []string{signInFailedHeading, "The sign-in could not be completed"},
			notWant:    []string{internalDetail},
		},
		{
			name:       "login cannot start",
			authMgr:    &fakeAuthManager{initiateErr: errors.New("discovery failed: " + internalDetail)},
			request:    func() *http.Request { return httptest.NewRequest(http.MethodGet, "/oauth/login", nil) },
			wantStatus: http.StatusInternalServerError,
			want:       []string{signInFailedHeading, "The sign-in could not be started"},
			notWant:    []string{internalDetail},
		},
		{
			name:       "device code missing",
			authMgr:    &fakeDeviceAuthManager{},
			request:    func() *http.Request { return deviceForm("") },
			wantStatus: http.StatusBadRequest,
			want:       []string{signInFailedHeading, "No code was entered", enterCodeAgain},
		},
		{
			name:       "device code invalid",
			authMgr:    &fakeDeviceAuthManager{deviceErr: errors.New("not found")},
			request:    func() *http.Request { return deviceForm("WDJB-MJHT") },
			wantStatus: http.StatusBadRequest,
			want:       []string{signInFailedHeading, "The code is invalid or has expired", enterCodeAgain},
		},
		{
			name:       "device sign-in cannot start",
			authMgr:    &fakeDeviceAuthManager{fakeAuthManager: fakeAuthManager{initiateErr: errors.New("discovery failed: " + internalDetail)}},
			request:    func() *http.Request { return deviceForm("WDJB-MJHT") },
			wantStatus: http.StatusInternalServerError,
			want:       []string{signInFailedHeading, "The sign-in could not be started", "Try again in a moment"},
			notWant:    []string{internalDetail},
		},
		{
			name:       "device sign-in not available",
			authMgr:    &fakeAuthManager{},
			request:    func() *http.Request { return deviceForm("WDJB-MJHT") },
			wantStatus: http.StatusInternalServerError,
			want:       []string{signInFailedHeading, "The device sign-in is not available", "problem on our side"},
			notWant:    []string{"Try again"},
		},
		{
			name:       "device method not allowed",
			authMgr:    &fakeDeviceAuthManager{},
			request:    func() *http.Request { return httptest.NewRequest(http.MethodPut, "/device", nil) },
			wantStatus: http.StatusMethodNotAllowed,
			want:       []string{"This page only accepts the device code form"},
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
			if tc.wantCookieCleared && !clearsDeviceCookie(rr.Result().Cookies()) {
				t.Errorf("expected the device cookie to be cleared, got Set-Cookie %q", rr.Header().Values("Set-Cookie"))
			}
		})
	}
}

func clearsDeviceCookie(cookies []*http.Cookie) bool {
	for _, c := range cookies {
		if c.Name == deviceUserCodeCookie && c.MaxAge < 0 {
			return true
		}
	}
	return false
}

// TestDeviceAuthorize_StartsSignIn covers a valid code: the code is kept in a
// cookie for the callback and the browser goes to the identity provider.
func TestDeviceAuthorize_StartsSignIn(t *testing.T) {
	s := &Server{authMgr: &fakeDeviceAuthManager{}, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	mux := http.NewServeMux()
	s.registerOAuthRoutes(mux)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, deviceForm("WDJB-MJHT"))

	if rr.Code != http.StatusTemporaryRedirect || rr.Header().Get("Location") != "https://idp.example.com/authorize" {
		t.Errorf("expected a redirect to the identity provider, got %d to %q", rr.Code, rr.Header().Get("Location"))
	}
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == deviceUserCodeCookie && c.Value == "WDJB-MJHT" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the user code in the device cookie, got Set-Cookie %q", rr.Header().Values("Set-Cookie"))
	}
}
