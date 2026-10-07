package core

// SessionMethodAllowed binds the session to the account's current identity.
// The owner retains a protected Google identity; delegated admins use their
// existing login method, without granting privileges from token claims.
func SessionMethodAllowed(user User, method, ownerEmail, pinnedOwnerSubject string) bool {
	if user.Email == ownerEmail {
		return method == "google" && user.GoogleSub != "" && user.IdentityPolicy == 1 && (user.GoogleAuthoritative || user.GoogleSub == pinnedOwnerSubject)
	}
	return (method == "google" && user.GoogleSub != "" && user.IdentityPolicy == 1) || (method == "password" && user.GoogleSub == "")
}
