#!/bin/bash

API="http://localhost:8080"

echo "🌙 Wake-on-LAN API Test Suite\n"

# 1. Health Check
echo "✅ 1. Health Check"
curl -s "$API/" | jq .

# 2. Ajouter device1 SANS IP (pas de ping)
echo "✅ 2. Ajouter device1 (SANS IP, pas de ping)"
curl -s -X POST "$API/devices" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "device1",
    "name": "Mon PC",
    "mac": "00:11:22:33:44:55",
    "ping_enabled": false,
    "status": "unknown"
  }' | jq .

# 3. Ajouter device2 AVEC IP ET PING ACTIVÉ
echo "✅ 3. Ajouter device2 (AVEC IP + PING ACTIVÉ)"
curl -s -X POST "$API/devices" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "device2",
    "name": "Google DNS",
    "mac": "AA:BB:CC:DD:EE:FF",
    "ip": "8.8.8.8",
    "ping_enabled": true,
    "status": "unknown"
  }' | jq .

# 4. Ajouter device3 AVEC IP (pour comparer)
echo "✅ 4. Ajouter device3 (AVEC IP mais PING DÉSACTIVÉ)"
curl -s -X POST "$API/devices" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "device3",
    "name": "Cloudflare",
    "mac": "11:22:33:44:55:66",
    "ip": "1.1.1.1",
    "ping_enabled": false,
    "status": "unknown"
  }' | jq .

# 5. Récupérer tous les devices
echo "✅ 5. Récupérer tous les devices"
curl -s "$API/devices" | jq .

# 6. Attendre 3 secondes pour que le monitor fasse un ping
echo "⏳ Attendre 3 secondes pour que le monitor fasse un ping..."
sleep 3

# 7. Vérifier le statut de device2 (devrait être "up")
echo "✅ 6. Vérifier device2 (devrait être UP)"
curl -s "$API/devices/device2" | jq .

# 8. Vérifier device3 (devrait rester "unknown" car ping désactivé)
echo "✅ 7. Vérifier device3 (devrait rester UNKNOWN - ping désactivé)"
curl -s "$API/devices/device3" | jq .

# 9. Tester Wake endpoint
echo "✅ 8. Wake device2"
curl -s "$API/wake/AA:BB:CC:DD:EE:FF" | jq .

# 10. Supprimer device1
echo "✅ 9. Supprimer device1"
curl -s -X DELETE "$API/devices/device1" | jq .

# 11. Vérifier les devices restants
echo "✅ 10. Vérifier les devices restants"
curl -s "$API/devices" | jq .

echo "\n✅ Tests terminés!"

