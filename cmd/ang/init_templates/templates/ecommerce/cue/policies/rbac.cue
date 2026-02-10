package policies

#RBAC: {
	roles: {
		admin:    ["product.manage", "order.read", "order.manage"]
		operator: ["order.read", "order.manage"]
	}
	permissions: {
		"product.manage": "Manage product catalog"
		"order.read":     "Read orders"
		"order.manage":   "Create/update orders"
	}
}
