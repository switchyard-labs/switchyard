include <filesystem>;
function main() -> int : (ExecError, FilesystemError) {
    version := exec("git", ["--version"]);
    if (version.exit_code != 0) { print("git --version failed"); return 1; }
    string dir := "/tmp/strut-git-probe-" + to_string(now_ms());
    make_dir(dir);
    write_file(dir + "/hello.txt", "hello from strut\n");
    init := exec("git", ["init", "-q", dir]);
    if (init.exit_code != 0) { print("git init failed"); return 2; }
    add := exec("git", ["-C", dir, "add", "hello.txt"]);
    if (add.exit_code != 0) { print("git add failed"); return 3; }
    commit := exec("git", ["-C", dir, "-c", "user.name=strut", "-c", "user.email=strut@local", "commit", "-q", "-m", "probe commit"]);
    if (commit.exit_code != 0) { print("git commit failed: ", commit.stderr); return 4; }
    log := exec("git", ["-C", dir, "log", "--oneline"]);
    if (log.exit_code != 0 || log.stdout == "") { print("git log failed"); return 5; }
    print("git OK version=", version.stdout, " log=", log.stdout);
    remove_all(dir);
    return 0;
}
