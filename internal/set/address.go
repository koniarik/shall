package set

import (
	"net/url"
	"strings"
)

// Credentials tells whether the git address of a dependency carries
// credentials: a URL with a password, or an http or https URL with a user,
// which is often a token.
func Credentials(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.User == nil {
		return false
	}
	_, password := u.User.Password()
	return password || u.Scheme == "http" || u.Scheme == "https"
}

// Redacted returns a git address with the credentials it carries, if any,
// replaced by ***.
func Redacted(address string) string {
	if !Credentials(address) {
		return address
	}
	u, _ := url.Parse(address)
	u.User = nil
	return u.Scheme + "://***@" + strings.TrimPrefix(u.String(), u.Scheme+"://")
}
