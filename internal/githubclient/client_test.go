package githubclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAPIBaseURL(t *testing.T) {
	t.Run("empty is allowed", func(t *testing.T) {
		got, err := NormalizeAPIBaseURL("")
		require.NoError(t, err)
		require.Equal(t, "", got)
	})

	t.Run("adds trailing slash", func(t *testing.T) {
		got, err := NormalizeAPIBaseURL("https://ghe.example.com/api/v3")
		require.NoError(t, err)
		require.Equal(t, "https://ghe.example.com/api/v3/", got)
	})

	t.Run("keeps trailing slash", func(t *testing.T) {
		got, err := NormalizeAPIBaseURL("https://api.github.com/")
		require.NoError(t, err)
		require.Equal(t, "https://api.github.com/", got)
	})

	t.Run("rejects relative url", func(t *testing.T) {
		_, err := NormalizeAPIBaseURL("/api/v3/")
		require.Error(t, err)
	})
}

func TestNewClient(t *testing.T) {
	t.Run("default base url when empty", func(t *testing.T) {
		c, err := NewClient("t", "")
		require.NoError(t, err)
		require.Equal(t, DefaultAPIBaseURL, c.BaseURL.String())
	})

	t.Run("enterprise base url is applied when provided", func(t *testing.T) {
		c, err := NewClient("t", "https://ghe.example.com/api/v3")
		require.NoError(t, err)
		require.Equal(t, "https://ghe.example.com/api/v3/", c.BaseURL.String())
	})
}

