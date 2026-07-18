package project

// This sample emits into targets[].output_dir, so declare release mode explicitly.
build: mode: "release"

// architecture_mode relaxed: cross-service access warnings are non-fatal (sample project only).
#Project: {
	architecture_mode: "relaxed"
}

state: {
	target: {
		lang:      "go"
		framework: "chi"
		db:        "postgres"
	}

	// Optional multi-target build matrix. `ang build` will generate all targets.
	// You can select one or many via `ang build --target=name`.
	targets: [{
		name:       "go"
		lang:       "go"
		framework:  "chi"
		db:         "postgres"
		output_dir: "dist/release/go-service"
	}]
}
