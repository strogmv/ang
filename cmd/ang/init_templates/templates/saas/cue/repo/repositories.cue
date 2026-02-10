package repo

Repositories: {
	User: {
		finders: [{name: "ListUsers", returns: "many"}]
	}
	Invoice: {
		finders: [{name: "ListInvoices", returns: "many"}]
	}
}
