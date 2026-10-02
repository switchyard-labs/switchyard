function main() -> int {
    value := json.parse("{\"name\":\"Nick\",\"items\":[10,20,30],\"ok\":true}");
    if (value["name"].as_string() != "Nick") { print("name mismatch"); return 1; }
    if (value["items"][1].as_int() != 20) { print("items[1] mismatch"); return 2; }
    out := json.stringify(value);
    if (out == "") { print("stringify empty"); return 3; }
    back := json.parse(out);
    if (back["name"].as_string() != "Nick" || back["items"][2].as_int() != 30) { print("roundtrip mismatch"); return 4; }
    if (json.stringify(back["ok"]) != "true") { print("bool roundtrip mismatch"); return 5; }
    print("json OK name=", value["name"].as_string(), " len=", out.size());
    return 0;
}
