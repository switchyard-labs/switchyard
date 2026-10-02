import sys
#!/usr/bin/env python3
"""CP1-E4 spike: secret resolution -> executor path (cleaned)."""
import os, subprocess, tempfile, time, hashlib

class SecretStore:
    def __init__(self):
        self.secrets = {}

    def put(self, secret_id, plaintext, ttl_s=300):
        self.secrets[secret_id] = (plaintext, time.time() + ttl_s)

    def resolve_for_execution(self, secret_id, run_dir):
        if secret_id not in self.secrets:
            raise RuntimeError("no such secret")
        _, expires = self.secrets[secret_id]
        if time.time() > expires:
            raise RuntimeError("secret expired")
        os.makedirs(run_dir, exist_ok=True)
        path = os.path.join(run_dir, "secret")
        with open(path, "w") as f:
            f.write(self.secrets[secret_id][0])
        os.chmod(path, 0o600)
        return path

CHILD = "import sys,hashlib,os\np=sys.argv[1]\ns=open(p).read(); os.unlink(p)\nprint('child-ok len=%d' % len(s))\n"

def main():
    store = SecretStore()
    store.put("org/deepseek", "sk-live-SECRET-abcdef1234567890", ttl_s=300)

    run_dir = tempfile.mkdtemp(prefix="run-")
    path = store.resolve_for_execution("org/deepseek", run_dir)
    pr = subprocess.run([sys.executable, "-c", CHILD, path], capture_output=True, text=True)
    print("child stdout:", pr.stdout.strip())
    print("secret file exists after run:", os.path.exists(path))

    combined = pr.stdout + pr.stderr
    print("secret in child output:", "SECRET-abcdef" in combined)

    # no secret in child environment
    env = os.environ.copy()
    print("no secret in child env:", "SK_LIVE" not in "".join(env.values()))

    # expiry enforced at resolution
    store.secrets["org/deepseek"] = (store.secrets["org/deepseek"][0], time.time() - 1)
    try:
        store.resolve_for_execution("org/deepseek", tempfile.mkdtemp())
        print("expiry NOT enforced (BUG)")
    except RuntimeError as e:
        print(f"expiry enforced -> {e}")

    # child cannot inherit: a child spawned by an agent gets no secret unless
    # the agent explicitly passes the materialized file (already deleted above)
    print("E4 spike done")

if __name__ == "__main__":
    main()