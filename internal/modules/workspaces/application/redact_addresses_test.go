package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactAddressesHidesEveryWayAServiceAddressAppearsInAnErrorText(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string
	}{
		"http url with credentials":  {`Get "http://user:s3cret@127.0.0.1:7437/projects": refused`, `Get "[address]": refused`},
		"https url":                  {`request to https://engram.example.com/projects failed`, `request to [address] failed`},
		"another scheme":             {`dial ftp://files.internal:21 failed`, `dial [address] failed`},
		"upper case scheme":          {`HTTP://Host:7437/x broke`, `[address] broke`},
		"bare ipv4 with port":        {`dial tcp 127.0.0.1:7437: connection refused`, `dial tcp [address]: connection refused`},
		"bare hostname with port":    {`dial tcp engram.local:7437: i/o timeout`, `dial tcp [address]: i/o timeout`},
		"localhost with port":        {`dial tcp localhost:7437: connection refused`, `dial tcp [address]: connection refused`},
		"bracketed ipv6 with port":   {`dial tcp [::1]:7437: connection refused`, `dial tcp [address]: connection refused`},
		"userinfo without a scheme":  {`proxy user:s3cret@10.0.0.1 rejected`, `proxy [address] rejected`},
		"several addresses at once":  {`a http://one.test b 10.0.0.2:80 c`, `a [address] b [address] c`},
		"plain message is untouched": {`connection refused`, `connection refused`},
		"status and counters":        {`status 500 after 3 tries, retry:5`, `status 500 after 3 tries, retry:5`},
		"words with a colon":         {`error: timeout waiting`, `error: timeout waiting`},
		"empty text":                 {``, ``},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, redactAddresses(tc.in))
		})
	}
}
