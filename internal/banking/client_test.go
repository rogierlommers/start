package banking

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnableBankingClientReadOnlyFlow(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	keyPath := t.TempDir() + "/private.pem"
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: mustPKCS8(t, privateKey)})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertJWT(t, r, privateKey.PublicKey, "app-id")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode auth request: %v", err)
			}
			access := body["access"].(map[string]any)
			if access["balances"] != true {
				t.Fatalf("access = %#v, want balances", access)
			}
			if _, found := access["transactions"]; found {
				t.Fatalf("access = %#v, must not request unused transaction access", access)
			}
			_, _ = w.Write([]byte(`{"url":"https://auth.enablebanking.test/start"}`))
		case "/sessions":
			_, _ = w.Write([]byte(`{"session_id":"session-1","accounts":[{"uid":"account-1","name":"Current account","currency":"EUR"},{"uid":"account-2","product":"Savings account","currency":"EUR"}],"access":{"valid_until":"2027-01-01T00:00:00Z"}}`))
		case "/sessions/session-1":
			_, _ = w.Write([]byte(`{"accounts":["account-1","account-2"],"accounts_data":[{"uid":"account-1"},{"uid":"account-2"}],"access":{"valid_until":"2027-01-01T00:00:00Z"}}`))
		case "/accounts/account-2/details":
			_, _ = w.Write([]byte(`{"uid":"account-2","product":"Savings account","currency":"EUR"}`))
		case "/accounts/account-1/balances":
			_, _ = w.Write([]byte(`{"balances":[{"name":"Available balance","balance_type":"CLAV","balance_amount":{"amount":"123.45","currency":"EUR"},"last_change_date_time":"2026-10-10T09:00:00Z"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewEnableBankingClient("app-id", keyPath, server.Client())
	if err != nil {
		t.Fatalf("NewEnableBankingClient() error = %v", err)
	}
	client.baseURL = server.URL

	redirectURL, err := client.StartAuthorization(context.Background(), AuthorizationRequest{
		State: "state", RedirectURL: "https://dashboard.example/api/banking/callback",
		ASPSPName: "ING", Country: "NL", ValidUntil: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil || redirectURL != "https://auth.enablebanking.test/start" {
		t.Fatalf("StartAuthorization() = (%q, %v)", redirectURL, err)
	}
	session, err := client.AuthorizeSession(context.Background(), "code")
	if err != nil || len(session.Accounts) != 2 || session.Accounts[0].ID != "account-1" || session.Accounts[1].ID != "account-2" {
		t.Fatalf("AuthorizeSession() = (%+v, %v)", session, err)
	}
	recovered, err := client.GetSession(context.Background(), session.ID)
	if err != nil || len(recovered.Accounts) != 2 || recovered.Accounts[1].ID != "account-2" {
		t.Fatalf("GetSession() = (%+v, %v)", recovered, err)
	}
	details, err := client.GetAccountDetails(context.Background(), recovered.Accounts[1].ID)
	if err != nil || details.Product != "Savings account" {
		t.Fatalf("GetAccountDetails() = (%+v, %v)", details, err)
	}
	balances, err := client.GetBalances(context.Background(), session.Accounts[0].ID)
	if err != nil || len(balances) != 1 || balances[0].Amount != "123.45" {
		t.Fatalf("GetBalances() = (%+v, %v)", balances, err)
	}
}

func mustPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	return encoded
}

func assertJWT(t *testing.T, r *http.Request, publicKey rsa.PublicKey, appID string) {
	t.Helper()
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("Authorization does not contain a JWT: %q", r.Header.Get("Authorization"))
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode JWT header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerJSON, &header); err != nil || header["kid"] != appID || header["alg"] != "RS256" {
		t.Fatalf("JWT header = %#v, error = %v", header, err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || rsa.VerifyPKCS1v15(&publicKey, crypto.SHA256, digest[:], signature) != nil {
		t.Fatal("JWT signature is invalid")
	}
}
