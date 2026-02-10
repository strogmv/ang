#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BENCH_DIR="$ROOT_DIR/benchmarks/three_iteration_marketplace"
OUT_DIR="$BENCH_DIR/output"
PROJECT_DIR="$OUT_DIR/workdir"
ANG_BIN="${ANG_BIN:-$ROOT_DIR/bin/ang}"

run_build() {
  local log_file="$1"
  (
    cd "$PROJECT_DIR"
    "$ANG_BIN" build --target=go > "$log_file" 2>&1 || true
  )
  if grep -q "Build SUCCESSFUL." "$log_file"; then
    echo "SUCCESS"
  else
    echo "FAIL"
  fi
}

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

if [[ ! -x "$ANG_BIN" ]]; then
  echo "ANG binary not found at $ANG_BIN" >&2
  echo "Build it first: make build" >&2
  exit 1
fi

echo "[1/6] Init marketplace template"
"$ANG_BIN" init "$PROJECT_DIR" --template marketplace --lang go --db postgres

# Iteration 1: intentionally invalid FSM (missing 'paid' in states)
echo "[2/6] Iteration 1 patch (intentional FSM error)"
cat > "$PROJECT_DIR/cue/domain/order.cue" <<'CUE'
package domain

#Order: {
	name: "Order"
	fields: {
		id: {type: "uuid"}
		buyerID: {type: "uuid"}
		status: {type: "string"}
		totalCents: {type: "int"}
	}
	fsm: {
		field: "status"
		states: ["draft", "shipped", "delivered"]
		transitions: [
			{from: "draft", to: "paid"},
			{from: "paid", to: "shipped"},
			{from: "shipped", to: "delivered"},
		]
	}
}
CUE

cat > "$PROJECT_DIR/cue/api/orders.cue" <<'CUE'
package api

CreateOrder: {
	service: "transaction"
	input: {
		buyerID: string
	}
	output: {
		ok: bool
	}
}

ConfirmPayment: {
	service: "transaction"
	input: {
		orderID: string
		providerEventID: string
	}
	output: {
		ok: bool
	}
}
CUE

cat >> "$PROJECT_DIR/cue/api/http.cue" <<'CUE'

HTTP: {
	CreateOrder: {
		method: "POST"
		path:   "/orders"
	}
	ConfirmPayment: {
		method: "POST"
		path:   "/webhooks/stripe"
	}
}
CUE

iter1_status="$(run_build "$OUT_DIR/iter1.build.log")"
iter1_fsm_code="no"
if grep -q "E_FSM_UNDEFINED_STATE" "$OUT_DIR/iter1.build.log"; then
  iter1_fsm_code="yes"
fi

# Iteration 2: fix FSM
echo "[3/6] Iteration 2 patch (FSM fix)"
sed -i 's/states: \["draft", "shipped", "delivered"\]/states: ["draft", "paid", "shipped", "delivered"]/' "$PROJECT_DIR/cue/domain/order.cue"
sed -i '/__force_error__/d' "$PROJECT_DIR/cue/domain/order.cue"
cat > "$PROJECT_DIR/cue/api/orders.cue" <<'CUE'
package api

CreateOrder: {
	service: "transaction"
	input: {
		buyerID: string
	}
	output: {
		ok: bool
	}
}

ConfirmPayment: {
	service: "transaction"
	input: {
		orderID: string
		providerEventID: string
	}
	output: {
		ok: bool
	}
}
CUE

iter2_status="$(run_build "$OUT_DIR/iter2.build.log")"

# Iteration 3: add business flow skeleton + event publish
echo "[4/6] Iteration 3 patch (business flow)"
cat > "$PROJECT_DIR/cue/api/orders.cue" <<'CUE'
package api

CreateOrder: {
	service: "transaction"
	description: "Create draft order"
	input: {
		buyerID: string
		totalCents: int
	}
	output: {
		ok: bool
	}
}

ConfirmPayment: {
	service: "transaction"
	description: "Confirm Stripe payment webhook"
	input: {
		orderID: string
		providerEventID: string
	}
	output: {
		ok: bool
	}
}
CUE

cat > "$PROJECT_DIR/cue/architecture/services.cue" <<'CUE'
package architecture

#Services: {
	listing: {
		name: "Listing"
		entities: ["Listing"]
	}
	transaction: {
		name: "Transaction"
		entities: ["Offer", "Transaction", "Order"]
		publishes: ["OrderPaid"]
	}
	notifications: {
		name: "Notifications"
		description: "Email notifications"
		subscribes: {
			OrderPaid: "NotifySeller"
		}
	}
}
CUE

iter3_status="$(run_build "$OUT_DIR/iter3.build.log")"

# Report
echo "[5/6] Build benchmark summary"
final_count=0
if [[ -d "$PROJECT_DIR/dist/release/go-service" ]]; then
  final_count=$(find "$PROJECT_DIR/dist/release/go-service" -type f | wc -l | tr -d ' ')
fi

cat > "$OUT_DIR/summary.md" <<MD
# 3-Iteration Backend Benchmark Summary

- Project: $PROJECT_DIR
- Iteration 1 build: **$iter1_status** (expected FAIL)
- Iteration 1 contains E_FSM_UNDEFINED_STATE: **$iter1_fsm_code** (expected yes)
- Iteration 2 build: **$iter2_status**
- Iteration 3 build: **$iter3_status**
- Generated files in final artifact (dist/release/go-service): **$final_count**

## Log Files

- iter1.build.log
- iter2.build.log
- iter3.build.log
MD

echo "[6/6] Done: $OUT_DIR/summary.md"
