package domain

#Tenant: {
	name: "Tenant"
	fields: {
		id: {type: "uuid"}
		name: {type: "string"}
		plan: {type: "string"}
		createdAt: {type: "time"}
	}
}

#User: {
	name: "User"
	fields: {
		id: {type: "uuid"}
		tenantID: {type: "uuid"}
		email: {type: "string"}
		role: {type: "string"}
		createdAt: {type: "time"}
	}
}

#Subscription: {
	name: "Subscription"
	fields: {
		id: {type: "uuid"}
		tenantID: {type: "uuid"}
		status: {type: "string"}
		priceCents: {type: "int"}
	}
}

#Invoice: {
	name: "Invoice"
	fields: {
		id: {type: "uuid"}
		tenantID: {type: "uuid"}
		status: {type: "string"}
		totalCents: {type: "int"}
		issuedAt: {type: "time"}
	}
}
