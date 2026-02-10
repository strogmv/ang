package policies

#RBAC: {
	roles: {
		admin: ["tenant.manage", "user.read", "invoice.read"]
		member: ["user.read", "invoice.read"]
	}
	permissions: {
		"tenant.manage": "Manage tenant settings"
		"user.read":     "Read users in tenant"
		"invoice.read":  "Read tenant invoices"
	}
}
