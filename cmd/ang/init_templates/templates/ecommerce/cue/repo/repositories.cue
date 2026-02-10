package repo

Repositories: {
	Product: {
		finders: [{name: "ListProducts", returns: "many"}]
	}
	Order: {
		finders: [{name: "ListOrders", returns: "many"}]
	}
}
