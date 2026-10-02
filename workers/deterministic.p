// Deterministic Switchyard worker (CP4): reads a task JSON file, reads the
// current content of a target file, appends a deterministic line, writes the
// new content to an output path, and reports. This is bounded, deterministic
// Strut execution — the control plane applies the produced change through the
// shared ref-mutation substrate.
include <filesystem>;

function main() -> int : (FilesystemError, Error) {
    task_path := env("SWITCHYARD_TASK") ?? "";
    task := json.parse(read_file(task_path));
    input := read_file(task["input_path"].as_string());
    append := task["append"].as_string();
    write_file(task["output_path"].as_string(), input + append);
    print("ok");
    return 0;
}