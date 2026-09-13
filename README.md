# PostShiba Go

> [!NOTE]
> [PostShiba skills](https://github.com/postshiba/postshiba-skills) connect agents to MCP for first send, domains, inboxes, and sandbox mail.

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
client := postshiba.NewClient(os.Getenv("POSTSHIBA_API_KEY"), postshiba.WithTeamID("KjkAJW"))

res, err := client.EmailsSend(ctx, map[string]any{
	"from":    "hello@mail.example.com",
	"to":      []string{"you@example.com"},
	"subject": "PostShiba test",
	"text":    "hello from PostShiba",
	"html":    "<p>hello from PostShiba</p>",
})
```

Pass a cluster id to pin `X-Capsule-Cluster-Id`. Omit it and the header is not sent.

```go
res, err := client.EmailsSend(ctx, body, &postshiba.SendOptions{ClusterID: "NmQpXr"})
```

Send a published template. `EmailsSend` takes the same opaque body. There is no `SendTemplate` method.

```go
res, err := client.EmailsSend(ctx, map[string]any{
	"to": []string{"you@example.com"},
	"template": map[string]any{
		"id":        "welcome",
		"variables": map[string]any{"name": "Ada"},
	},
})
```

Cluster send can set `Idempotency-Key` and `"sandbox": true`.

```go
res, err := client.EmailsSendOnCluster(ctx, "NmQpXr", body, &postshiba.SendOnClusterOptions{
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
cluster, err := client.ClustersGet(ctx, "NmQpXr")
cluster, err := client.ClustersCreate(ctx, map[string]any{
	"cluster": map[string]any{"name": "edge", "size": "small", "region": "manual", "plan": "nano"},
})
cluster, err := client.ClustersUpdate(ctx, "NmQpXr", map[string]any{
	"cluster": map[string]any{"plan": "small"},
})
cluster, err := client.ClustersSuspend(ctx, "NmQpXr")
cluster, err := client.ClustersResume(ctx, "NmQpXr")
cluster, err := client.ClustersDelete(ctx, "NmQpXr")
cluster, err := client.ClustersBoost(ctx, "NmQpXr", map[string]any{"sku": "small_to_large"})
cluster, err := client.ClustersExtendBoost(ctx, "NmQpXr", map[string]any{"idempotency_key": "extend-1"})
cluster, err := client.ClustersCancelBoost(ctx, "NmQpXr")
```

### Network

```go
ips, err := client.NetworkList(ctx)
ip, err := client.NetworkCreate(ctx, map[string]any{"ip_address_id": "IpQwEr", "cluster_id": "NmQpXr"})
ip, err := client.NetworkAssign(ctx, map[string]any{"ip_address_id": "IpQwEr", "cluster_id": "NmQpXr"})
ip, err := client.NetworkUnassign(ctx, map[string]any{"ip_address_id": "IpQwEr", "cluster_id": "NmQpXr"})
ip, err := client.NetworkSwitch(ctx, map[string]any{"ip_address_id": "IpQwEr", "cluster_id": "NmQpXr"})
ip, err := client.NetworkRelease(ctx, map[string]any{"ip_address_id": "IpQwEr"})
```

### Sending domains

```go
domains, err := client.SendingDomainsList(ctx)
domain, err := client.SendingDomainsGet(ctx, "HsVtYk")
domain, err := client.SendingDomainsCreate(ctx, map[string]any{
	"sending_domain": map[string]any{"name": "mail.example.com", "tenant_id": "WbLcFd"},
})
domain, err := client.SendingDomainsUpdate(ctx, "HsVtYk", map[string]any{
	"sending_domain": map[string]any{"dkim_selector": "s1", "dkim_manual": true},
})
domain, err := client.SendingDomainsRefresh(ctx, "HsVtYk")
domain, err := client.SendingDomainsVerify(ctx, "HsVtYk")
domain, err := client.SendingDomainsSuspend(ctx, "HsVtYk")
domain, err := client.SendingDomainsResume(ctx, "HsVtYk")
domain, err := client.SendingDomainsMakePrimary(ctx, "HsVtYk")
_, err = client.SendingDomainsDelete(ctx, "HsVtYk")
```

### Tenants

```go
tenants, err := client.TenantsList(ctx)
tenant, err := client.TenantsGet(ctx, "WbLcFd")
tenant, err := client.TenantsCreate(ctx, map[string]any{
	"tenant": map[string]any{"name": "Acme Florist"},
})
_, err = client.TenantsDelete(ctx, "WbLcFd")
```

### Inboxes

```go
inboxes, err := client.InboxesList(ctx)
inbox, err := client.InboxesGet(ctx, "PqRzMn")
inbox, err := client.InboxesCreate(ctx, map[string]any{
	"inbox": map[string]any{
		"name":        "agent",
		"webhook_url": "https://hooks.example.com/mail",
		"host":        "inbound.example.com",
		"forward_to":  "you@example.com",
	},
})
inbox, err := client.InboxesVerify(ctx, "PqRzMn")
inbox, err := client.InboxesDelete(ctx, "PqRzMn")
```

### Messages

```go
messages, err := client.MessagesList(ctx, "PqRzMn")
message, err := client.MessagesGet(ctx, "PqRzMn", "GxTyVu")
file, err := client.MessagesDownloadAttachment(ctx, "PqRzMn", "GxTyVu", 1)
```

### Events

```go
events, err := client.EventsListTeam(ctx)
events, err := client.EventsList(ctx, "NmQpXr")
event, err := client.EventsGet(ctx, "JkLmNp")
```

### SMTP credentials

```go
cred, err := client.SMTPCredentialsCreate(ctx, "NmQpXr", map[string]any{
	"smtp_credential": map[string]any{"tenant_id": "WbLcFd"},
})
_, err = client.SMTPCredentialsDelete(ctx, "NmQpXr", "RvWsXq")
```

The password is present on create only.

### Webhooks

```go
hooks, err := client.WebhooksList(ctx)
hook, err := client.WebhooksGet(ctx, "CdFgHj")
hook, err := client.WebhooksCreate(ctx, map[string]any{
	"webhook_endpoint": map[string]any{
		"url":         "https://hooks.example.com/capsule",
		"event_types": []string{"delivered", "bounce"},
		"cluster_id":  "NmQpXr",
	},
})
hook, err = client.WebhooksUpdate(ctx, "CdFgHj", map[string]any{
	"webhook_endpoint": map[string]any{
		"enabled":     false,
		"event_types": []string{"delivered", "bounce"},
	},
})
_, err = client.WebhooksDelete(ctx, "CdFgHj")
```

The secret is present on get and create. It is omitted on list and update.

### Templates

```go
templates, err := client.TemplatesList(ctx)
tpl, err := client.TemplatesGet(ctx, "welcome")
tpl, err := client.TemplatesCreate(ctx, map[string]any{
	"email_template": map[string]any{
		"name":    "Welcome",
		"alias":   "welcome",
		"subject": "Hi {{ name }}",
		"html":    "<p>Hi {{ name }}</p>",
	},
})
tpl, err := client.TemplatesUpdate(ctx, "TpLmQr", map[string]any{
	"email_template": map[string]any{"subject": "Welcome, {{ name }}"},
})
tpl, err := client.TemplatesPublish(ctx, "TpLmQr")
tpl, err := client.TemplatesDuplicate(ctx, "TpLmQr")
_, err = client.TemplatesDelete(ctx, "TpLmQr")
```

Send uses the published snapshot. `TemplatesGet` and member routes accept the public id or the alias.

### Suppressions

```go
rows, err := client.SuppressionsList(ctx)
row, err := client.SuppressionsCreate(ctx, map[string]any{
	"suppression": map[string]any{"email": "blocked@example.com", "tenant_id": "WbLcFd"},
})
row, err := client.SuppressionsImport(ctx, map[string]any{
	"emails":    []string{"blocked@example.com", "old@example.com"},
	"tenant_id": "WbLcFd",
})
_, err = client.SuppressionsDelete(ctx, "YtReWq")
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
_, err = client.FirewallDeleteEntry(ctx, "PqRzMn")
```

## Verify webhooks

HMAC-SHA256 of `{timestamp}.{rawBody}` compared to `X-Capsule-Signature`. A `sha256=` prefix is stripped.

```go
err := postshiba.VerifyWebhook(secret, timestamp, signature, rawBody)
```

## Errors and throttling

Non-2xx responses return `*postshiba.Error` with `Err`, `Field`, and `Message`. `Err` is the JSON `error` key. Go uses `Error()` for the error interface.

```go
var apiErr *postshiba.Error
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Err, apiErr.Field, apiErr.Message)
}
```

A `429` response with `error` `throttled` means the cluster hit its hourly send limit. Do not retry that send immediately. Immediate retries hit the same cap. Wait until the next hour. The client does not delay for you. In a queued worker, check `apiErr.Err == "throttled"` before sending again.

A team-scoped call without `WithTeamID` returns an error. The client does not read a team id from `UsersMe`.

## Contributing

```sh
go test ./...
```
