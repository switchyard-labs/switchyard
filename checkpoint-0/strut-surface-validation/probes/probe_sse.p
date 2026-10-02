function main() -> int : (HttpError, Error) {
    received := new("");
    total := new(0);
    expected := "event: event-one\ndata: payload-1\n\nevent: event-two\ndata: payload-2\n\nevent: event-three\ndata: payload-3\n\nevent: event-four\ndata: payload-4\n\nevent: event-five\ndata: payload-5\n\n";
    response := http_request_stream("GET", "http://127.0.0.1:8090/sse", {"max_response_body_bytes": 1000000}, null, (bytes chunk) => {
        *total = *total + 1;
        *received = *received + chunk.to_string();
        return true;
    });
    if (response.status != 200) { print("sse status=", response.status); return 1; }
    if (*received != expected) { print("sse body mismatch len=", (*received).size(), " expected=", expected.size()); return 2; }
    print("sse OK chunks=", *total, " len=", (*received).size());
    return 0;
}
