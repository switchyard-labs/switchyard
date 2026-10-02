// Research-only round-trip Worker for Checkpoint 0.
// NOT the beginning of the Switchyard API. Used to prove the Git-plane loop:
//   create/fork repo -> mint token -> (git client pushes) -> event -> Trestle.
export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    // POST /repos?name=...  — create a repo, return remote + initial token
    if (request.method === "POST" && url.pathname === "/repos") {
      const name = url.searchParams.get("name") || "cp0-baseline";
      const created = await env.ARTIFACTS.create(name, {
        description: "switchyard checkpoint 0 round trip",
        setDefaultBranch: "main",
      });
      return Response.json({
        name: created.name,
        remote: created.remote,
        defaultBranch: created.defaultBranch,
        token: created.token,
      });
    }

    // POST /fork?name=...&from=...  — fork a repo (agent task workspace)
    if (request.method === "POST" && url.pathname === "/fork") {
      const from = url.searchParams.get("from");
      const name = url.searchParams.get("name") || "cp0-task";
      using repo = await env.ARTIFACTS.get(from);
      const forked = await repo.fork(name, { defaultBranchOnly: true });
      return Response.json({ name: forked.name, remote: forked.remote, token: forked.token });
    }

    // POST /tokens?repo=...&scope=read|write&ttl=... — short-lived git token
    if (request.method === "POST" && url.pathname === "/tokens") {
      const repoName = url.searchParams.get("repo");
      const scope = url.searchParams.get("scope") || "write";
      const ttl = Number(url.searchParams.get("ttl") || 3600);
      using repo = await env.ARTIFACTS.get(repoName);
      const token = await repo.createToken(scope, ttl);
      return Response.json({ id: token.id, plaintext: token.plaintext, expiresAt: token.expiresAt });
    }

    // POST /ingest — forward a normalized pushed event to Trestle (research bridge).
    // In production this is driven by a Queues event subscription on
    // cf.artifacts.repo.pushed instead of an inbound POST.
    if (request.method === "POST" && url.pathname === "/ingest") {
      const event = await request.json();
      const normalized = normalize(event);
      const resp = await fetch(`${env.TRESTLE_BASE_URL}/api/v1/...`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${env.TRESTLE_SERVICE_TOKEN}`,
        },
        body: JSON.stringify(normalized),
      });
      return new Response(await resp.text(), { status: resp.status });
    }

    return Response.json({ message: "round-trip worker" }, { status: 404 });
  },
};

// Normalize a cf.artifacts.repo.pushed event into the Trestle ingestion shape
// that the checkpoint-1 coordination slice will define. Research shape only.
function normalize(event) {
  return {
    source: "artifacts",
    namespace: event.source?.namespace,
    repoName: event.source?.repoName,
    ref: event.payload?.ref,
    before: event.payload?.before,
    after: event.payload?.after,
    commits: (event.payload?.commits || []).map((c) => ({
      id: c.id,
      message: c.message,
      author: c.author?.email,
      timestamp: c.timestamp,
    })),
    totalCommitsCount: event.payload?.totalCommitsCount,
    occurredAt: event.metadata?.eventTimestamp,
  };
}