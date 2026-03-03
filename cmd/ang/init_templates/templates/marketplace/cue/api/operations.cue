package api

import "github.com/strogmv/ang/cue/schema"

ListListings: schema.#Operation & {
	service: "listing"
	output: {
		data: [...{
			id: string
			title: string
			priceCents: int
		}]
	}
	flow: [
		{action: "repo.List", source: "Listing", output: "items"},
		{action: "mapping.Assign", to: "resp.data", value: "items"},
	]
}

CreateListing: schema.#Operation & {
	service: "listing"
	input: {
		title: string
		priceCents: int
		sellerID:   string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "mapping.Map", output: "newListing", entity: "Listing"},
		{action: "mapping.Assign", to: "newListing.title", value: "req.title"},
		{action: "mapping.Assign", to: "newListing.priceCents", value: "req.priceCents"},
		{action: "mapping.Assign", to: "newListing.sellerID", value: "req.sellerID"},
		{action: "mapping.Assign", to: "newListing.status", value: "\"active\""},
		{action: "repo.Save", source: "Listing", input: "newListing"},
		{action: "mapping.Assign", to: "resp.ok", value: "true"},
	]
}

PurchaseListing: schema.#Operation & {
	service: "listing"
	input: {
		listingID: string
		buyerID:   string
	}
	output: {
		ok: bool
	}
	flow: [
		{action: "flow.Tag", name: "\"listing_id\"", value: "req.listingID"},
		{action: "flow.Tag", name: "\"buyer_id\"", value: "req.buyerID"},

		{action: "flow.Saga", do: [
			{action: "tx.Block", do: [
				// 1. Lock listing
				{action: "db.Lock", source: "Listing", input: "req.listingID", output: "listing"},
				{action: "flow.Validate", condition: "listing.status == \"active\"", throw: "Listing is not active"},
			]},

			// 2. Set processing state
			{action: "state.Set", key: "\"purchase:\" + req.listingID", value: "\"processing\""},
			{action: "flow.Compensate", do: [
				{action: "state.Delete", key: "\"purchase:\" + req.listingID"},
			]},

			// 3. Parallel tasks
			{action: "flow.Parallel", branches: {
				updateListing: [
					{action: "mapping.Assign", to: "listing.status", value: "\"sold\""},
					{action: "repo.Save", source: "Listing", input: "listing"},
				]
				createTx: [
					{action: "mapping.Map", output: "tx", entity: "Transaction"},
					{action: "mapping.Assign", to: "tx.listingID", value: "req.listingID"},
					{action: "mapping.Assign", to: "tx.buyerID", value: "req.buyerID"},
					{action: "mapping.Assign", to: "tx.sellerID", value: "listing.sellerID"},
					{action: "mapping.Assign", to: "tx.status", value: "\"completed\""},
					{action: "repo.Save", source: "Transaction", input: "tx"},
				]
			}}
		]},

		{action: "mapping.Assign", to: "resp.ok", value: "true"},
	]
}

ListTransactions: schema.#Operation & {
	service: "transaction"
	output: {
		data: [...{
			id: string
			listingID: string
			status: string
		}]
	}
	flow: [
		{action: "repo.List", source: "Transaction", output: "items"},
		{action: "mapping.Assign", to: "resp.data", value: "items"},
	]
}
