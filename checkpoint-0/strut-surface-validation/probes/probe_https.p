function main() -> int : HttpError {
    response := http_get("https://example.com/");
    if (response.status != 200) { print("status=", response.status); return 1; }
    if (response.body.size() < 100) { print("body too small"); return 2; }
    custom := http_request("GET", "https://example.com/", {"headers": {"accept": "text/html"}});
    if (custom.status != 200) { print("custom status=", custom.status); return 3; }
    print("https OK status=", response.status, " bytes=", response.body.size());
    return 0;
}