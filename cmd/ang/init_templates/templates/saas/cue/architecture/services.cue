package architecture

#Services: {
	auth: {
		name: "Auth"
		description: "Authentication and user lifecycle"
		entities: ["User", "Tenant"]
	}
	billing: {
		name: "Billing"
		description: "Subscriptions and invoices"
		entities: ["Subscription", "Invoice"]
	}
}
