package linodego

import "context"

// AccountTransfer represents an Account's network utilization for the current month.
type AccountTransfer struct {
	Billable int `json:"billable"`
	Quota    int `json:"quota"`
	Used     int `json:"used"`

	RegionTransfers []AccountTransferRegion `json:"region_transfers"`
}

// AccountTransferRegion represents an Account's network utilization for the current month
// in a given region.
type AccountTransferRegion struct {
	ID       string `json:"id"`
	Billable int    `json:"billable"`
	Quota    int    `json:"quota"`
	Used     int    `json:"used"`
}

// GetAccountTransfer gets current Account's network utilization for the current month.
func (c *Client) GetAccountTransfer(ctx context.Context) (*AccountTransfer, error) {
	req := c.R(ctx).SetResult(&AccountTransfer{})
	e := "account/transfer"
	r, err := coupleAPIErrors(req.Get(e))
	if err != nil {
		return nil, err
	}

	return r.Result().(*AccountTransfer), nil
}
