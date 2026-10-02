function main() -> int : (ExecError, Error) {
    r := exec("sh", ["-c", "echo out-line; echo err-line 1>&2; exit 7"]);
    if (r.exit_code != 7) { print("unexpected exit ", r.exit_code); return 1; }
    if (r.stdout != "out-line\n") { print("missing stdout"); return 2; }
    if (r.stderr != "err-line\n") { print("missing stderr"); return 3; }
    child := process("sh", ["-c", "printf ready; sleep 0.2; printf done"]);
    first := child.out.read(5);
    rest := child.out.read_all();
    code := child.wait();
    if (first != "ready" || rest != "done" || code != 0) { print("process streaming mismatch"); return 4; }
    string[] noargs := [];
    piped := pipe_exec("echo", ["pipe-a"], "cat", noargs);
    if (piped.exit_code != 0 || piped.stdout != "pipe-a\n") { print("pipe_exec failed"); return 5; }
    print("process OK exit=", r.exit_code, " first=", first, " rest=", rest);
    return 0;
}
