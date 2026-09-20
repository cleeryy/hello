#!/bin/bash
set -euo pipefail

API="${API:-http://localhost:8080}"

echo "Wake-on-LAN API test suite"

echo "1. Waiting for a healthy server"
for _ in $(seq 1 30); do
  if curl -sf "$API/health" > /dev/null; then
    break
  fi
  sleep 1
done
curl -sf "$API/health" | jq .

echo "2. Add device1 (no IP, no ping)"
curl -sf -X POST "$API/devices" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "device1",
    "name": "Mon PC",
    "mac": "00:11:22:33:44:55",
    "ping_enabled": false,
    "status": "unknown"
  }' | jq .

echo "3. Add device2 (IP + ping enabled)"
curl -sf -X POST "$API/devices" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "device2",
    "name": "Google DNS",
    "mac": "AA:BB:CC:DD:EE:FF",
    "ip": "8.8.8.8",
    "ping_enabled": true,
    "status": "unknown"
  }' | jq .

echo "4. List devices"
curl -sf "$API/devices" | jq .

echo "5. Wait for the monitor to ping"
sleep 3

echo "6. device2 should be up"
curl -sf "$API/devices/device2" | jq .

echo "7. Wake by MAC (POST canonical)"
if [ "${SKIP_WAKE:-0}" = "1" ]; then
  echo "skipped (SKIP_WAKE=1: no UDP broadcast route in this environment)"
else
  curl -sf -X POST "$API/wake/AA:BB:CC:DD:EE:FF" | jq .
fi

echo "8. Wake by ID"
if [ "${SKIP_WAKE:-0}" = "1" ]; then
  echo "skipped (SKIP_WAKE=1: no UDP broadcast route in this environment)"
else
  curl -sf -X POST "$API/devices/device2/wake" | jq .
fi

echo "9. Delete device1 (204, empty body)"
code=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$API/devices/device1")
if [ "$code" != "204" ]; then
  echo "expected 204, got $code"
  exit 1
fi
echo "deleted (204)"

echo "10. Remaining devices"
curl -sf "$API/devices" | jq .

echo "11. Create a schedule for device2"
curl -sf -X POST "$API/schedules" \
  -H "Content-Type: application/json" \
  -d '{"id":"e2e-morning","device_id":"device2","cron":"@daily"}' | jq .

echo "12. Reject a schedule for an unknown device (422)"
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/schedules" \
  -H "Content-Type: application/json" \
  -d '{"id":"e2e-ghost","device_id":"ghost","cron":"@daily"}')
if [ "$code" != "422" ]; then
  echo "expected 422, got $code"
  exit 1
fi
echo "rejected (422)"

echo "13. List schedules"
curl -sf "$API/schedules" | jq .

echo "14. Pause then delete the schedule"
curl -sf -X PUT "$API/schedules/e2e-morning" \
  -H "Content-Type: application/json" \
  -d '{"id":"e2e-morning","device_id":"device2","cron":"@daily","enabled":false}' | jq .
code=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$API/schedules/e2e-morning")
if [ "$code" != "204" ]; then
  echo "expected 204, got $code"
  exit 1
fi
echo "deleted (204)"

echo "Tests done"
