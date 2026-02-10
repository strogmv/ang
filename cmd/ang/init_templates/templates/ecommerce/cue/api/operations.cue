package api

ListProducts: {
	service: "catalog"
	output: {
		data: [...{
			id: string
			sku: string
			name: string
			priceCents: int
		}]
	}
	sources: {
		products: {kind: "sql", entity: "Product"}
	}
}

CreateOrder: {
	service: "order"
	input: {
		customerEmail: string
	}
	output: {
		ok: bool
	}
}

ListOrders: {
	service: "order"
	output: {
		data: [...{
			id: string
			status: string
			totalCents: int
		}]
	}
	sources: {
		orders: {kind: "sql", entity: "Order"}
	}
}
