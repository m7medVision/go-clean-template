package limiter

import "net/netip"

func ClientID(scope string, addr netip.Addr) string {
	return clientID(scope, addr)
}

func AccountID(scope, account string) string {
	return accountID(scope, account)
}
