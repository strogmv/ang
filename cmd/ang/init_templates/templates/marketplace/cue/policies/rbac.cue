package policies

#RBAC: {
	roles: {
		admin:  ["listing.manage", "transaction.read"]
		seller: ["listing.manage", "transaction.read"]
		buyer:  ["listing.read", "transaction.read"]
	}
	permissions: {
		"listing.manage":   "Create/update listings"
		"listing.read":     "Read listings"
		"transaction.read": "Read transactions"
	}
}
