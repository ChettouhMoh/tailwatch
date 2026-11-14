# Tailwatch

Tailwatch sends a notification for **every successful SSH login** it observes on a Linux VPS. It listens to Tailscale SSH and OpenSSH in the systemd journal. Every login goes to stdout as one JSON object. Select an HTTPS webhook, Telegram, or WhatsApp to receive the same login remotely.

## Install on Ubuntu

On an Ubuntu Server VPS with systemd 249 or newer (Ubuntu 22.04 and 24.04 meet this requirement), run `dpkg --print-architecture`. Copy the matching `tailwatch-amd64.deb` or `tailwatch-arm64.deb` onto the VPS. From the directory containing the file, run:

```sh
sudo apt install "./tailwatch-$(dpkg --print-architecture).deb"
sudo tailwatch-setup
```

The package installs and enables the service. A fresh install starts recording successful SSH logins in the local journal immediately. Setup asks for your channel and the details in the table. It hides the credential as you type, validates the settings locally and restarts Tailwatch. Run the same command later to change channels or rotate a token.

| Channel | Have this ready |
| --- | --- |
| `stdout` | Nothing. Read alerts with `sudo journalctl -u tailwatch -f -o cat`. |
| `telegram` | Bot token and destination chat ID or channel username. Start the bot in a private chat or give it posting access in the destination. |
| `whatsapp` | WhatsApp Business Cloud API token, sender phone number ID, recipient with country code, Graph API version, approved template name and language. The template needs one positional body parameter. |
| `webhook` | The full HTTPS URL of the receiver. |

Open a **fresh SSH connection** to verify delivery. Use `systemctl status tailwatch` to check the service and `sudo journalctl -u tailwatch -n 30 -o cat` to inspect recent events or delivery errors. Install the Tailscale CLI if you want peer enrichment. OpenSSH monitoring works on its own.

### Where settings and credentials live

| File | Contents |
| --- | --- |
| `/etc/tailwatch/config.json` | Trust lists, lookup settings and the default channel. Maintainers can edit it with `sudoedit` and apply changes with `sudo systemctl restart tailwatch`. The wizard leaves this file alone. |
| `/etc/tailwatch/channel.json` | The selected channel, its destination and its secret. The installer creates an empty root-owned file with mode `0600`. The wizard fills it with plaintext and systemd passes a private read-only copy to the service. Each successful change replaces the previous channel. |
| `/etc/tailwatch/environment` | Optional environment variables from earlier installations. With no wizard-managed channel, Tailwatch uses the notification settings in the main JSON and the existing environment. |
| `/var/lib/tailwatch/journal.json` | The journal checkpoint and pending delivery. Package upgrades and setup preserve it. |

Use `sudo apt install ./tailwatch-amd64.deb` (or the arm64 file) for a newer package. `sudo apt remove tailwatch` stops the packaged service and keeps its configuration. `sudo apt purge tailwatch` also removes the package configuration and wizard-managed credential. An administrator-created environment file and journal state remain on disk.

### Earlier manual installs

If you previously copied the unit to `/etc/systemd/system/tailwatch.service`, systemd gives that file priority over the packaged unit. The installer preserves your local unit. To adopt the packaged one, review any customizations with `systemctl cat tailwatch`, install the package, then run:

```sh
sudo mv -i /etc/systemd/system/tailwatch.service /etc/systemd/system/tailwatch.service.manual-backup
sudo systemctl daemon-reload
sudo systemctl reenable tailwatch
sudo systemctl restart tailwatch
sudo tailwatch-setup
```

If apt asks about a modified `config.json`, choose to keep your local version. The packaged unit invokes `/usr/bin/tailwatch`. A previous copy in `/usr/local/bin` may still take priority when you type `tailwatch` at a shell. After verifying a migrated channel, remove obsolete credentials from any manually maintained files or backups that contain them.

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

## Manual channel configuration

For a foreground run, copy `config.example.json` to `config.json` and use one of the notification objects below. These settings also apply to the packaged service before a wizard-managed channel exists. Stdout remains available with every provider. Restart the service after editing files by hand. The sample configuration uses stdout alone.

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

## Build the Ubuntu package

From a checkout with Go 1.21+ and `dpkg-deb` on the build machine, choose a Debian package version and target architecture:

```sh
sh deploy/build-deb.sh '0.1.0~dev1' amd64 tailwatch-amd64.deb
sh deploy/build-deb.sh '0.1.0~dev1' arm64 tailwatch-arm64.deb
```

To package a binary already cross-compiled on another build host, set `TAILWATCH_BINARY` to its path when running the same script. The packager checks its Go package and target architecture.

The installed VPS uses the compiled binary. The service runs under an unprivileged account with journal access. For a foreground run, use `./tailwatch -config config.json -state ./tailwatch-state.json` from a writable directory with permission to read the system journal. A first run starts at the current second.

## Verify detection

Open a fresh SSH connection through each entry point enabled on your VPS. For OpenSSH, avoid reusing a multiplexed connection during this check. Confirm that the Linux account and server in the notification match the login. Connect once with an allowlisted device too and verify it still sends. After Tailscale upgrades, repeat the native SSH check to confirm its journal format.

To inspect the last ten minutes using the same source filters as the daemon:

```sh
sudo journalctl --since '10 minutes ago' -o json \
  _SYSTEMD_UNIT=tailscaled.service _COMM=tailscaled + \
  _UID=0 _COMM=sshd _COMM=sshd-session _COMM=sshd-auth > login.jsonl
./tailwatch -config config.example.json -replay login.jsonl
```
