package rbac

// Roles defines the RBAC policy for the application
var Roles = map[string][]string{}

// Permissions describes available permissions (optional metadata)
var Permissions = map[string]string{}

// CheckPermission checks if a role has a specific permission
func CheckPermission(role, permission string) bool {
	perms, ok := Roles[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == "*" || p == permission {
			return true
		}
	}
	return false
}
