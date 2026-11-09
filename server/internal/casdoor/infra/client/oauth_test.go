package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOAuthCachesConcurrentTokenRequestsAndUsesBearer(t *testing.T) {
	var minted atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/login/oauth/access_token" {
			minted.Add(1)
			fmt.Fprint(w, `{"access_token":"fixture-token","token_type":"Bearer","expires_in":3600}`)
			return
		}
		require.Equal(t, "Bearer fixture-token", r.Header.Get("Authorization"))
		if r.URL.Query().Get("p") == "2" {
			fmt.Fprint(w, `{"status":"ok","data":[]}`)
			return
		}
		fmt.Fprint(w, `{"status":"ok","data":[{"owner":"admin","name":"built-in","displayName":"Built-in"}]}`)
	}))
	defer server.Close()
	oauth := NewOAuthClient(Config{ServerURL: server.URL, ClientID: "client", ClientSecret: "secret", TLSVerify: true})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := oauth.Organizations(context.Background())
			require.NoError(t, err)
			require.Len(t, rows, 1)
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, minted.Load())
	oauth.mu.Lock()
	oauth.expiresAt = time.Now().Add(-time.Second)
	oauth.mu.Unlock()
	_, err := oauth.Organizations(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, minted.Load())
}

func TestOAuthManagementRejectsBusinessOrganizationCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/login/oauth/access_token" {
			fmt.Fprint(w, `{"access_token":"token","token_type":"Bearer","expires_in":600}`)
			return
		}
		fmt.Fprint(w, `{"status":"ok","data":[{"owner":"admin","name":"business-app","organization":"org","clientId":"client"}]}`)
	}))
	defer server.Close()
	_, err := NewOAuthClient(Config{ServerURL: server.URL, ClientID: "client"}).ValidateManagement(context.Background())
	require.Error(t, err)
}

func TestOAuthRejectsExpiredAndMalformedTokensWithoutLeakingSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"error":"invalid_client","error_description":"secret-value"}`)
	}))
	defer server.Close()
	_, err := NewOAuthClient(Config{ServerURL: server.URL, ClientSecret: "secret-value", Timeout: time.Second}).Organizations(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret-value")
}

func TestOAuthOrganizationsStopsAtDeclaredTotalWithClampedLastPage(t *testing.T) {
	var pages atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/login/oauth/access_token" {
			fmt.Fprint(w, `{"access_token":"token","token_type":"Bearer","expires_in":3600}`)
			return
		}
		pages.Add(1)
		fmt.Fprint(w, `{"status":"ok","data":[{"owner":"admin","name":"built-in"}],"data2":1}`)
	}))
	defer server.Close()
	rows, err := NewOAuthClient(Config{ServerURL: server.URL}).Organizations(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 1, pages.Load())
}
