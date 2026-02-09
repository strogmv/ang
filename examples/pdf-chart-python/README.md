# PDF Chart Python Example

ANG example where generation from CUE produces a Python/FastAPI service endpoint that returns a PDF with a chart.

## Generate

```bash
go run ./cmd/ang build examples/pdf-chart-python --target=python
```

The source of truth is only `examples/pdf-chart-python/cue/*`.
`ang build` reads that CUE and regenerates `examples/pdf-chart-python/app/*` and `examples/pdf-chart-python/api/*`.
Heavy custom PDF logic is intentionally kept in `examples/pdf-chart-python/app/lib/pdf_chart.py`,
while CUE `impl` only delegates to this helper.

## Run

```bash
cd examples/pdf-chart-python/app
python3 -m venv .venv
. .venv/bin/activate
python3 -m pip install -U pip
python3 -m pip install -e .
python3 -m pip install matplotlib reportlab
uvicorn app.main:app --reload
```

## Test endpoint

```bash
curl -X POST http://127.0.0.1:8000/reports/pdf \
  -H 'Content-Type: application/json' \
  -d '{"title":"Revenue","points":[{"x":1,"y":1.2},{"x":2,"y":2.7},{"x":3,"y":2.3},{"x":4,"y":4.9}]}' \
  --output report.pdf
```

You should get a `report.pdf` with a plotted line chart.
