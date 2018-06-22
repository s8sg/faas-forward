package function

import (
	"fmt"
)

// Handle a serverless
func Handle(req []byte) ([]byte, error) {
	return []byte(fmt.Sprintf("Hello, Go-Forward: %s", string(req))), nil
}
