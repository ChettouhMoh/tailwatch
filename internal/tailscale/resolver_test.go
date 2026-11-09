package tailscale

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"testing"
	"time"
)

const whoisFixture = `{"Node":{"ID":123,"StableID":"n-stable","Name":"laptop.example.ts.net.","Tags":[]},"UserProfile":{"LoginName":"alice@example.com"}}`

// The resolver executes this test binary with the real CLI arguments.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "whois" {
		if len(os.Args) != 4 || os.Args[2] != "--json" || os.Args[3] != "100.100.12.5" {
			os.Exit(2)
		}
		if os.Getenv("TAILWATCH_TEST_WHOIS") == "slow" {
			time.Sleep(10 * time.Second)
		}
		fmt.Print(os.Getenv("TAILWATCH_TEST_WHOIS"))
		return
	}
	os.Exit(m.Run())
}

func TestResolveCachesSuccessAndRetriesFailure(t *testing.T) {
	ip := netip.MustParseAddr("100.100.12.5")
	r := NewResolver(os.Args[0], 3*time.Second, time.Minute)
	t.Setenv("TAILWATCH_TEST_WHOIS", "{}")
	if device, err := r.Resolve(context.Background(), ip); err == nil || device != nil {
		t.Fatal("accepted invalid identity")
	}
	t.Setenv("TAILWATCH_TEST_WHOIS", whoisFixture)
	device, err := r.Resolve(context.Background(), ip)
	if err != nil || device.ID != "n-stable" || device.Hostname != "laptop.example.ts.net" || device.LoginName != "alice@example.com" {
		t.Fatalf("device=%+v error=%v", device, err)
	}
	t.Setenv("TAILWATCH_TEST_WHOIS", "{}")
	if _, err := r.Resolve(context.Background(), ip); err != nil {
		t.Fatalf("cache miss: %v", err)
	}
	r.cache[ip] = cachedDevice{device: *device, expires: time.Now().Add(-time.Second)}
	if _, err := r.Resolve(context.Background(), ip); err == nil {
		t.Fatal("stale cached device returned")
	}
}

func TestWhoisTimeout(t *testing.T) {
	t.Setenv("TAILWATCH_TEST_WHOIS", "slow")
	r := NewResolver(os.Args[0], 100*time.Millisecond, time.Minute)
	_, err := r.Resolve(context.Background(), netip.MustParseAddr("100.100.12.5"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
}

func TestParseTaggedNode(t *testing.T) {
	device, err := parseWhois([]byte(`{"Node":{"StableID":"n-tag","Tags":["tag:ci"]},"UserProfile":{"LoginName":"tagged-devices"}}`))
	if err != nil || !device.Tagged || device.ID != "n-tag" {
		t.Fatalf("device=%+v error=%v", device, err)
	}
}
