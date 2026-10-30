package inbound_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound"
)

func request(method string, origin string, host string) *http.Request {
	req, _ := http.NewRequest(method, "http://"+host+"/", nil)
	req.Host = host
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestSameOriginWriteIsAllowed(t *testing.T) {
	req := request(http.MethodPost, "http://192.168.1.5:3000", "192.168.1.5:3000")

	require.False(t, inbound.IsCrossSiteWrite(req))
}

func TestOtherOriginWriteIsRejected(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodDelete, http.MethodPut} {
		req := request(method, "https://evil.example", "localhost:3000")
		require.True(t, inbound.IsCrossSiteWrite(req), method)
	}
}

func TestOpaqueOriginIsRejected(t *testing.T) {
	req := request(http.MethodPost, "null", "localhost:3000")

	require.True(t, inbound.IsCrossSiteWrite(req))
}

func TestMissingHostCannotBeProvenSameOrigin(t *testing.T) {
	req := request(http.MethodPost, "http://localhost:3000", "localhost:3000")
	req.Host = ""

	require.True(t, inbound.IsCrossSiteWrite(req))
}

func TestReadsAndOriginlessClientsAreNotAffected(t *testing.T) {
	cross := request(http.MethodGet, "https://evil.example", "localhost:3000")
	require.False(t, inbound.IsCrossSiteWrite(cross))

	head := request(http.MethodHead, "https://evil.example", "localhost:3000")
	require.False(t, inbound.IsCrossSiteWrite(head))

	options := request(http.MethodOptions, "https://evil.example", "localhost:3000")
	require.False(t, inbound.IsCrossSiteWrite(options))

	curl := request(http.MethodPost, "", "localhost:3000")
	require.False(t, inbound.IsCrossSiteWrite(curl))
}

func TestHostComparisonIgnoresASCIICase(t *testing.T) {
	req := request(http.MethodPost, "http://LOCALHOST:3000", "localhost:3000")

	require.False(t, inbound.IsCrossSiteWrite(req))
}
