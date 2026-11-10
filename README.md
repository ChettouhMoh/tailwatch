# Tailwatch

Tailwatch sends a notification for **every successful SSH login** it observes on a Linux VPS. It listens to Tailscale SSH and OpenSSH in the systemd journal. Every login goes to stdout as one JSON object. Select an HTTPS webhook, Telegram, or WhatsApp to receive the same login remotely.

## What triggers an alert

| Source | Evidence of success | Information available |
| --- | --- | --- |
| Tailscale SSH | A recognized post-authentication session start or grant from `tailscaled`. Adjacent records with the same session ID produce one event. | Server, time, local account and the Tailscale login from the journal. A detailed start also supplies the source IP. A successful lookup can add the device name and stable ID. |
| OpenSSH with a recognized Tailscale peer | The SSH daemon logs `Accepted` after successful authentication. | Server, time, local account, method and peer IP. A successful `tailscale whois` lookup can add the device and the associated owner. The owner describes the peer and the SSH account describes the login. |
| OpenSSH from a public, LAN or unresolved address | The same OpenSSH authentication event. | Server, time, account, method and the source IP seen by the VPS. A proxy, bastion or subnet router may be the immediate peer. Tailwatch leaves the original device and route unspecified. |

Logins that match an allowlist still generate a notification. Failed attempts, incomplete MFA steps, duplicate PAM session-open messages, and ordinary network connections do not. With multiplexed OpenSSH connections, authentication happens once and later channels reuse it. Tailscale SSH emits events per authorized session channel. Provider web consoles, serial consoles and local logins are outside this SSH collector.

The live reader accepts Tailscale messages from `tailscaled.service` and root-owned journal records from `sshd`, `sshd-session` and `sshd-auth`, including socket-activated units. The host must retain those messages in journald. A syslog-only host needs to make its SSH logs available in the journal. Tailscale's native SSH messages are diagnostic and can change with upgrades. [Tailscale logging](https://tailscale.com/docs/features/logging?tab=linux) · [Tailscale SSH source](https://github.com/tailscale/tailscale/blob/main/ssh/tailssh/tailssh.go) · [OpenSSH authentication source](https://github.com/openssh/openssh-portable/blob/master/auth.c)

## Try it locally

Go 1.21 or newer is required to build. The replay fixture runs on Linux, macOS and Windows.

```sh
go test ./...
go run ./cmd/tailwatch -config config.example.json -replay testdata/journal.jsonl
```

With an extracted Linux binary, use `./tailwatch -config config.example.json -replay testdata/journal.jsonl`. Expect **four events** from the mixed fixture. Replay prints events locally, skips remote senders and leaves the live checkpoint untouched. Diagnostics go to stderr. The JSON includes `ssh_server`, a readable `message`, and optional `auth_method` or peer identity fields. Existing consumers can still use `event_type: SSH_SESSION_STARTED` and `id`. An OpenSSH event ID is derived from its journal cursor.

## Choose a notification channel

Copy `config.example.json` to `config.json`. Replace its `notification` object with one of the examples below. Stdout remains available with every provider. Restart the service after changing the configuration or environment. The sample configuration uses stdout alone.

**Generic HTTPS webhook**

```json
{"provider":"webhook"}
```

Set `TAILWATCH_WEBHOOK_URL` to the receiver URL, or set `notification.webhook_url` in the file. The environment variable takes precedence. The receiver gets the full JSON event and must return HTTP 2xx. Use the `Idempotency-Key` request header, also present in the body as `id`, to deduplicate retries. Upgrading from the original webhook behavior enables delivery for trusted logins too.

**Telegram**

```json
{"provider":"telegram","telegram":{"chat_id":"-1001234567890"}}
```

Set `TAILWATCH_TELEGRAM_BOT_TOKEN` to your bot token. Start a private chat with the bot, or give it posting access to the destination group or channel. Use the actual chat ID or channel username. Tailwatch sends a plain-text login summary using `sendMessage`. [Telegram Bot API](https://core.telegram.org/bots/api#sendmessage)

**WhatsApp Business Cloud API**

```json
{
  "provider":"whatsapp",
  "whatsapp":{
    "api_version":"v23.0",
    "phone_number_id":"1234567890",
    "recipient":"15550100000",
    "template":"vps_login",
    "language":"en_US"
  }
}
```

Use the Graph API version enabled for your integration, the sender's **phone number ID**, and the recipient's phone number with country code, digits only. Set `TAILWATCH_WHATSAPP_ACCESS_TOKEN` to a token with messaging access. Configure an approved text template in the indicated language with **one positional body parameter** and no required header or button parameters. For example, the template body might read “SSH login alert — {{1}}. Check your server log for details.” Tailwatch fills that parameter with the login summary. A successful API response with a message ID confirms submission to Meta. [Meta's template message example](https://www.postman.com/meta/whatsapp-business-platform/request/o65u5m5/send-message-template-text)

All remote requests use a 10-second timeout. Failed sends leave the original event pending. After a restart Tailwatch retries it before reading later events. A crash after the receiver accepts a message can produce a duplicate, including in Telegram or WhatsApp. Journal retention limits catch-up after an extended outage.

## Identity and trust

For OpenSSH, Tailwatch attempts a lookup only for addresses in Tailscale's IPv4 or IPv6 ranges. An address in those ranges by itself does not establish a peer identity. Tagged devices can have a device name and ID without a human owner. Missing identity still produces an alert. Lookups during replay or backlog processing reflect the mapping at lookup time. The default timeout is 3 seconds and successful lookups are cached for 60 seconds. [Tailscale CLI](https://tailscale.com/docs/reference/tailscale-cli#whois)

Configure `trusted_device_ids` for individual devices, `trusted_users` for every user-owned device belonging to an exact login, or `trusted_ips` for exact addresses, including when lookup fails. Any matching rule yields `TRUSTED` unless a native Tailscale session's recorded login conflicts with the current lookup. Other classifications are `UNRECOGNIZED_TAILNET`, `UNKNOWN`, and `EXTERNAL`. Here `EXTERNAL` means an unresolved source outside Tailscale's address ranges and can include a LAN address. Classification annotates the event. Every classification is delivered.

## Run on a Linux VPS

The live source needs a systemd host with `journalctl` supporting `--output-fields` and access to the system journal. OpenSSH monitoring can run without Tailscale installed. Install the Tailscale CLI for peer enrichment and enable Tailscale SSH on hosts where you want to observe native sessions.

When building from source, run `go build -o tailwatch ./cmd/tailwatch`. From the repository or extracted Linux archive:

```sh
sudo install -m 0755 tailwatch /usr/local/bin/tailwatch
sudo install -d -m 0755 /etc/tailwatch
if ! sudo test -e /etc/tailwatch/config.json; then
    sudo install -m 0644 config.example.json /etc/tailwatch/config.json
fi
sudoedit /etc/tailwatch/config.json
sudo install -m 0644 deploy/tailwatch.service /etc/systemd/system/tailwatch.service
sudo systemctl daemon-reload
sudo systemctl enable tailwatch
sudo systemctl restart tailwatch
sudo journalctl -u tailwatch -f -o cat
```

For a remote channel, put the required environment variable in a root-owned, mode-0600 `/etc/tailwatch/environment` before restarting. The unit runs under an unprivileged account with journal access. It stores the pending event, cursor and last 256 delivered IDs in `/var/lib/tailwatch/journal.json`. Existing checkpoints carry forward. A foreground run can use `./tailwatch -config config.json -state ./tailwatch-state.json` from a writable directory. A first run starts at the current second.

## Verify detection

Open a fresh SSH connection through each entry point enabled on your VPS. For OpenSSH, avoid reusing a multiplexed connection during this check. Confirm that the Linux account and server in the notification match the login. Connect once with an allowlisted device too and verify it still sends. After Tailscale upgrades, repeat the native SSH check to confirm its journal format.

To inspect the last ten minutes using the same source filters as the daemon:

```sh
sudo journalctl --since '10 minutes ago' -o json \
  _SYSTEMD_UNIT=tailscaled.service _COMM=tailscaled + \
  _UID=0 _COMM=sshd _COMM=sshd-session _COMM=sshd-auth > login.jsonl
./tailwatch -config config.example.json -replay login.jsonl
```
