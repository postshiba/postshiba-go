package postshiba

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	teamID           = "1"
	clusterID        = "4"
	sendingDomainID  = "8"
	tenantID         = "12"
	inboxID          = "3"
	messageID        = "21"
	eventID          = "44"
	smtpCredentialID = "9"
	suppressionID    = "7"
	firewallEntryID  = "3"
	webhookID        = "2"
)

func fixturePath(name string) string {
	return filepath.Join("..", "..", "fixtures", "catalog", name+".json")
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixturePath(name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func loadJSON(t *testing.T, name string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(loadFixture(t, name), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func asMap(t *testing.T, name string) map[string]any {
	t.Helper()
	v, ok := loadJSON(t, name).(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object", name)
	}
	return v
}

func equalJSON(t *testing.T, got any, wantName string) {
	t.Helper()
	want := loadJSON(t, wantName)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func equalJSONList(t *testing.T, got any, wantName string) {
	t.Helper()
	want := []any{loadJSON(t, wantName)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

type route struct {
	status  int
	body    []byte
	list    bool
	fixture string
}

type captured struct {
	req  *http.Request
	body []byte
}

func newMux(t *testing.T, routes map[string]route) (*httptest.Server, *captured) {
	t.Helper()
	cap := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		cap.req = r.Clone(r.Context())
		cap.body = body
		key := r.Method + " " + r.URL.Path
		rt, ok := routes[key]
		if !ok {
			t.Errorf("unexpected %s", key)
			http.NotFound(w, r)
			return
		}
		if rt.status != 0 {
			w.WriteHeader(rt.status)
		}
		if rt.body != nil {
			_, _ = w.Write(rt.body)
			return
		}
		if rt.fixture == "" {
			return
		}
		data := loadFixture(t, rt.fixture)
		if rt.list {
			data = append(append([]byte("["), data...), ']')
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, cap
}

func clientFor(srv *httptest.Server) *Client {
	return NewClient("test-key", WithBaseURL(srv.URL), WithTeamID(teamID))
}

func TestBearerAndBaseURL(t *testing.T) {
	var gotAuth, gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotHost = r.Host
		_, _ = w.Write(loadFixture(t, "whoami"))
	}))
	t.Cleanup(srv.Close)

	c := NewClient("test-key", WithBaseURL(srv.URL))
	if _, err := c.UsersMe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(srv.URL, gotHost) {
		t.Fatalf("host %q not in %s", gotHost, srv.URL)
	}
}

func TestEmailsSend(t *testing.T) {
	srv, cap := newMux(t, map[string]route{
		"POST /api/v1/emails": {fixture: "email_send_response"},
	})
	c := clientFor(srv)
	got, err := c.EmailsSend(context.Background(), asMap(t, "email_send_request"))
	if err != nil {
		t.Fatal(err)
	}
	equalJSON(t, got, "email_send_response")
	var sent any
	if err := json.Unmarshal(cap.body, &sent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sent, loadJSON(t, "email_send_request")) {
		t.Fatalf("body = %#v", sent)
	}
}

func TestEmailsSendOnClusterIdempotencyAndSandbox(t *testing.T) {
	var gotKey string
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/teams/1/clusters/4/sends" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		gotKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(loadFixture(t, "email_sandbox_response"))
	}))
	t.Cleanup(srv.Close)

	body := asMap(t, "email_send_request")
	c := clientFor(srv)
	got, err := c.EmailsSendOnCluster(context.Background(), clusterID, body, &SendOnClusterOptions{
		IdempotencyKey: "idem-1",
		Sandbox:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	equalJSON(t, got, "email_sandbox_response")
	if gotKey != "idem-1" {
		t.Fatalf("Idempotency-Key = %q", gotKey)
	}
	if sent["sandbox"] != true {
		t.Fatalf("sandbox = %#v", sent["sandbox"])
	}
	if _, ok := body["sandbox"]; ok {
		t.Fatal("caller body was mutated")
	}
	queued, _ := got.(map[string]any)["queued"].(bool)
	if queued {
		t.Fatal("sandbox response queued should be false")
	}
}

func TestEveryMethod(t *testing.T) {
	ctx := context.Background()
	routes := map[string]route{
		"GET /api/v1/users/me":                                    {fixture: "whoami"},
		"POST /api/v1/emails":                                     {fixture: "email_send_response"},
		"POST /api/v1/teams/1/clusters/4/sends":                   {fixture: "email_sandbox_response"},
		"GET /api/v1/teams/1/clusters":                            {fixture: "cluster", list: true},
		"GET /api/v1/clusters/4":                                  {fixture: "cluster"},
		"POST /api/v1/teams/1/clusters":                           {fixture: "cluster"},
		"PATCH /api/v1/clusters/4":                                {fixture: "cluster_updated"},
		"POST /api/v1/clusters/4/suspend":                         {fixture: "cluster_suspended"},
		"POST /api/v1/clusters/4/resume":                          {fixture: "cluster"},
		"DELETE /api/v1/clusters/4":                               {fixture: "cluster_deprovisioned"},
		"GET /api/v1/teams/1/sending_domains":                     {fixture: "sending_domain", list: true},
		"GET /api/v1/sending_domains/8":                           {fixture: "sending_domain"},
		"POST /api/v1/teams/1/sending_domains":                    {fixture: "sending_domain"},
		"POST /api/v1/sending_domains/8/verify":                   {fixture: "sending_domain"},
		"POST /api/v1/sending_domains/8/suspend":                  {fixture: "sending_domain_suspended"},
		"POST /api/v1/sending_domains/8/resume":                   {fixture: "sending_domain"},
		"POST /api/v1/sending_domains/8/make_primary":             {fixture: "sending_domain_primary"},
		"DELETE /api/v1/sending_domains/8":                        {fixture: "empty"},
		"GET /api/v1/teams/1/tenants":                             {fixture: "tenant", list: true},
		"GET /api/v1/tenants/12":                                  {fixture: "tenant"},
		"POST /api/v1/teams/1/tenants":                            {fixture: "tenant"},
		"DELETE /api/v1/tenants/12":                               {fixture: "empty"},
		"GET /api/v1/teams/1/inboxes":                             {fixture: "inbox_index", list: true},
		"GET /api/v1/inboxes/3":                                   {fixture: "inbox"},
		"POST /api/v1/teams/1/inboxes":                            {fixture: "inbox"},
		"POST /api/v1/inboxes/3/verify":                           {fixture: "inbox_index"},
		"DELETE /api/v1/inboxes/3":                                {fixture: "inbox_index"},
		"GET /api/v1/inboxes/3/inbound_messages":                  {fixture: "message", list: true},
		"GET /api/v1/inboxes/3/inbound_messages/21":               {fixture: "message_show"},
		"GET /api/v1/inboxes/3/inbound_messages/21/attachments/1": {body: []byte("png")},
		"GET /api/v1/teams/1/clusters/4/message_events":           {fixture: "event", list: true},
		"GET /api/v1/message_events/44":                           {fixture: "event"},
		"POST /api/v1/teams/1/clusters/4/smtp_credentials":        {fixture: "smtp_credential_create"},
		"DELETE /api/v1/teams/1/clusters/4/smtp_credentials/9":    {fixture: "smtp_credential_deleted"},
		"GET /api/v1/teams/1/webhook_endpoints":                   {fixture: "webhook", list: true},
		"GET /api/v1/webhook_endpoints/2":                         {fixture: "webhook_show"},
		"POST /api/v1/teams/1/webhook_endpoints":                  {fixture: "webhook_show"},
		"GET /api/v1/teams/1/suppressions":                        {fixture: "suppression", list: true},
		"POST /api/v1/teams/1/suppressions":                       {fixture: "suppression"},
		"DELETE /api/v1/suppressions/7":                           {fixture: "empty"},
		"GET /api/v1/teams/1/firewall":                            {fixture: "firewall"},
		"PATCH /api/v1/teams/1/firewall":                          {fixture: "firewall"},
		"POST /api/v1/teams/1/firewall_entries":                   {fixture: "firewall_entry"},
		"DELETE /api/v1/firewall_entries/3":                       {fixture: "empty"},
	}
	srv, cap := newMux(t, routes)
	c := clientFor(srv)

	cases := []struct {
		name string
		call func() (any, error)
		want string
		list bool
	}{
		{"users.me", func() (any, error) { return c.UsersMe(ctx) }, "whoami", false},
		{"emails.send", func() (any, error) { return c.EmailsSend(ctx, asMap(t, "email_send_request")) }, "email_send_response", false},
		{"emails.sendOnCluster", func() (any, error) {
			return c.EmailsSendOnCluster(ctx, clusterID, asMap(t, "email_send_request"), nil)
		}, "email_sandbox_response", false},
		{"clusters.list", func() (any, error) { return c.ClustersList(ctx) }, "cluster", true},
		{"clusters.get", func() (any, error) { return c.ClustersGet(ctx, clusterID) }, "cluster", false},
		{"clusters.create", func() (any, error) { return c.ClustersCreate(ctx, asMap(t, "cluster_create_request")) }, "cluster", false},
		{"clusters.update", func() (any, error) {
			return c.ClustersUpdate(ctx, clusterID, asMap(t, "cluster_update_request"))
		}, "cluster_updated", false},
		{"clusters.suspend", func() (any, error) { return c.ClustersSuspend(ctx, clusterID) }, "cluster_suspended", false},
		{"clusters.resume", func() (any, error) { return c.ClustersResume(ctx, clusterID) }, "cluster", false},
		{"clusters.delete", func() (any, error) { return c.ClustersDelete(ctx, clusterID) }, "cluster_deprovisioned", false},
		{"sendingDomains.list", func() (any, error) { return c.SendingDomainsList(ctx) }, "sending_domain", true},
		{"sendingDomains.get", func() (any, error) { return c.SendingDomainsGet(ctx, sendingDomainID) }, "sending_domain", false},
		{"sendingDomains.create", func() (any, error) {
			return c.SendingDomainsCreate(ctx, asMap(t, "sending_domain_create_request"))
		}, "sending_domain", false},
		{"sendingDomains.verify", func() (any, error) { return c.SendingDomainsVerify(ctx, sendingDomainID) }, "sending_domain", false},
		{"sendingDomains.suspend", func() (any, error) {
			return c.SendingDomainsSuspend(ctx, sendingDomainID)
		}, "sending_domain_suspended", false},
		{"sendingDomains.resume", func() (any, error) { return c.SendingDomainsResume(ctx, sendingDomainID) }, "sending_domain", false},
		{"sendingDomains.makePrimary", func() (any, error) {
			return c.SendingDomainsMakePrimary(ctx, sendingDomainID)
		}, "sending_domain_primary", false},
		{"sendingDomains.delete", func() (any, error) { return c.SendingDomainsDelete(ctx, sendingDomainID) }, "empty", false},
		{"tenants.list", func() (any, error) { return c.TenantsList(ctx) }, "tenant", true},
		{"tenants.get", func() (any, error) { return c.TenantsGet(ctx, tenantID) }, "tenant", false},
		{"tenants.create", func() (any, error) { return c.TenantsCreate(ctx, asMap(t, "tenant_create_request")) }, "tenant", false},
		{"tenants.delete", func() (any, error) { return c.TenantsDelete(ctx, tenantID) }, "empty", false},
		{"inboxes.list", func() (any, error) { return c.InboxesList(ctx) }, "inbox_index", true},
		{"inboxes.get", func() (any, error) { return c.InboxesGet(ctx, inboxID) }, "inbox", false},
		{"inboxes.create", func() (any, error) { return c.InboxesCreate(ctx, asMap(t, "inbox_create_request")) }, "inbox", false},
		{"inboxes.verify", func() (any, error) { return c.InboxesVerify(ctx, inboxID) }, "inbox_index", false},
		{"inboxes.delete", func() (any, error) { return c.InboxesDelete(ctx, inboxID) }, "inbox_index", false},
		{"messages.list", func() (any, error) { return c.MessagesList(ctx, inboxID) }, "message", true},
		{"messages.get", func() (any, error) { return c.MessagesGet(ctx, inboxID, messageID) }, "message_show", false},
		{"events.list", func() (any, error) { return c.EventsList(ctx, clusterID) }, "event", true},
		{"events.get", func() (any, error) { return c.EventsGet(ctx, eventID) }, "event", false},
		{"smtpCredentials.create", func() (any, error) {
			return c.SMTPCredentialsCreate(ctx, clusterID, asMap(t, "smtp_credential_create_request"))
		}, "smtp_credential_create", false},
		{"smtpCredentials.delete", func() (any, error) {
			return c.SMTPCredentialsDelete(ctx, clusterID, smtpCredentialID)
		}, "smtp_credential_deleted", false},
		{"webhooks.list", func() (any, error) { return c.WebhooksList(ctx) }, "webhook", true},
		{"webhooks.get", func() (any, error) { return c.WebhooksGet(ctx, webhookID) }, "webhook_show", false},
		{"webhooks.create", func() (any, error) {
			return c.WebhooksCreate(ctx, asMap(t, "webhook_create_request"))
		}, "webhook_show", false},
		{"suppressions.list", func() (any, error) { return c.SuppressionsList(ctx) }, "suppression", true},
		{"suppressions.create", func() (any, error) {
			return c.SuppressionsCreate(ctx, asMap(t, "suppression_create_request"))
		}, "suppression", false},
		{"suppressions.delete", func() (any, error) { return c.SuppressionsDelete(ctx, suppressionID) }, "empty", false},
		{"firewall.get", func() (any, error) { return c.FirewallGet(ctx) }, "firewall", false},
		{"firewall.update", func() (any, error) {
			return c.FirewallUpdate(ctx, asMap(t, "firewall_update_request"))
		}, "firewall", false},
		{"firewall.addEntry", func() (any, error) {
			return c.FirewallAddEntry(ctx, asMap(t, "firewall_entry_create_request"))
		}, "firewall_entry", false},
		{"firewall.deleteEntry", func() (any, error) { return c.FirewallDeleteEntry(ctx, firewallEntryID) }, "empty", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.call()
			if err != nil {
				t.Fatal(err)
			}
			if tc.list {
				equalJSONList(t, got, tc.want)
				return
			}
			equalJSON(t, got, tc.want)
		})
	}

	raw, err := c.MessagesDownloadAttachment(ctx, inboxID, messageID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "png" {
		t.Fatalf("attachment = %q", raw)
	}
	if cap.req.URL.Path != "/api/v1/inboxes/3/inbound_messages/21/attachments/1" {
		t.Fatalf("attachment path = %s", cap.req.URL.Path)
	}
}

func TestAPIErrors(t *testing.T) {
	cases := []struct {
		status  int
		fixture string
		code    string
		field   string
	}{
		{403, "error_403", "cluster_not_ready", "cluster"},
		{422, "error_422", "invalid", "from"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			srv, _ := newMux(t, map[string]route{
				"GET /api/v1/teams/1/clusters": {status: tc.status, fixture: tc.fixture},
			})
			_, err := clientFor(srv).ClustersList(context.Background())
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v", err)
			}
			if apiErr.Err != tc.code || apiErr.Field != tc.field {
				t.Fatalf("%+v", apiErr)
			}
			if apiErr.Message == "" {
				t.Fatal("missing message")
			}
		})
	}
}

func TestVerifyWebhook(t *testing.T) {
	fix := asMap(t, "webhook_verify")
	secret := fix["secret"].(string)
	timestamp := fix["timestamp"].(string)
	body := []byte(fix["body"].(string))
	signature := fix["signature"].(string)

	if err := VerifyWebhook(secret, timestamp, signature, body); err != nil {
		t.Fatal(err)
	}
	if err := VerifyWebhook(secret, timestamp, strings.TrimPrefix(signature, "sha256="), body); err != nil {
		t.Fatal(err)
	}
	if err := VerifyWebhook(secret, timestamp, signature, []byte("nope")); err == nil {
		t.Fatal("expected reject")
	}
}

func TestSMTPPasswordCreateAndDelete(t *testing.T) {
	ctx := context.Background()
	srv, _ := newMux(t, map[string]route{
		"POST /api/v1/teams/1/clusters/4/smtp_credentials":     {fixture: "smtp_credential_create"},
		"DELETE /api/v1/teams/1/clusters/4/smtp_credentials/9": {fixture: "smtp_credential_deleted"},
	})
	c := clientFor(srv)
	created, err := c.SMTPCredentialsCreate(ctx, clusterID, asMap(t, "smtp_credential_create_request"))
	if err != nil {
		t.Fatal(err)
	}
	if created.(map[string]any)["password"] != "once-only-password" {
		t.Fatalf("create password = %#v", created)
	}
	deleted, err := c.SMTPCredentialsDelete(ctx, clusterID, smtpCredentialID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := deleted.(map[string]any)["password"]; ok {
		t.Fatal("delete leaked password")
	}
}

func TestWebhookSecretListGetCreate(t *testing.T) {
	ctx := context.Background()
	srv, _ := newMux(t, map[string]route{
		"GET /api/v1/teams/1/webhook_endpoints":  {fixture: "webhook", list: true},
		"GET /api/v1/webhook_endpoints/2":        {fixture: "webhook_show"},
		"POST /api/v1/teams/1/webhook_endpoints": {fixture: "webhook_show"},
	})
	c := clientFor(srv)
	list, err := c.WebhooksList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	items := list.([]any)
	if _, ok := items[0].(map[string]any)["secret"]; ok {
		t.Fatal("list leaked secret")
	}
	show, err := c.WebhooksGet(ctx, webhookID)
	if err != nil {
		t.Fatal(err)
	}
	if show.(map[string]any)["secret"] != "hex-secret" {
		t.Fatalf("get secret = %#v", show)
	}
	created, err := c.WebhooksCreate(ctx, asMap(t, "webhook_create_request"))
	if err != nil {
		t.Fatal(err)
	}
	if created.(map[string]any)["secret"] != "hex-secret" {
		t.Fatalf("create secret = %#v", created)
	}
}

func TestMissingTeamID(t *testing.T) {
	c := NewClient("test-key", WithBaseURL("http://127.0.0.1:1"))
	_, err := c.ClustersList(context.Background())
	if err == nil || !strings.Contains(err.Error(), "team id") {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultBaseURL(t *testing.T) {
	c := NewClient("key")
	if c.baseURL != "https://app.postshiba.com" {
		t.Fatalf("baseURL = %q", c.baseURL)
	}
}
