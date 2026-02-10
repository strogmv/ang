package api

ListListings: {
	service: "listing"
	output: {
		data: [...{
			id: string
			title: string
			priceCents: int
		}]
	}
	sources: {
		listings: {kind: "sql", entity: "Listing"}
	}
}

CreateListing: {
	service: "listing"
	input: {
		title: string
		priceCents: int
	}
	output: {
		ok: bool
	}
}

ListTransactions: {
	service: "transaction"
	output: {
		data: [...{
			id: string
			listingID: string
			status: string
		}]
	}
	sources: {
		transactions: {kind: "sql", entity: "Transaction"}
	}
}
