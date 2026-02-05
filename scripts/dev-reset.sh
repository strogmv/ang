#!/bin/bash
set -e

echo ">>> 🛑 Stopping running services..."
fuser -k 8080/tcp || true

echo ">>> 🧹 Cleaning Database..."
export PGPASSWORD=password
psql -h localhost -p 5439 -U user -d tender_platform -c "TRUNCATE TABLE users CASCADE; TRUNCATE TABLE companys CASCADE; TRUNCATE TABLE tenders CASCADE;"

echo ">>> 🏗️ Rebuilding Server..."
export GOCACHE=/tmp/go-build
go build -o server_bin ./cmd/server

echo ">>> 🚀 Starting Server (Background)..."
export DATABASE_URL="postgres://user:password@localhost:5439/tender_platform?sslmode=disable"
# Load keys if they exist, otherwise gen
if [ -f private.pem ]; then
    export JWT_PRIVATE_KEY="$(cat private.pem)"
    export JWT_PUBLIC_KEY="$(cat public.pem)"
fi

nohup ./server_bin > server.log 2>&1 &
SERVER_PID=$!

echo ">>> ✅ Environment Reset Complete."
echo ">>> Server PID: $SERVER_PID. Logs: server.log"
echo ">>> Waiting for port 8080..."

# Wait for port 8080 to be active
timeout 10 bash -c 'until echo > /dev/tcp/localhost/8080; do sleep 0.5; done'

echo ">>> 🎉 System Ready!"
