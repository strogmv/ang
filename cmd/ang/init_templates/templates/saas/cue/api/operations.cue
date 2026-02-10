package api

RegisterTenant: {
	service: "auth"
	input: {
		tenantName: string
		adminEmail: string
	}
	output: {
		ok: bool
	}
}

ListUsers: {
	service: "auth"
	input: {
		limit?: int
		offset?: int
	}
	output: {
		data: [...{
			id: string
			email: string
			role: string
		}]
	}
	sources: {
		users: {
			kind: "sql"
			entity: "User"
		}
	}
}

ListInvoices: {
	service: "billing"
	input: {
		tenantID: string
	}
	output: {
		data: [...{
			id: string
			status: string
			totalCents: int
		}]
	}
	sources: {
		invoices: {
			kind: "sql"
			entity: "Invoice"
		}
	}
}
