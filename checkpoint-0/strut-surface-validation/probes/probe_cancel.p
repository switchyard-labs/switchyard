function main() -> int : (ThreadError, Error, TimeError) {
    cancellation_source source;
    cancellation_token token := source.token();
    channel<int> results;
    waiter := thread(() => {
        token.wait();
        if (token.cancelled()) { results.send(1); } else { results.send(0); }
    });
    sleep_ms(50);
    source.cancel();
    waiter.join();
    got := results.receive();
    if (got != 1) { print("cancellation not observed"); return 1; }
    if (!token.cancelled()) { print("token.cancelled() false"); return 2; }
    print("cancel OK");
    return 0;
}
