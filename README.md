# PostShiba Go

Go client for the PostShiba API.

## Installation

```sh
go get github.com/postshiba/postshiba-go
```

This module is not on a package registry yet. Install from GitHub.

## How It Works

`NewClient` sends JSON to `https://app.postshiba.com/api/v1` with a Bearer token. Team-scoped paths use `WithTeamID`. `GET /users/me` does not return a team id, so the client will not guess one.

## Send an email

```go
client := postshiba.NewClient(os.Getenv("POSTSHIBA_API_KEY"), postshiba.WithTeamID("1"))

res, err := client.EmailsSend(ctx, map[string]any{
	"from":    "hello@mail.example.com",
	"to":      []string{"you@example.com"},
	"subject": "PostShiba test",
	"text":    "hello from PostShiba",
	"html":    "<p>hello from PostShiba</p>",
})
```

Cluster send can set `Idempotency-Key` and `"sandbox": true`.

```go
res, err := client.EmailsSendOnCluster(ctx, "4", body, &postshiba.SendOnClusterOptions{
	IdempotencyKey: "idem-1",
	Sandbox:        true,
})
```

## API

### Users

```go
me, err := client.UsersMe(ctx)
```

### Clusters

```go
clusters, err := client.ClustersList(ctx)
cluster, err := client.ClustersGet(ctx, "4")
cluster, err := client.ClustersCreate(ctx, map[string]any{
	"cluster": map[string]any{"name": "edge", "size": "small", "region": "manual", "plan": "nano"},
})
cluster, err := client.ClustersUpdate(ctx, "4", map[string]any{
	"cluster": map[string]any{"plan": "small"},
})
cluster, err := client.ClustersSuspend(ctx, "4")
cluster, err := client.ClustersResume(ctx, "4")
cluster, err := client.ClustersDelete(ctx, "4")
```

### Sending domains

```go
domains, err := client.SendingDomainsList(ctx)
domain, err := client.SendingDomainsGet(ctx, "8")
domain, err := client.SendingDomainsCreate(ctx, map[string]any{
	"sending_domain": map[string]any{"name": "mail.example.com", "tenant_id": 12},
})
domain, err := client.SendingDomainsVerify(ctx, "8")
domain, err := client.SendingDomainsSuspend(ctx, "8")
domain, err := client.SendingDomainsResume(ctx, "8")
domain, err := client.SendingDomainsMakePrimary(ctx, "8")
_, err = client.SendingDomainsDelete(ctx, "8")
```

### Tenants

```go
tenants, err := client.TenantsList(ctx)
tenant, err := client.TenantsGet(ctx, "12")
tenant, err := client.TenantsCreate(ctx, map[string]any{
	"tenant": map[string]any{"name": "Acme Florist"},
})
_, err = client.TenantsDelete(ctx, "12")
```

### Inboxes

```go
inboxes, err := client.InboxesList(ctx)
inbox, err := client.InboxesGet(ctx, "3")
inbox, err := client.InboxesCreate(ctx, map[string]any{
	"inbox": map[string]any{"name": "agent", "webhook_url": "https://hooks.example.com/mail"},
})
inbox, err := client.InboxesVerify(ctx, "3")
inbox, err := client.InboxesDelete(ctx, "3")
```

### Messages

```go
messages, err := client.MessagesList(ctx, "3")
message, err := client.MessagesGet(ctx, "3", "21")
file, err := client.MessagesDownloadAttachment(ctx, "3", "21", 0)
```

### Events

```go
events, err := client.EventsList(ctx, "4")
event, err := client.EventsGet(ctx, "44")
```

### SMTP credentials

```go
cred, err := client.SMTPCredentialsCreate(ctx, "4", map[string]any{
	"smtp_credential": map[string]any{"tenant_id": 12},
})
_, err = client.SMTPCredentialsDelete(ctx, "4", "9")
```

The password is present on create only.

### Webhooks

```go
hooks, err := client.WebhooksList(ctx)
hook, err := client.WebhooksGet(ctx, "2")
hook, err := client.WebhooksCreate(ctx, map[string]any{
	"webhook_endpoint": map[string]any{
		"url":         "https://hooks.example.com/capsule",
		"event_types": []string{"delivered", "bounce"},
		"cluster_id":  4,
	},
})
```

The secret is present on get and create. It is omitted on list. The API has no update or delete.

### Suppressions

```go
rows, err := client.SuppressionsList(ctx)
row, err := client.SuppressionsCreate(ctx, map[string]any{
	"suppression": map[string]any{"email": "blocked@example.com", "tenant_id": 12},
})
_, err = client.SuppressionsDelete(ctx, "7")
```

### Firewall

```go
fw, err := client.FirewallGet(ctx)
fw, err := client.FirewallUpdate(ctx, map[string]any{
	"firewall": map[string]any{"enabled_checks": []string{"temp_providers", "plus_addressing"}},
})
entry, err := client.FirewallAddEntry(ctx, map[string]any{
	"firewall_entry": map[string]any{"list": "deny", "value": "mailinator.com"},
})
_, err = client.FirewallDeleteEntry(ctx, "3")
```

## Verify webhooks

HMAC-SHA256 of `{timestamp}.{rawBody}` compared to `X-Capsule-Signature`. A `sha256=` prefix is stripped.

```go
err := postshiba.VerifyWebhook(secret, timestamp, signature, rawBody)
```

## Errors

Non-2xx responses return `*postshiba.Error` with `Err`, `Field`, and `Message`. `Err` is the JSON `error` key. Go uses `Error()` for the error interface.

```go
var apiErr *postshiba.Error
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Err, apiErr.Field, apiErr.Message)
}
```

A team-scoped call without `WithTeamID` returns an error. The client does not read a team id from `UsersMe`.

## Contributing

```sh
go test ./...
```
