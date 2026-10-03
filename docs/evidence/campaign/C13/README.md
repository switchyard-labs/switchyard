# C13 — captured Actions output

The screenshot shows the actual native Artifacts push run `11a042fc-df57-4acb-b9d4-4ffccff88fbc`, source `b4e3fe7f7d3e76931b0f7a9dbdabc248c10cb402`, through the isolated Go/Trestle application and signed Cloudflare provider. Browser checks observed 17 lines, search narrowed those to the SHA line, reconnect retained exactly 17 lines, and capture-time display showed the actual step completion time.

Read-only ANSI SGR rendering follows Warden's foreground/bold style model. OSC links/control sequences are discarded and printable output is inserted as text nodes. No stdin, PTY, prompt or browser command execution is added. Gantry's terminal package handles session/scrollback validation rather than ANSI rendering; the editor adaptation remains C20.

SSE supports cursor resume and stops on disconnect, terminal capture or a bounded wait. API pages cap at 100 lines/64 KiB; the viewer caps at 10,000 lines and 8 KiB per line. Provider capture caps stdout and stderr at 1 MiB each. Logs remain in Cloudflare R2. Step capture time is available; per-line timestamps and original stdout/stderr interleaving are unavailable and are explicitly described in the UI. This is captured step output, not a claim of live subprocess streaming.

Validation: Actions/application race suites, log scope/SHA/resume/download/bounds regressions and ANSI injection/control tests passed. Full six-width/product browser certification remains C22.
