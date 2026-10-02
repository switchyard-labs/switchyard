include <filesystem>;
function main() -> int : FilesystemError {
    string dir := "/tmp/strut-fs-" + to_string(now_ms());
    make_dir(dir);
    write_file(dir + "/a.txt", "alpha");
    write_file(dir + "/b.bin", bytes.from_string("beta"));
    if (read_file(dir + "/a.txt") != "alpha") { print("read_file mismatch"); return 1; }
    if (exists(dir + "/a.txt") != true) { print("exists false"); return 2; }
    if (is_file(dir + "/a.txt") != true || is_dir(dir) != true) { print("type checks failed"); return 3; }
    files := ls(dir);
    if (files.size() != 2) { print("ls size ", files.size()); return 4; }
    string copy_dir_src := dir + "/a.txt";
    string copy_dir_dst := dir + "/a-copy.txt";
    copy(copy_dir_src, copy_dir_dst);
    if (!exists(dir + "/a-copy.txt")) { print("copy failed"); return 5; }
    string move_dir_dst := dir + "/a-moved.txt"; move(copy_dir_dst, move_dir_dst);
    if (!exists(dir + "/a-moved.txt")) { print("move failed"); return 6; }
    remove_all(dir);
    if (exists(dir)) { print("remove_all failed"); return 7; }
    print("fs OK");
    return 0;
}
