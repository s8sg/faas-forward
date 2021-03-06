package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/rs/xid"
	"handler/function"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	reqDIR = "/home/app"
)

var (
	//regex       = regexp.MustCompile("[^,\\s][^\\,]*[^,\\s]*")
	inputType            = "FILE"
	fileFormName         = "file"
	forwardEnable        = true
	async                = false
	contentType          = "application/octet-stream"
	forwardAddr          string
	reqStore             = make(map[string][]byte)
	requestQueue         = make(chan string, 10)
	readTimeout          time.Duration
	writeTimeout         time.Duration
	acceptingConnections bool
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
	var err error

	// in case no failure get requestID and data
	switch inputType {
	// Input type
	case "FILE":
		// Try to read request as forwarded request
		err = r.ParseMultipartForm(32 << 20)
		if err != nil {
			log.Printf("failed to parse forwarded data, error: %v", err)
			http.Error(w, fmt.Sprintf("failed to parse forwarded data, error: %v", err), http.StatusInternalServerError)
			return
		}
		req, header, err := r.FormFile(fileFormName)
		if err != nil {
			log.Printf("failed to parse forwarded data, error: %v", err)
			http.Error(w, fmt.Sprintf("failed to parse forwarded data, error: %v", err), http.StatusInternalServerError)
			return
		}
		defer req.Close()
		requestID = header.Filename
		reqsize := header.Size
		log.Printf("received request with request-ID '%s' with size '%d'", requestID, reqsize)
		body, err = ioutil.ReadAll(req)
		if err != nil {
			log.Printf("failed to read forwarded request with ID '%s', error: %v", requestID, err)
			http.Error(w, fmt.Sprintf("failed to read forwarded request with ID '%s', error: %v", requestID, err), http.StatusInternalServerError)
			return
		}
	case "POST":
		// Generate the request id
		requestID = genRequestId()
		log.Printf("received fresh request, generated request ID: %s", requestID)
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("failed to read forwarded request '%s', error: %v", requestID, err)
			http.Error(w, fmt.Sprintf("failed to read forwarded request '%s', error: %v", requestID, err), http.StatusInternalServerError)
			return
		}
	}

	// handle the request using user defined handler
	respbytes, err := function.Handle(body)
	if err != nil {
		// in case of failure just fallback
		log.Printf("Failed to handle request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to handle request: %v", err), http.StatusInternalServerError)
		return
	}

	if !forwardEnable {
		w.Write(respbytes)
		w.Header().Set("Content-Type", contentType)
		return
	}

	// Check for request to perform in Sync
	switch async {
	case true:
		reqStore[requestID] = respbytes
		// put on the request queue to be performed in async
		requestQueue <- requestID
	case false:
		client := &http.Client{}
		data, respType, err := forward(client, forwardAddr, requestID, respbytes)
		if err != nil {
			log.Printf("failed to request %s, error : %v", forwardAddr, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(data)
		w.Header().Set("Content-Type", respType)
		// TODO: Post request handler (we might implement it later)
		//       This way the last function on the chain would be executed at first
		//       although user approah is more likely to be:
		//            result = [data].apply(func1).apply(func2).apply(func3)
	}
	return
}

// forward the request data
func forward(client *http.Client, url string, requestID string, data []byte) (result []byte, respType string, err error) {

	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer

	w := multipart.NewWriter(&b)

	var fw io.Writer

	if fw, err = w.CreateFormFile("file", requestID); err != nil {
		return
	}

	fw.Write(data)

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
	defer res.Body.Close()

	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
		return
	}

	respType = res.Header.Get("Content-Type")

	// Read the result
	result, err = ioutil.ReadAll(res.Body)

	return
}

// forward request to the function
func forwardToFunction(requestID string) error {
	client := &http.Client{}
	_, _, err := forward(client, forwardAddr, requestID, reqStore[requestID])
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

func lockFilePresent() bool {
	path := filepath.Join(os.TempDir(), ".lock")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func createLockFile() (string, error) {
	path := filepath.Join(os.TempDir(), ".lock")
	log.Printf("Writing lock-file to: %s\n", path)
	writeErr := ioutil.WriteFile(path, []byte{}, 0660)
	acceptingConnections = true

	return path, writeErr
}

// handle health request
func healthHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if acceptingConnections == false || lockFilePresent() == false {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		break
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func parseIntOrDurationValue(val string, fallback time.Duration) time.Duration {
	if len(val) > 0 {
		parsedVal, parseErr := strconv.Atoi(val)
		if parseErr == nil && parsedVal >= 0 {
			return time.Duration(parsedVal) * time.Second
		}
	}
	duration, durationErr := time.ParseDuration(val)
	if durationErr != nil {
		return fallback
	}
	return duration
}

// initialize
func initialize() {
	forwardAddr = os.Getenv("forward")
	if forwardAddr == "" {
		log.Printf("No forward address provided, considering function as end of chain")
		forwardEnable = false
	}
	forwardAddr = "http://" + forwardAddr + ":8080"
	if strings.ToUpper(os.Getenv("async")) == "TRUE" {
		log.Printf("Async flag is set, function won't wait for forward chain")
		async = true
	}

	if strings.ToUpper(os.Getenv("input_type")) == "POST" {
		inputType = "POST"
	} else {
		if os.Getenv("file_form_name") != "" {
			fileFormName = os.Getenv("file_form_name")
		}
	}

	if os.Getenv("content_type") != "" {
		contentType = os.Getenv("content_type")
	}

	readTimeout = parseIntOrDurationValue(os.Getenv("read_timeout"), time.Second*5)
	writeTimeout = parseIntOrDurationValue(os.Getenv("write_timeout"), time.Second*5)
}

func main() {

	initialize()

	// Start the forwarder queue if async request is needed
	if async {
		go forwarder()
	}

	s := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	// handle request with request handle
	http.HandleFunc("/", reqHandle)
	http.HandleFunc("/_/health", healthHandler)

	path, writeErr := createLockFile()
	if writeErr != nil {
		log.Panicf("Cannot write %s\n Error: %s.\n", path, writeErr.Error())
	}

	listenUntilShutdown(writeTimeout, s)
}

func listenUntilShutdown(shutdownTimeout time.Duration, s *http.Server) {

	idleConnsClosed := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM)

		<-sig

		log.Printf("SIGTERM received.. shutting down server")

		acceptingConnections = false

		if err := s.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("Error in Shutdown: %v", err)
		}

		<-time.Tick(shutdownTimeout)

		close(idleConnsClosed)
	}()

	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("Error ListenAndServe: %v", err)
		close(idleConnsClosed)
	}

	<-idleConnsClosed
}
