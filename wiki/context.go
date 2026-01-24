package wiki

// ContextKey is a type for context keys used in the wiki package.
type ContextKey string

// UserKey is the context key for storing the current user.
const UserKey ContextKey = "periwiki.user"
