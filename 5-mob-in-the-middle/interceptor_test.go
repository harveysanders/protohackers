package mobprox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBcoinReplacer(t *testing.T) {
	testCases := []struct {
		label string
		input []byte
		want  []byte
	}{
		{
			label: "replaces valid address at the end of a message",
			input: []byte("Hi alice, please send payment to 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX"),
			want:  []byte("Hi alice, please send payment to 7YWHMfk9JZe0LM0g1ZauHuiSxhI"),
		},
		{
			label: "replaces valid address in the middle of a message",
			input: []byte("Hi alice, my address is 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX. Send it there"),
			want:  []byte("Hi alice, my address is 7YWHMfk9JZe0LM0g1ZauHuiSxhI. Send it there"),
		},
		{
			label: "handles newlines at the end of a message",
			input: []byte(`Hi alice, please send payment to 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX
`),
			want: []byte(`Hi alice, please send payment to 7YWHMfk9JZe0LM0g1ZauHuiSxhI
`),
		},
		{
			label: "handles multiple addresses in a message",
			input: []byte("Please pay the ticket price of 15 Boguscoins to one of these addresses: 76TaUbtoWIjufQZYZ5eHBjZYl1Yg 7EViBcjBeCzkIDD7QRMEmbbSgFyzg 7sxiP0k46XkP3x5nLdqwewRPNRJKW1Nwcnp"),
			want:  []byte("Please pay the ticket price of 15 Boguscoins to one of these addresses: 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI"),
		},
		{
			label: "handles message already containing Tony's boguscoin address",
			input: []byte(`Please pay the ticket price of 15 Boguscoins to one of these addresses: 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI`),
			want:  []byte(`Please pay the ticket price of 15 Boguscoins to one of these addresses: 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI`),
		},
		{
			label: "only addresses and newline",
			input: []byte(`7hKVVA8GhgZryQuA3OSPN4tVWd036vG2 7xRm57z5LSNUciHUSa0UVavaLm91ueX
`),
			want: []byte(`7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI
`),
		},
		{
			label: "ignores addresses that are too long",
			input: []byte(`This is too long: 7oPno7jAAMsJixZQDjjNb9xqYZFqtZ4oJsPf`),
			want:  []byte(`This is too long: 7oPno7jAAMsJixZQDjjNb9xqYZFqtZ4oJsPf`),
		},
		{
			label: "ignores addresses that are product IDs",
			input: []byte(`This is a product ID, not a Boguscoin: 7UgRd4RcbsAqqujDwCYkiWcBxb-B7uAdDXLOAcD7svIouZXXB7kgL5orsy5cW-1234`),
			want:  []byte(`This is a product ID, not a Boguscoin: 7UgRd4RcbsAqqujDwCYkiWcBxb-B7uAdDXLOAcD7svIouZXXB7kgL5orsy5cW-1234`),
		},
	}

	tonyBcoin := "7YWHMfk9JZe0LM0g1ZauHuiSxhI"
	i := newbcoinReplacer(tonyBcoin)
	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			got := i.intercept(tc.input)
			require.Equal(t, string(tc.want), string(got))
		})
	}
}
