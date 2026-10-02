#!/usr/bin/env python3
"""CP1-E2 spike: recoverable transient draft semantics.

Disposable experiment. Hypothesis:
  - Save synchronously persists a draft revision (manifest + content-addressed
    blobs), reload/multi-device reconstruct the same working set, and an Agent
    apply against an explicit base revision merges cleanly or surfaces
    contention -- with no version history, no Git involvement.
This spike uses a filesystem-backed manifest+blob store as a stand-in for the
planned Trestle manifest + object-store blobs. It measures save latency only.
"""
import hashlib, json, os, subprocess, tempfile, time

class DraftStore:
    def __init__(self, root):
        self.root = root
        self.blobs = os.path.join(root, "blobs")
        self.manifest = os.path.join(root, "manifest.json")
        os.makedirs(self.blobs, exist_ok=True)
        self.state = {"sessions": {}}
        if os.path.exists(self.manifest):
            self.state = json.load(open(self.manifest))

    def _persist_manifest(self):
        json.dump(self.state, open(self.manifest, "w"), indent=1)

    def _put_blob(self, content):
        h = hashlib.sha256(content.encode()).hexdigest()
        with open(os.path.join(self.blobs, h), "w") as f:
            f.write(content)
        return h

    def _get_blob(self, ref):
        with open(os.path.join(self.blobs, ref)) as f:
            return f.read()

    def new_session(self, session, base_sha, ttl_s=3600):
        self.state["sessions"][session] = {
            "base_sha": base_sha, "revision": 0, "expires_at": time.time() + ttl_s,
            "files": {},
        }
        self._persist_manifest()

    def _session(self, session):
        s = self.state["sessions"][session]
        if time.time() > s["expires_at"]:
            raise RuntimeError("draft expired")
        return s

    def save(self, session, path, content, mutator, expected_rev=None):
        """Human/agent save with per-session optimistic CAS."""
        s = self._session(session)
        if expected_rev is not None and s["revision"] != expected_rev:
            return ("STALE", s["revision"])
        ref = self._put_blob(content)
        s["files"][path] = {"ref": ref, "mutator": mutator, "rev": s["revision"]}
        s["revision"] += 1
        self._persist_manifest()
        return ("OK", s["revision"])

    def load(self, session):
        s = self._session(session)
        return {
            "base_sha": s["base_sha"], "revision": s["revision"],
            "files": {p: {"content": self._get_blob(f["ref"]), "mutator": f["mutator"], "rev": f["rev"]}
                      for p, f in s["files"].items()},
        }

def merge3(base, ours, theirs):
    """Three-way merge via git merge-file. Returns (content, conflicts_bool)."""
    d = tempfile.mkdtemp()
    paths = {k: os.path.join(d, k) for k in ("base", "ours", "theirs")}
    for k, v in (("base", base), ("ours", ours), ("theirs", theirs)):
        open(paths[k], "w").write(v)
    p = subprocess.run(["git", "merge-file", "-p", paths["ours"], paths["base"], paths["theirs"]],
                       capture_output=True, text=True)
    return (p.stdout, p.returncode != 0)

def main():
    root = tempfile.mkdtemp(prefix="cp1-e2-")
    store = DraftStore(root)
    store.new_session("sess-1", "base-sha-abc", ttl_s=60)

    # Human edits, saves (measuring sync latency)
    t0 = time.perf_counter()
    r1 = store.save("sess-1", "src/auth.p", "line1\nline2\nline3\n", "human:nick")
    lat = (time.perf_counter() - t0) * 1000
    print(f"save#1 -> {r1[0]} rev={r1[1]}  persist_latency_ms={lat:.2f}")

    # Autosave (debounced) of a second edit
    store.save("sess-1", "src/auth.p", "line1\nline2\nHUMAN-edit\n", "human:nick")
    rev_after_human = store.state["sessions"]["sess-1"]["revision"]

    # Reload (browser crash) -> reconstruct
    store2 = DraftStore(root)  # fresh instance = reload
    loaded = store2.load("sess-1")
    print(f"reload -> rev={loaded['revision']} files={list(loaded['files'])} content_end={loaded['files']['src/auth.p']['content'].splitlines()[-1]}")
    assert loaded["files"]["src/auth.p"]["content"].endswith("HUMAN-edit\n")

    # Multi-device: third client sees same state
    store3 = DraftStore(root)
    print(f"device2 -> rev={store3.load('sess-1')['revision']}")

    # Agent apply against explicit base revision
    agent_base = rev_after_human - 1   # stale by one save
    stale = store.save("sess-1", "src/auth.p", "line1\nAGENT-edit\nline3\n", "agent:exec-1", expected_rev=agent_base)
    print(f"agent stale save -> {stale[0]} (current rev {stale[1]})")

    # Correct protocol: agent returns patch against base; control plane merges
    base_content = "line1\nline2\nline3\n"
    human_now = "line1\nline2\nHUMAN-edit\nline3\n" if False else loaded["files"]["src/auth.p"]["content"]
    # rebuild: human content after 2 saves:
    human_now = store.load("sess-1")["files"]["src/auth.p"]["content"]
    agent_patch_base = "line1\nline2\nline3\n"
    agent_edit = "line1\nAGENT-edit\nline3\n"
    merged, conflicted = merge3(agent_patch_base, human_now, agent_edit)
    print(f"merge(different lines) conflicted={conflicted}\n  {merged!r}")

    # True overlap: both edit line 2
    agent_edit_overlap = "line1\nAGENT-edit\nline3\n"
    human_overlap = "line1\nHUMAN-edit\nline3\n"
    m2, c2 = merge3("line1\nline2\nline3\n", human_overlap, agent_edit_overlap)
    print(f"merge(overlap line2) conflicted={c2} (markers present: {'<<<<<<<' in m2})")

    # Expiry
    store.state["sessions"]["sess-1"]["expires_at"] = time.time() - 1
    try:
        store.load("sess-1")
        print("expiry NOT enforced (BUG)")
    except RuntimeError as e:
        print(f"expiry enforced -> {e}")

    print("E2 spike done")

if __name__ == "__main__":
    main()