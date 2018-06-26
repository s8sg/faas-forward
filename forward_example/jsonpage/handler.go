package function

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
)

type HtmlObject struct {
	Urls map[string]int
}

var (
	gen *template.Template = template.Must(template.ParseGlob("assets/*.html"))
)

// Handle a serverless
func Handle(req []byte) ([]byte, error) {

	// Get function name
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	log.Printf("Executing %s", hostname)

	var data map[string]int

	// Unmarshal json
	err = json.Unmarshal(req, &data)
	if err != nil {
		log.Printf("failed to parse data, error %v", err)
		return nil, fmt.Errorf("failed to parse data, error %v", err)
	}

	htmlObj := HtmlObject{Urls: data}

	var b bytes.Buffer
	w := bufio.NewWriter(&b)

	// parse template
	err = gen.ExecuteTemplate(w, "page", htmlObj)
	if err != nil {
		log.Printf("failed to generate html, error %v", err)
		return nil, fmt.Errorf("failed to generate html, error %v", err)
	}
	resp := b.Bytes()
	return resp, nil
}
