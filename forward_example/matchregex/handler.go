package function

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
)

// Handle a serverless
func Handle(req []byte) ([]byte, error) {

	// Get function name
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	log.Printf("Executing %s", hostname)

	regexString := os.Getenv("regex")

	r, err := regexp.Compile(regexString)
	if err != nil {
		log.Printf("failed to compile regex %s, error %v", regexString, err)
		return nil, fmt.Errorf("failed to compile regex %s, error %v", regexString, err)
	}

	matches := r.FindAllString(string(req), -1)
	var jsonMap map[string]int
	for _, key := range matches {
		if _, ok := jsonMap[key]; !ok {
			jsonMap[key] = 1
		} else {
			jsonMap[key] = jsonMap[key] + 1
		}
	}

	bytes, err := json.Marshal(jsonMap)
	if err != nil {
		log.Printf("failed to marshal %v, error %v", jsonMap, err)
		return nil, fmt.Errorf("failed to marshal %v, error %v", jsonMap, err)
	}

	return bytes, nil
}
