export PATH=$PATH:/usr/local/go/bin
TRESTLE=/opt/cp0/trestle-v015
export TRESTLE_REPLICATION_INSECURE_PLAINTEXT=true
ADMIN_USER=admin
if [ -f /opt/cp0/repro/.adminpass ]; then ADMIN_PASS=$(cat /opt/cp0/repro/.adminpass); else ADMIN_PASS=$(openssl rand -hex 24); echo -n "$ADMIN_PASS" > /opt/cp0/repro/.adminpass; chmod 600 /opt/cp0/repro/.adminpass; fi
setup_node() { local dir=$1 email=$2; mkdir -p "$dir"; env -u TRESTLE_REPLICATION_ENABLED -u TRESTLE_REPLICATION_LISTEN -u TRESTLE_REPLICATION_NODE_ID -u TRESTLE_REPLICATION_BOOTSTRAP TRESTLE_DATA_DIR="$dir" $TRESTLE setup --email "$email" --username $ADMIN_USER --password-file /opt/cp0/repro/.adminpass >"$dir/setup.log" 2>&1 || true; }
login() { local base=$1 jar=$2; curl -s -c "$jar" -X POST "$base/admin/v1/session" -H "Content-Type: application/json" -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" >/dev/null; curl -s -b "$jar" "$base/admin/v1/session" | jq -r ".csrfToken"; }
admin_post() { curl -s -b "$2" -X POST "$1$4" -H "Content-Type: application/json" -H "X-Trestle-CSRF: $3" -d "$5"; }
app_post() { curl -s -b "$2" -X POST "$1$4" -H "Content-Type: application/json" -H "X-Trestle-CSRF: $3" -d "$5"; }
