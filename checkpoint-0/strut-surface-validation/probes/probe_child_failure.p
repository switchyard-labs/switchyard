function main() -> int : (ExecError, Error) {
    failing := process("sh", ["-c", "echo before-crash; exit 3"]);
    first := failing.out.read_all();
    code := failing.wait();
    if (code != 3) { print("expected child exit 3, got ", code); return 1; }
    if (first != "before-crash\n") { print("missing child output"); return 2; }
    survived := exec("sh", ["-c", "echo survived"]);
    if (survived.exit_code != 0 || survived.stdout != "survived\n") { print("parent did not survive child failure"); return 3; }
    print("child_failure OK exit=", code, " parent=", survived.stdout);
    return 0;
}
