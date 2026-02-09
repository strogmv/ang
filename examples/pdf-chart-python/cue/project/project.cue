package project

#Project: {
  name: "PDF Chart Service"
  version: "0.1.0"
}

#Target: {
  name:       "python-pdf"
  lang:       "python"
  framework:  "fastapi"
  db:         "postgres"
  output_dir: "examples/pdf-chart-python"
}

state: {
  target: #Target
  targets: [#Target]
}
