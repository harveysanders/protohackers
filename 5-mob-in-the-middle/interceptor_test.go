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
			label: "ignores valid address in the middle of a message",
			input: []byte("Hi alice, my address is 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX. Send it there"),
			want:  []byte("Hi alice, my address is 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX. Send it there"),
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
