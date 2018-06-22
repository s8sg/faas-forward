package main

import (
	"bytes"
	"fmt"
	"github.com/rs/xid"
	"handler/function"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	reqDIR = "/home/app"
)

var (
	async       = false
	forwardAddr string
	//regex       = regexp.MustCompile("[^,\\s][^\\,]*[^,\\s]*")
	reqStore     = make(map[string][]byte)
	requestQueue = make(chan string, 10)
)

// generate a request id based on request
func genRequestId() string {
	id := xid.New()
	return id.String()
}

// upload logic
func reqHandle(w http.ResponseWriter, r *http.Request) {

	var body []byte
	var requestID string

	// Try to read request as forwarded request
	r.ParseMultipartForm(32 << 20)
	req, header, err := r.FormFile("data")
	defer req.Close()
	switch err {
	// in case no failure get requestID and data
	case nil:
		requestID := header.Filename
		reqsize := header.Size
		log.Printf("received request with request-ID '%s' with size '%d'", requestID, reqsize)
		body, err = ioutil.ReadAll(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read forwarded request with ID '%s', error: %v", requestID, err), http.StatusInternalServerError)
			return
		}
	// in case of error treat it as direct request
	default:
		// Generate the request id
		requestID := genRequestId()
		log.Printf("received fresh request, generated request ID: %s", requestID)
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read forwarded request '%s', error: %v", requestID, err), http.StatusInternalServerError)
			return
		}

	}

	// handle the request using user defined handler
	respbytes, err := function.Handle(body)
	if err != nil {
		// in case of failure just fallback
		http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
		return
	}

	// Check for request to perform in Sync
	switch async {
	case true:
		reqStore[requestID] = respbytes
		// put on the request queue to be performed in async
		requestQueue <- requestID
	case false:
		err = forwardToFunction(requestID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// TODO: Post request handler (we might implement it later)
		//       This way the last function on the chain would be executed at first
		//       although user approah is more likely to be:
		//            result = [data].apply(func1).apply(func2).apply(func3)
	}
	return
}

// forward the request data
func forward(client *http.Client, url string, requestID string, data []byte) (err error) {

	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	var fw io.Writer

	if fw, err = w.CreateFormFile("data", requestID); err != nil {
		return
	}

	r := bytes.NewReader(data)
	if _, err = io.Copy(fw, r); err != nil {
		return err
	}

	w.Close()

	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Submit the request
	res, err := client.Do(req)
	if err != nil {
		return
	}

	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	return
}

// forward request to the function
func forwardToFunction(requestID string) error {
	client := &http.Client{}
	err := forward(client, forwardAddr, requestID, reqStore[requestID])
	if err != nil {
		return err
	}
	return nil
}

// The request forwarder thread
func forwarder() {
	for true {
		// read from channel
		select {
		// consume from the request queue
		case requestID := <-requestQueue:
			log.Printf("New request '%s' received from queue", requestID)
			err := forwardToFunction(requestID)
			if err != nil {
				log.Printf("failed to forward the request to '%s', error %v", requestID, err)
			}
			// delete the request buffer from req Store
			delete(reqStore, requestID)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// initialize
func initialize() {
	forwardAddr = os.Getenv("forward")
	if forwardAddr == "" {
		log.Printf("No forward address provided, considering function as end of chain")
	}
	if strings.ToUpper(os.Getenv("async")) == "TRUE" {
		log.Printf("Async flag is set, function won't wait for forward chain")
		async = true
	}
}

func main() {

	initialize()

	// Start the forwarder queue if async request is needed
	if async {
		go forwarder()
	}

	// handle request with request handle
	http.HandleFunc("/", reqHandle)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
