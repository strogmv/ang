package domain

#Listing: {
	name: "Listing"
	fields: {
		id: {type: "uuid"}
		sellerID: {type: "uuid"}
		title: {type: "string"}
		priceCents: {type: "int"}
		status: {type: "string"}
	}
}

#Offer: {
	name: "Offer"
	fields: {
		id: {type: "uuid"}
		listingID: {type: "uuid"}
		buyerID: {type: "uuid"}
		amountCents: {type: "int"}
		status: {type: "string"}
	}
}

#Transaction: {
	name: "Transaction"
	fields: {
		id: {type: "uuid"}
		listingID: {type: "uuid"}
		buyerID: {type: "uuid"}
		sellerID: {type: "uuid"}
		status: {type: "string"}
		createdAt: {type: "time"}
	}
}
