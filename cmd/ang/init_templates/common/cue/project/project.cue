package project

state: {
	target: {
		lang:      "{{LANG}}"
		framework: "{{FRAMEWORK}}"
		db:        "{{DB}}"
	}

	targets: [{
		name:       "{{LANG}}"
		lang:       "{{LANG}}"
		framework:  "{{FRAMEWORK}}"
		db:         "{{DB}}"
		output_dir: "dist/release/{{LANG}}-service"
	}]
}
