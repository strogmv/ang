package project

// architecture_mode relaxed: cross-service access warnings are non-fatal (sample project only).
#Project: {
	architecture_mode: "relaxed"
}

// The sample generates in place: its own go.mod owns internal/, api/, db/ and sdk/.
state: {
	target: {
		lang:      "go"
		framework: "chi"
		db:        "postgres"
	}
}
