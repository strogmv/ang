package project

state: {
	target: {
		lang:      "go"
		framework: "chi"
		db:        "postgres"
	}

	// Optional multi-target build matrix. `ang build` will generate all targets.
	// You can select one or many via `ang build --target=name` or `--target=python`.
	targets: [{
		name:       "go"
		lang:       "go"
		framework:  "chi"
		db:         "postgres"
		output_dir: "dist/release/go-service"
	}, {
		name:       "python"
		lang:       "python"
		framework:  "fastapi"
		db:         "postgres"
		output_dir: "dist/release/python-service"
	}]
}
