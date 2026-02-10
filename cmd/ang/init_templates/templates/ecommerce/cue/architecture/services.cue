package architecture

#Services: {
	catalog: {
		name: "Catalog"
		entities: ["Product"]
	}
	order: {
		name: "Order"
		entities: ["Order", "Payment"]
	}
}
