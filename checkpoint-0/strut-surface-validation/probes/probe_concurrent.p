include <filesystem>;
function work(int_64 id, string dir) -> int_64 : FilesystemError {
    write_file(dir + "/step-" + to_string(id) + ".txt", "done-" + to_string(id));
    return id * 2;
}
function main() -> int : (ThreadError, FilesystemError) {
    string dir := "/tmp/strut-concurrent-" + to_string(now_ms());
    make_dir(dir);
    channel<int_64> results;
    for (int_64 i := 0; i < 16; i++) {
        thread(() => {
            try { results.send(work(i, dir)); }
            catch (FilesystemError caught) { results.send(-1); }
        });
    }
    int_64 expected := 0;
    for (int_64 i := 0; i < 16; i++) {
        got := results.receive();
        if (got == null) { print("channel closed early"); return 3; }
        if (*got != i * 2) { print("step ", i, " returned ", *got); return 1; }
        expected = expected + *got;
    }
    files := ls(dir);
    if (files.size() != 16) { print("expected 16 files, got ", files.size()); return 2; }
    print("concurrent OK sum=", expected, " files=", files.size());
    remove_all(dir);
    return 0;
}
