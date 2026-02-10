package domain

#Product: {
	name: "Product"
	fields: {
		id: {type: "uuid"}
		sku: {type: "string"}
		name: {type: "string"}
		priceCents: {type: "int"}
		stock: {type: "int"}
	}
}

#Order: {
	name: "Order"
	fields: {
		id: {type: "uuid"}
		customerEmail: {type: "string"}
		status: {type: "string"}
		totalCents: {type: "int"}
		createdAt: {type: "time"}
	}
}

#Payment: {
	name: "Payment"
	fields: {
		id: {type: "uuid"}
		orderID: {type: "uuid"}
		status: {type: "string"}
		provider: {type: "string"}
	}
}
