package fieldkey

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecideGraceKeepsOverlapForActiveDeployments(t *testing.T) {
	cases := []struct {
		name        string
		deployments int
		wantHours   int
		wantAction  string
	}{
		{name: "deployments still have old value", deployments: 2, wantHours: 24, wantAction: "keep old key active while deployments update"},
		{name: "deployment scan is clear", deployments: 0, wantHours: 1, wantAction: "finish rotation after final deployment check"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideGrace(tc.deployments)
			if got.GraceHours != tc.wantHours || got.Action != tc.wantAction {
				t.Fatalf("DecideGrace(%d) = %#v", tc.deployments, got)
			}
		})
	}
}

func TestRotateForWorkOrderRevokesTemporaryKey(t *testing.T) {
	for _, failSearch := range []bool{false, true} {
		t.Run(fmt.Sprintf("searchFailure=%t", failSearch), func(t *testing.T) {
			var revoked, rotatedRevoked bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v1/account/keys/create":
					fmt.Fprint(w, `{"ok":true,"data":{"key_id":"temporary-id"}}`)
				case "/v1/account/keys/rotate/temporary-id":
					fmt.Fprint(w, `{"ok":true,"data":{"key_id":"masked-id","key":"ifr_abcdef2e99"}}`)
				case "/v1/logs/search":
					if r.URL.Query().Get("q") != "deployment old API value work-order WO-1042" || r.URL.Query().Has("query") {
						t.Errorf("invalid log search fields: %s", r.URL.RawQuery)
					}
					if failSearch {
						fmt.Fprint(w, `{"ok":false,"error":{"code":"SEARCH_FAILED","message":"unavailable"}}`)
					} else {
						fmt.Fprint(w, `{"ok":true,"data":[]}`)
					}
				case "/v1/account/keys/revoke/temporary-id":
					revoked = true
					fmt.Fprint(w, `{"ok":true,"data":{}}`)
				case "/v1/account/keys/revoke/ifr_...2e99":
					rotatedRevoked = true
					fmt.Fprint(w, `{"ok":true,"data":{}}`)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
			}))
			defer server.Close()
			client := &Client{baseURL: server.URL, key: "test", http: server.Client()}
			_, err := RotateForWorkOrder(context.Background(), client, WorkOrder{ID: "WO-1042"}, 2)
			if (err != nil) != failSearch {
				t.Errorf("error = %v, want failure %t", err, failSearch)
			}
			if !revoked || !rotatedRevoked {
				t.Errorf("keys not revoked: temporary=%t rotated=%t", revoked, rotatedRevoked)
			}
		})
	}
}
