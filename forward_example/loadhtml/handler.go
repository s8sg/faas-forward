package function

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// Handle a serverless
func Handle(req []byte) ([]byte, error) {

	// Get function name
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	log.Printf("Executing %s", hostname)

	url := string(req)

	// http get
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("failed to request %s, error %v", url, err)
		return nil, fmt.Errorf("failed to request %s, error %v", url, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to request %s, error %v", url, err)
		return nil, fmt.Errorf("failed to request %s, error %v", url, err)
	}

	return body, nil
}
