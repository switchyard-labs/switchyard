function main() -> int {
    print("shutdown-ready");
    wait_for_shutdown_signal();
    print("shutdown-observed");
    return 0;
}