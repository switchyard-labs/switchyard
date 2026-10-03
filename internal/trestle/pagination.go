package trestle

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type Record struct {
	ID      string         `json:"id"`
	Version int            `json:"version"`
	Values  map[string]any `json:"values"`
}
type RecordPage struct {
	Items      []Record `json:"items"`
	NextCursor string   `json:"nextCursor"`
}

func (c *Client) ListRecordsPage(collection, filter, cursor string, limit int) (RecordPage, error) {
	var page RecordPage
	if limit < 1 || limit > 100 {
		return page, fmt.Errorf("record page limit must be 1..100")
	}
	query := url.Values{}
	query.Set("limit", fmt.Sprint(limit))
	if filter != "" {
		query.Set("filter", filter)
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	resp, body, err := c.do(http.MethodGet, "/api/v1/collections/"+url.PathEscape(collection)+"/records?"+query.Encode(), nil)
	if err != nil {
		if errors.Is(err, ErrResponseTooLarge) && limit > 1 {
			return c.ListRecordsPage(collection, filter, cursor, max(1, limit/2))
		}
		return page, err
	}
	if resp.StatusCode != 200 {
		return page, responseError("list", collection, resp.StatusCode, body)
	}
	err = json.Unmarshal(body, &page)
	return page, err
}
