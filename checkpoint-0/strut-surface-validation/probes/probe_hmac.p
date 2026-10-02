include <crypto>;
include <encoding>;
function main() -> int : (CryptoError, EncodingError) {
    key := bytes.from_string("key");
    message := bytes.from_string("The quick brown fox jumps over the lazy dog");
    bytes expected := [247, 188, 131, 244, 48, 83, 132, 36, 177, 50, 152, 230, 170, 111, 177, 67, 239, 77, 89, 161, 73, 70, 23, 89, 151, 71, 157, 188, 45, 26, 60, 216];
    digest := hmac_sha256(key, message);
    if (!constant_time_equal(digest, expected)) { print("hmac mismatch"); return 1; }
    if (base64_encode(digest) != "97yD9DBThCSxMpjmqm+xQ+9NWaFJRhdZl0edvC0aPNg=") { print("base64 mismatch"); return 2; }
    round := base64_decode(base64_encode(digest));
    if (!constant_time_equal(round, digest)) { print("base64 roundtrip mismatch"); return 3; }
    bytes binary := [0, 15, 240];
    if (base64_encode(binary) != "AA/w" || base64url_encode(binary) != "AA_w") { print("base64url mismatch"); return 4; }
    if (sha256(bytes.from_string("")).size() != 32) { print("sha256 size"); return 5; }
    if (secure_random_bytes(32).size() != 32) { print("random size"); return 6; }
    print("hmac OK");
    return 0;
}
