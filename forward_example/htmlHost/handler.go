package function

import (
	"fmt"
	"os"
)

// Handle a serverless
func Handle(req []byte) ([]byte, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("applying %s to %s", hostname, string(req))), nil
}
