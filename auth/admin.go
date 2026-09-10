package auth

import (
	"os"
	"slices"
	"strings"
)

// defaultAdminEmails is the built-in admin list, used when ADMIN_EMAILS is unset.
var defaultAdminEmails = []string{
	"simon.bundgaard-egeberg@it-minds.dk",
}

// AdminEmails returns the lower-cased list of admin emails. Set ADMIN_EMAILS to a
// comma-separated list to override the built-in list without a redeploy of the code.
func AdminEmails() []string {
	raw := os.Getenv("ADMIN_EMAILS")
	if strings.TrimSpace(raw) == "" {
		emails := make([]string, len(defaultAdminEmails))
		for i, e := range defaultAdminEmails {
			emails[i] = strings.ToLower(e)
		}
		return emails
	}

	var emails []string
	for _, e := range strings.Split(raw, ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			emails = append(emails, e)
		}
	}
	return emails
}

// IsAdmin reports whether the given email belongs to an admin.
func IsAdmin(email string) bool {
	return slices.Contains(AdminEmails(), strings.ToLower(strings.TrimSpace(email)))
}
