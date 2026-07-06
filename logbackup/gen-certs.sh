#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CERT_DIR="$SCRIPT_DIR/certs"
TMP_DIR="$SCRIPT_DIR/.tmp_certs"
mkdir -p "$CERT_DIR" "$TMP_DIR"

# 1. CA
cat > "$TMP_DIR/ca.cnf" <<'EOF'
[req]
default_bits = 4096
prompt = no
default_md = sha256
x509_extensions = v3_ca
distinguished_name = dn

[dn]
C = CN
ST = Beijing
L = Beijing
O = LogBackup
CN = LogBackup CA

[v3_ca]
basicConstraints = critical,CA:TRUE
keyUsage = critical,keyCertSign,cRLSign
EOF

openssl req -new -x509 -days 3650 -newkey rsa:4096 -nodes \
  -keyout "$CERT_DIR/ca.key" \
  -out "$CERT_DIR/ca.crt" \
  -config "$TMP_DIR/ca.cnf"

# 2. Server
cat > "$TMP_DIR/server.cnf" <<'EOF'
[req]
default_bits = 2048
prompt = no
default_md = sha256
req_extensions = v3_req
distinguished_name = dn

[dn]
C = CN
ST = Beijing
L = Beijing
O = LogBackup
CN = log-server

[v3_req]
subjectAltName = @alt_names

[alt_names]
IP.1 = 127.0.0.1
IP.2 = 132.232.231.41
EOF

openssl req -new -newkey rsa:2048 -nodes \
  -keyout "$CERT_DIR/server.key" \
  -out "$CERT_DIR/server.csr" \
  -config "$TMP_DIR/server.cnf"

openssl x509 -req -days 3650 \
  -in "$CERT_DIR/server.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
  -out "$CERT_DIR/server.crt" \
  -extfile "$TMP_DIR/server.cnf" -extensions v3_req

# 3. Client
cat > "$TMP_DIR/client.cnf" <<'EOF'
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn

[dn]
C = CN
ST = Beijing
L = Beijing
O = LogBackup
CN = network-device-01
EOF

openssl req -new -newkey rsa:2048 -nodes \
  -keyout "$CERT_DIR/client.key" \
  -out "$CERT_DIR/client.csr" \
  -config "$TMP_DIR/client.cnf"

openssl x509 -req -days 3650 \
  -in "$CERT_DIR/client.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
  -out "$CERT_DIR/client.crt"

rm -rf "$TMP_DIR"
rm -f "$CERT_DIR"/*.srl

echo "=== Certificates generated ==="
echo "CA:     $CERT_DIR/ca.crt"
echo "Server: $CERT_DIR/server.crt + server.key"
echo "Client: $CERT_DIR/client.crt + client.key"
openssl x509 -in "$CERT_DIR/ca.crt" -noout -subject -dates
openssl x509 -in "$CERT_DIR/server.crt" -noout -subject -dates
openssl x509 -in "$CERT_DIR/client.crt" -noout -subject -dates
