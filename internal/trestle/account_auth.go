package trestle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type AccountToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AccountAuth delegates token security and account mutation to Trestle. Raw
// secrets are sent in JSON bodies and never included in an error message.
func (c *Client) AccountAuth(collection, id string, input map[string]any) (AccountToken, error) {
	var out AccountToken
	resp, b, err := c.do(http.MethodPost, "/api/v1/collections/"+url.PathEscape(collection)+"/records/"+url.PathEscape(id)+"/account-auth", input)
	if err != nil {
		return out, fmt.Errorf("account operation unavailable")
	}
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return out, &APIError{Status: resp.StatusCode, Operation: "account-auth", Collection: collection, Code: "account_operation_failed", Message: "account operation failed"}
	}
	if json.Unmarshal(b, &out) != nil {
		return out, fmt.Errorf("account response invalid")
	}
	return out, nil
}
