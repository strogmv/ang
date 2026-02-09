# PDF Chart Python Example

ANG example where generation from CUE produces a Python/FastAPI service endpoint that returns a PDF with a chart.

## Generate

```bash
go run ./cmd/ang build examples/pdf-chart-python --backend examples/pdf-chart-python --frontend examples/pdf-chart-python/sdk
```

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
