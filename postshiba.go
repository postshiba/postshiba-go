// Package postshiba is a client for the PostShiba API.
// Version 0.1.0
package postshiba

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const defaultBaseURL = "https://app.postshiba.com"

// Client calls the PostShiba HTTP API.
type Client struct {
	apiKey  string
	baseURL string
	teamID  string
	http    *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the default API host.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithTeamID sets the team used for team-scoped paths.
func WithTeamID(teamID string) Option {
	return func(c *Client) {
		c.teamID = teamID
	}
}

// NewClient builds a client. TeamID is required for team-scoped paths.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Error is a non-2xx API response.
// Err is the JSON "error" key. Go reserves Error() for the error interface.
type Error struct {
	Err     string `json:"error"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err
}

// SendOptions pins emails.send to a cluster via X-Capsule-Cluster-Id.
type SendOptions struct {
	ClusterID string
}

// SendOnClusterOptions adds Idempotency-Key and sandbox to a cluster send.
type SendOnClusterOptions struct {
	IdempotencyKey string
	Sandbox        bool
}

func (c *Client) requireTeamID() (string, error) {
	if c.teamID == "" {
		return "", errors.New("team id is required")
	}
	return c.teamID, nil
}

func (c *Client) teamsPath(suffix string) (string, error) {
	teamID, err := c.requireTeamID()
	if err != nil {
		return "", err
	}
	return "/api/v1/teams/" + teamID + suffix, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, extra http.Header) (any, error) {
	raw, err := c.doRaw(ctx, method, path, body, extra)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) doRaw(ctx context.Context, method, path string, body any, extra http.Header) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &Error{}
		_ = json.Unmarshal(raw, apiErr)
		return nil, apiErr
	}
	return raw, nil
}

func (c *Client) UsersMe(ctx context.Context) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/users/me", nil, nil)
}

func (c *Client) EmailsSend(ctx context.Context, body any, opts ...*SendOptions) (any, error) {
	var extra http.Header
	if len(opts) > 0 && opts[0] != nil && opts[0].ClusterID != "" {
		extra = http.Header{"X-Capsule-Cluster-Id": {opts[0].ClusterID}}
	}
	return c.do(ctx, http.MethodPost, "/api/v1/emails", body, extra)
}

func (c *Client) EmailsSendOnCluster(ctx context.Context, clusterID string, body map[string]any, opts *SendOnClusterOptions) (any, error) {
	path, err := c.teamsPath("/clusters/" + clusterID + "/sends")
	if err != nil {
		return nil, err
	}
	payload := body
	var extra http.Header
	if opts != nil {
		if opts.Sandbox {
			payload = cloneMap(body)
			payload["sandbox"] = true
		}
		if opts.IdempotencyKey != "" {
			extra = http.Header{"Idempotency-Key": {opts.IdempotencyKey}}
		}
	}
	return c.do(ctx, http.MethodPost, path, payload, extra)
}

func (c *Client) ClustersList(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/clusters")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) ClustersGet(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/clusters/"+id, nil, nil)
}

func (c *Client) ClustersCreate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/clusters")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) ClustersUpdate(ctx context.Context, id string, body any) (any, error) {
	return c.do(ctx, http.MethodPatch, "/api/v1/clusters/"+id, body, nil)
}

func (c *Client) ClustersSuspend(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/clusters/"+id+"/suspend", nil, nil)
}

func (c *Client) ClustersResume(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/clusters/"+id+"/resume", nil, nil)
}

func (c *Client) ClustersDelete(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodDelete, "/api/v1/clusters/"+id, nil, nil)
}

func (c *Client) ClustersBoost(ctx context.Context, id string, body any) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/clusters/"+id+"/boost", body, nil)
}

func (c *Client) ClustersExtendBoost(ctx context.Context, id string, body any) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/clusters/"+id+"/extend_boost", body, nil)
}

func (c *Client) ClustersCancelBoost(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/clusters/"+id+"/cancel_boost", nil, nil)
}

func (c *Client) NetworkList(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/network")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) NetworkCreate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/network")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) NetworkAssign(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/network/assign")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) NetworkUnassign(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/network/unassign")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) NetworkSwitch(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/network/switch")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) NetworkRelease(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/network/release")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) SendingDomainsList(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/sending_domains")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) SendingDomainsGet(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/sending_domains/"+id, nil, nil)
}

func (c *Client) SendingDomainsCreate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/sending_domains")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) SendingDomainsUpdate(ctx context.Context, id string, body any) (any, error) {
	return c.do(ctx, http.MethodPatch, "/api/v1/sending_domains/"+id, body, nil)
}

func (c *Client) SendingDomainsRefresh(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/sending_domains/"+id+"/refresh", nil, nil)
}

func (c *Client) SendingDomainsVerify(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/sending_domains/"+id+"/verify", nil, nil)
}

func (c *Client) SendingDomainsSuspend(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/sending_domains/"+id+"/suspend", nil, nil)
}

func (c *Client) SendingDomainsResume(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/sending_domains/"+id+"/resume", nil, nil)
}

func (c *Client) SendingDomainsMakePrimary(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/sending_domains/"+id+"/make_primary", nil, nil)
}

func (c *Client) SendingDomainsDelete(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodDelete, "/api/v1/sending_domains/"+id, nil, nil)
}

func (c *Client) TenantsList(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/tenants")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) TenantsGet(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/tenants/"+id, nil, nil)
}

func (c *Client) TenantsCreate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/tenants")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) TenantsDelete(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodDelete, "/api/v1/tenants/"+id, nil, nil)
}

func (c *Client) InboxesList(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/inboxes")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) InboxesGet(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/inboxes/"+id, nil, nil)
}

func (c *Client) InboxesCreate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/inboxes")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) InboxesVerify(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/inboxes/"+id+"/verify", nil, nil)
}

func (c *Client) InboxesDelete(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodDelete, "/api/v1/inboxes/"+id, nil, nil)
}

func (c *Client) MessagesList(ctx context.Context, inboxID string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/inboxes/"+inboxID+"/inbound_messages", nil, nil)
}

func (c *Client) MessagesGet(ctx context.Context, inboxID, id string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/inboxes/"+inboxID+"/inbound_messages/"+id, nil, nil)
}

func (c *Client) MessagesDownloadAttachment(ctx context.Context, inboxID, id string, index int) ([]byte, error) {
	path := fmt.Sprintf("/api/v1/inboxes/%s/inbound_messages/%s/attachments/%d", inboxID, id, index)
	return c.doRaw(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) EventsList(ctx context.Context, clusterID string) (any, error) {
	path, err := c.teamsPath("/clusters/" + clusterID + "/message_events")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) EventsListTeam(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/message_events")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) EventsGet(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/message_events/"+id, nil, nil)
}

func (c *Client) SMTPCredentialsCreate(ctx context.Context, clusterID string, body any) (any, error) {
	path, err := c.teamsPath("/clusters/" + clusterID + "/smtp_credentials")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) SMTPCredentialsDelete(ctx context.Context, clusterID, id string) (any, error) {
	path, err := c.teamsPath("/clusters/" + clusterID + "/smtp_credentials/" + id)
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) WebhooksList(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/webhook_endpoints")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) WebhooksGet(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/webhook_endpoints/"+id, nil, nil)
}

func (c *Client) WebhooksCreate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/webhook_endpoints")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) WebhooksUpdate(ctx context.Context, id string, body any) (any, error) {
	return c.do(ctx, http.MethodPatch, "/api/v1/webhook_endpoints/"+id, body, nil)
}

func (c *Client) WebhooksDelete(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodDelete, "/api/v1/webhook_endpoints/"+id, nil, nil)
}

func (c *Client) TemplatesList(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/templates")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) TemplatesGet(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/templates/"+id, nil, nil)
}

func (c *Client) TemplatesCreate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/templates")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) TemplatesUpdate(ctx context.Context, id string, body any) (any, error) {
	return c.do(ctx, http.MethodPatch, "/api/v1/templates/"+id, body, nil)
}

func (c *Client) TemplatesPublish(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/templates/"+id+"/publish", nil, nil)
}

func (c *Client) TemplatesDuplicate(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/templates/"+id+"/duplicate", nil, nil)
}

func (c *Client) TemplatesDelete(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodDelete, "/api/v1/templates/"+id, nil, nil)
}

func (c *Client) SuppressionsList(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/suppressions")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) SuppressionsCreate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/suppressions")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) SuppressionsImport(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/suppressions/import")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) SuppressionsDelete(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodDelete, "/api/v1/suppressions/"+id, nil, nil)
}

func (c *Client) FirewallGet(ctx context.Context) (any, error) {
	path, err := c.teamsPath("/firewall")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) FirewallUpdate(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/firewall")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPatch, path, body, nil)
}

func (c *Client) FirewallAddEntry(ctx context.Context, body any) (any, error) {
	path, err := c.teamsPath("/firewall_entries")
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) FirewallDeleteEntry(ctx context.Context, id string) (any, error) {
	return c.do(ctx, http.MethodDelete, "/api/v1/firewall_entries/"+id, nil, nil)
}

// VerifyWebhook checks X-Capsule-Signature as HMAC-SHA256 of timestamp + "." + raw body.
func VerifyWebhook(secret, timestamp, signature string, body []byte) error {
	signature = strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(signature)
	if err != nil {
		return errors.New("invalid webhook signature")
	}
	if !hmac.Equal(expected, got) {
		return errors.New("invalid webhook signature")
	}
	return nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}
