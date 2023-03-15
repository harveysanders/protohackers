package budgetchat_test

import (
	"testing"

	chat "github.com/harveysanders/protohackers/budgetchat"
	"github.com/stretchr/testify/require"
)

func TestValidateName(t *testing.T) {
	testCases := []struct {
		name    string
		isValid bool
	}{
		{"ice T", false},
		{"taco", true},
		{"taco%^@", false},
	}

	for _, tc := range testCases {
		err := chat.ValidateName([]byte(tc.name))
		if tc.isValid {
			require.Nil(t, err)
		} else {
			require.Error(t, err)
		}
	}
}
