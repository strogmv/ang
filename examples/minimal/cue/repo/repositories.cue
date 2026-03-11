package repo

Repositories: {
	User: {
		finders: [
			{
				name: "List"
				returns: "many"
				order_by: "name ASC"
			},
		]
	}
}
