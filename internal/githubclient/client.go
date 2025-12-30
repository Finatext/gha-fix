package githubclient

import (
	"net/url"
	"strings"

	"github.com/cockroachdb/errors"
	gogithub "github.com/google/go-github/v72/github"
)

const DefaultAPIBaseURL = "https://api.github.com/"

// NormalizeAPIBaseURL normalizes a GitHub API base URL ensuring it has a trailing slash
// and is a valid absolute URL.
//
// The input is expected to be a full API base URL, e.g.
//   - https://github.enterprise.company.com/api/v3/
//   - https://api.github.com/
func NormalizeAPIBaseURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if !strings.HasSuffix(s, "/") {
		s += "/"
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", errors.Wrap(err, "parse api server url")
	}
	if !u.IsAbs() {
		return "", errors.New("api server url must be absolute")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", errors.New("api server url scheme must be http or https")
	}

	return u.String(), nil
}

// NewClient creates a go-github client using the provided auth token and API base URL.
//
// apiBaseURL is a full API base URL. If empty, DefaultAPIBaseURL is used.
func NewClient(token string, apiBaseURL string) (*gogithub.Client, error) {
	base := apiBaseURL
	if strings.TrimSpace(base) == "" {
		base = DefaultAPIBaseURL
	}

	base, err := NormalizeAPIBaseURL(base)
	if err != nil {
		return nil, err
	}

	// go-github uses BaseURL for API requests and UploadURL for uploads.
	// We only need API requests for this tool, but WithEnterpriseURLs sets both consistently.
	c := gogithub.NewClient(nil).WithAuthToken(token)

	if base != DefaultAPIBaseURL {
		c, err = c.WithEnterpriseURLs(base, base)
		if err != nil {
			return nil, errors.Wrap(err, "set enterprise urls")
		}
	}

	return c, nil
}

